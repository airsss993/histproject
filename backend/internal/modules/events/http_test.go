package events

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPListParsesFiltersAndFormatsPage(t *testing.T) {
	preview := "https://storage.test/preview.png"
	service := &eventHTTPServiceStub{page: Page{
		Items: []Event{{ID: 7, Title: "Event", Latitude: 55.5, Longitude: 60.5, EventDate: mustHTTPDate(t, "2020-01-02"), EventType: "military", PreviewImageURL: &preview}},
		Page:  2, PageSize: 10, Total: 11,
	}}
	handler := NewHandler(service, discardEventLogger())
	mux := http.NewServeMux()
	handler.Register(mux, passEventMiddleware)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/events?eventType=military&eventType=political&dateFrom=2020-01-01&dateTo=2021-01-01&page=2&pageSize=10", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.listInput.Page != 2 || service.listInput.PageSize != 10 || len(service.listInput.EventTypes) != 2 {
		t.Fatalf("list input = %#v", service.listInput)
	}
	var body struct {
		Items []struct {
			ID        int64   `json:"id"`
			Latitude  float64 `json:"latitude"`
			EventDate string  `json:"eventDate"`
			EventType string  `json:"eventType"`
		} `json:"items"`
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != 7 || body.Items[0].Latitude != 55.5 || body.Items[0].EventDate != "2020-01-02" || body.Items[0].EventType != "military" {
		t.Fatalf("items = %#v", body.Items)
	}
	if body.Page != 2 || body.PageSize != 10 || body.Total != 11 {
		t.Fatalf("page metadata = (%d, %d, %d)", body.Page, body.PageSize, body.Total)
	}
}

func TestHTTPListRejectsMalformedPagination(t *testing.T) {
	service := &eventHTTPServiceStub{}
	handler := NewHandler(service, discardEventLogger())
	mux := http.NewServeMux()
	handler.Register(mux, passEventMiddleware)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/events?page=many", nil))

	assertEventError(t, response, http.StatusBadRequest, "validation_failed")
	if service.listCalled {
		t.Fatal("service.List was called for malformed pagination")
	}
}

func TestHTTPGetMapsMissingEvent(t *testing.T) {
	service := &eventHTTPServiceStub{getError: ErrEventNotFound}
	handler := NewHandler(service, discardEventLogger())
	mux := http.NewServeMux()
	handler.Register(mux, passEventMiddleware)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/events/99", nil))

	assertEventError(t, response, http.StatusNotFound, "event_not_found")
}

func TestHTTPUpdateCoordinatesUsesProtectionAndValidation(t *testing.T) {
	service := &eventHTTPServiceStub{updateError: &ValidationError{Fields: map[string]string{"latitude": "invalid"}}}
	handler := NewHandler(service, discardEventLogger())
	mux := http.NewServeMux()
	protected := false
	handler.Register(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			protected = true
			next.ServeHTTP(w, r)
		})
	})
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/api/v1/admin/events/7/coordinates", strings.NewReader(`{"latitude":91,"longitude":60}`)))

	if !protected {
		t.Fatal("admin middleware was not applied")
	}
	assertEventError(t, response, http.StatusBadRequest, "validation_failed")
}

func TestHTTPDeleteReturnsNoContent(t *testing.T) {
	service := &eventHTTPServiceStub{}
	handler := NewHandler(service, discardEventLogger())
	mux := http.NewServeMux()
	handler.Register(mux, passEventMiddleware)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/events/8", nil))

	if response.Code != http.StatusNoContent || service.deletedID != 8 {
		t.Fatalf("delete status = %d, ID = %d", response.Code, service.deletedID)
	}
}

type eventHTTPServiceStub struct {
	listCalled  bool
	listInput   ListInput
	page        Page
	listError   error
	event       Event
	getError    error
	updatedID   int64
	updateError error
	deletedID   int64
	deleteError error
}

func (stub *eventHTTPServiceStub) List(_ context.Context, input ListInput) (Page, error) {
	stub.listCalled = true
	stub.listInput = input
	return stub.page, stub.listError
}

func (stub *eventHTTPServiceStub) Get(context.Context, int64) (Event, error) {
	return stub.event, stub.getError
}

func (stub *eventHTTPServiceStub) UpdateCoordinates(_ context.Context, id int64, _, _ float64) error {
	stub.updatedID = id
	return stub.updateError
}

func (stub *eventHTTPServiceStub) Delete(_ context.Context, id int64) error {
	stub.deletedID = id
	return stub.deleteError
}

func passEventMiddleware(next http.Handler) http.Handler { return next }

func discardEventLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func mustHTTPDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	return date
}

func assertEventError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, status, response.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
}
