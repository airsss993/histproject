package submissions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPCreateParsesMultipartAndReturnsProcessingSubmission(t *testing.T) {
	service := &submissionHTTPServiceStub{created: Submission{ID: 44, Status: StatusProcessing}}
	handler := NewHandler(service, nil, discardSubmissionLogger(), nil, testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	body, contentType := submissionMultipart(t, validSubmissionForm(), "site.zip", []byte("zip"))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.createInput.Title != "Event" || service.createInput.EventType != "military" || service.createInput.ArchiveName != "site.zip" ||
		service.createInput.Latitude != 55.75 || service.createInput.Longitude != 37.62 {
		t.Fatalf("create input = %#v", service.createInput)
	}
	if service.createInput.Email != "author@example.test" {
		t.Fatalf("email = %q, want account email instead of form value", service.createInput.Email)
	}
	if string(service.archive) != "zip" {
		t.Fatalf("archive = %q", service.archive)
	}
	var payload struct {
		Submission struct {
			ID     int64  `json:"id"`
			Status Status `json:"status"`
		} `json:"submission"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Submission.ID != 44 || payload.Submission.Status != StatusProcessing {
		t.Fatalf("submission = %#v", payload.Submission)
	}
}

func TestHTTPCreateRequiresVisitorSession(t *testing.T) {
	service := &submissionHTTPServiceStub{}
	guest := func(*http.Request) (string, error) { return "", ErrUnauthorized }
	handler := NewHandler(service, nil, discardSubmissionLogger(), nil, guest, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	body, contentType := submissionMultipart(t, validSubmissionForm(), "site.zip", []byte("zip"))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertSubmissionError(t, response, http.StatusUnauthorized, "unauthorized")
	if service.createCalled {
		t.Fatal("service.Create was called without session")
	}
}

func TestHTTPCreateLimitsBodyBeforeMultipartParsing(t *testing.T) {
	service := &submissionHTTPServiceStub{}
	handler := NewHandler(service, nil, discardSubmissionLogger(), nil, testAuthorReader, 64)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	body, contentType := submissionMultipart(t, validSubmissionForm(), "site.zip", bytes.Repeat([]byte("x"), 2<<20))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertSubmissionError(t, response, http.StatusRequestEntityTooLarge, "body_too_large")
	if service.createCalled {
		t.Fatal("service.Create was called for oversized body")
	}
}

func TestHTTPCreateValidatesPublicFields(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(map[string]string)
		filename string
		field    string
	}{
		{name: "required title", mutate: func(values map[string]string) { values["title"] = "" }, filename: "site.zip", field: "title"},
		{name: "required description", mutate: func(values map[string]string) { values["description"] = "" }, filename: "site.zip", field: "description"},
		{name: "date", mutate: func(values map[string]string) { values["eventDate"] = "02.01.2020" }, filename: "site.zip", field: "eventDate"},
		{name: "event type", mutate: func(values map[string]string) { values["eventType"] = "unknown" }, filename: "site.zip", field: "eventType"},
		{name: "telegram", mutate: func(values map[string]string) { values["telegramUsername"] = "bad name" }, filename: "site.zip", field: "telegramUsername"},
		{name: "required latitude", mutate: func(values map[string]string) { delete(values, "latitude") }, filename: "site.zip", field: "latitude"},
		{name: "longitude range", mutate: func(values map[string]string) { values["longitude"] = "181" }, filename: "site.zip", field: "longitude"},
		{name: "zip", mutate: func(map[string]string) {}, filename: "site.rar", field: "archive"},
		{name: "required archive", mutate: func(map[string]string) {}, filename: "", field: "archive"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &submissionHTTPServiceStub{}
			handler := NewHandler(service, nil, discardSubmissionLogger(), nil, testAuthorReader, 52_428_800)
			mux := http.NewServeMux()
			handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
			values := validSubmissionForm()
			test.mutate(values)
			body, contentType := submissionMultipart(t, values, test.filename, []byte("zip"))
			request := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", body)
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			assertSubmissionFieldError(t, response, test.field)
			if service.createCalled {
				t.Fatal("service.Create was called for invalid form")
			}
		})
	}
}

func TestHTTPCreateAppliesRateLimitWithoutContactData(t *testing.T) {
	service := &submissionHTTPServiceStub{}
	limiter := &submissionLimiterStub{allowed: false}
	handler := NewHandler(service, limiter, discardSubmissionLogger(), nil, testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	body, contentType := submissionMultipart(t, validSubmissionForm(), "site.zip", []byte("zip"))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/submissions", body)
	request.Header.Set("Content-Type", contentType)
	request.RemoteAddr = "203.0.113.9:54000"
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertSubmissionError(t, response, http.StatusTooManyRequests, "rate_limited")
	if limiter.remoteAddress != "203.0.113.9" {
		t.Fatalf("limiter input = %q", limiter.remoteAddress)
	}
	if service.createCalled {
		t.Fatal("service.Create was called after rate limit")
	}
}

func TestHTTPListAndDetailFormatAdminResponses(t *testing.T) {
	date := mustSubmissionDate(t, "2020-01-02")
	service := &submissionHTTPServiceStub{
		page:   SubmissionPage{Items: []Submission{{ID: 8, Title: "Event", EventDate: date, EventType: "military", Status: StatusReview}}, Page: 2, PageSize: 10, Total: 11},
		detail: Submission{ID: 8, Title: "Event", EventDate: date, EventType: "military", Email: "author@example.test", TelegramUsername: "author", Status: StatusReview},
	}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)

	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/submissions?status=review&q=Event&page=2&pageSize=10", nil))
	if listResponse.Code != http.StatusOK || service.listFilter.Status != StatusReview || service.listFilter.Search != "Event" || service.listFilter.Page != 2 {
		t.Fatalf("list status = %d, filter = %#v, body = %s", listResponse.Code, service.listFilter, listResponse.Body.String())
	}

	detailResponse := httptest.NewRecorder()
	mux.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/submissions/8", nil))
	if detailResponse.Code != http.StatusOK || service.getID != 8 || !strings.Contains(detailResponse.Body.String(), `"eventDate":"2020-01-02"`) {
		t.Fatalf("detail status = %d, ID = %d, body = %s", detailResponse.Code, service.getID, detailResponse.Body.String())
	}
}

func TestHTTPListRejectsMalformedPagination(t *testing.T) {
	service := &submissionHTTPServiceStub{}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	for _, page := range []string{"many", "0"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/submissions?page="+page, nil))
		assertSubmissionFieldError(t, response, "page")
	}
}

func TestHTTPModerationPassesActorAndInput(t *testing.T) {
	service := &submissionHTTPServiceStub{}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/admin/submissions/7/review"},
		{method: http.MethodPost, path: "/api/v1/admin/submissions/8/reject", body: `{"comment":"duplicate"}`},
		{method: http.MethodPost, path: "/api/v1/admin/submissions/9/publish", body: `{"latitude":55.5,"longitude":60.5,"eventType":"political"}`},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", test.path, response.Code, response.Body.String())
		}
	}
	if service.reviewID != 7 || service.rejectID != 8 || service.rejectComment != "duplicate" || service.publishID != 9 || service.eventType != "political" {
		t.Fatalf("moderation IDs = review:%d reject:%d publish:%d", service.reviewID, service.rejectID, service.publishID)
	}
	if service.actor.Login != "editor" || service.actor.Role != "admin" || service.latitude != 55.5 || service.longitude != 60.5 {
		t.Fatalf("moderation input = actor:%#v coordinates:%f,%f", service.actor, service.latitude, service.longitude)
	}
}

func TestHTTPPublishMapsCoordinateValidation(t *testing.T) {
	service := &submissionHTTPServiceStub{publishError: &ValidationError{Fields: map[string]string{"latitude": "От -90 до 90"}}}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/admin/submissions/9/publish", strings.NewReader(`{"latitude":91,"longitude":60}`)))

	assertSubmissionFieldError(t, response, "latitude")
}

func TestHTTPAuditUsesSuperAdminProtection(t *testing.T) {
	service := &submissionHTTPServiceStub{auditPage: AuditPage{Items: []AuditEntry{{ID: 1, SubmissionID: 8, SubmissionTitle: "Event", Action: "reviewed"}}, Page: 1, PageSize: 20, Total: 1}}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	adminCalls := 0
	superCalls := 0
	handler.Register(mux, countSubmissionMiddleware(&adminCalls), countSubmissionMiddleware(&superCalls))

	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/submissions", nil))
	auditResponse := httptest.NewRecorder()
	mux.ServeHTTP(auditResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-log?q=Event&page=1&pageSize=20", nil))

	if listResponse.Code != http.StatusOK || auditResponse.Code != http.StatusOK {
		t.Fatalf("statuses = list:%d audit:%d", listResponse.Code, auditResponse.Code)
	}
	if adminCalls != 1 || superCalls != 1 {
		t.Fatalf("middleware calls = admin:%d super:%d", adminCalls, superCalls)
	}
	if service.auditFilter.Search != "Event" || !strings.Contains(auditResponse.Body.String(), `"submissionTitle":"Event"`) {
		t.Fatalf("audit filter = %#v, body = %s", service.auditFilter, auditResponse.Body.String())
	}
}

func TestHTTPReturnsGenericInternalError(t *testing.T) {
	service := &submissionHTTPServiceStub{getError: errors.New("database password leaked")}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/submissions/9", nil))

	assertSubmissionError(t, response, http.StatusInternalServerError, "internal_error")
	if strings.Contains(response.Body.String(), "database") || strings.Contains(response.Body.String(), "password") {
		t.Fatalf("response leaks internal error: %s", response.Body.String())
	}
}

func TestHTTPMapsNotFoundAndInvalidTransition(t *testing.T) {
	service := &submissionHTTPServiceStub{getError: ErrSubmissionNotFound, reviewError: ErrInvalidTransition}
	handler := NewHandler(service, nil, discardSubmissionLogger(), testActorReader(), testAuthorReader, 52_428_800)
	mux := http.NewServeMux()
	handler.Register(mux, passSubmissionMiddleware, passSubmissionMiddleware)

	missingResponse := httptest.NewRecorder()
	mux.ServeHTTP(missingResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/submissions/99", nil))
	assertSubmissionError(t, missingResponse, http.StatusNotFound, "submission_not_found")

	conflictResponse := httptest.NewRecorder()
	mux.ServeHTTP(conflictResponse, httptest.NewRequest(http.MethodPost, "/api/v1/admin/submissions/7/review", nil))
	assertSubmissionError(t, conflictResponse, http.StatusConflict, "invalid_transition")
}

type submissionHTTPServiceStub struct {
	createCalled  bool
	createInput   CreateInput
	archive       []byte
	created       Submission
	createError   error
	listFilter    ListFilter
	page          SubmissionPage
	listError     error
	getID         int64
	detail        Submission
	getError      error
	reviewID      int64
	reviewError   error
	rejectID      int64
	rejectComment string
	rejectError   error
	publishID     int64
	latitude      float64
	longitude     float64
	publishError  error
	eventType     string
	actor         Actor
	auditFilter   AuditFilter
	auditPage     AuditPage
	auditError    error
}

func (stub *submissionHTTPServiceStub) Create(_ context.Context, input CreateInput) (Submission, error) {
	stub.createCalled = true
	stub.createInput = input
	stub.archive, _ = io.ReadAll(input.Archive)
	return stub.created, stub.createError
}

func (stub *submissionHTTPServiceStub) List(_ context.Context, filter ListFilter) (SubmissionPage, error) {
	stub.listFilter = filter
	return stub.page, stub.listError
}

func (stub *submissionHTTPServiceStub) Get(_ context.Context, id int64) (Submission, error) {
	stub.getID = id
	return stub.detail, stub.getError
}

func (stub *submissionHTTPServiceStub) Review(_ context.Context, id int64, actor Actor) (Submission, error) {
	stub.reviewID = id
	stub.actor = actor
	return Submission{ID: id, Status: StatusReview}, stub.reviewError
}

func (stub *submissionHTTPServiceStub) Reject(_ context.Context, id int64, actor Actor, comment string) (Submission, error) {
	stub.rejectID = id
	stub.rejectComment = comment
	stub.actor = actor
	return Submission{ID: id, Status: StatusRejected}, stub.rejectError
}

func (stub *submissionHTTPServiceStub) Publish(_ context.Context, id int64, actor Actor, latitude, longitude float64, eventType string) (Submission, error) {
	stub.eventType = eventType
	stub.publishID = id
	stub.actor = actor
	stub.latitude = latitude
	stub.longitude = longitude
	return Submission{ID: id, Status: StatusPublished}, stub.publishError
}

func (stub *submissionHTTPServiceStub) AuditLog(_ context.Context, filter AuditFilter) (AuditPage, error) {
	stub.auditFilter = filter
	return stub.auditPage, stub.auditError
}

type submissionLimiterStub struct {
	allowed       bool
	err           error
	remoteAddress string
}

func (stub *submissionLimiterStub) Allow(_ context.Context, remoteAddress string) (bool, error) {
	stub.remoteAddress = remoteAddress
	return stub.allowed, stub.err
}

func validSubmissionForm() map[string]string {
	return map[string]string{
		"title": "Event", "description": "Description", "eventDate": "2020-01-02",
		"eventType": "military", "email": "typo@example.test", "telegramUsername": "author_name",
		"latitude": "55.75", "longitude": "37.62",
	}
}

func submissionMultipart(t *testing.T, values map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range values {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if filename != "" {
		part, err := writer.CreateFormFile("archive", filename)
		if err != nil {
			t.Fatalf("create archive part: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write archive: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return body, writer.FormDataContentType()
}

func testActorReader() ActorReader {
	return func(context.Context) (Actor, bool) {
		return Actor{Login: "editor", Role: "admin"}, true
	}
}

func passSubmissionMiddleware(next http.Handler) http.Handler { return next }

func countSubmissionMiddleware(calls *int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*calls++
			next.ServeHTTP(w, r)
		})
	}
}

func discardSubmissionLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func assertSubmissionFieldError(t *testing.T, response *httptest.ResponseRecorder, field string) {
	t.Helper()
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Code != "validation_failed" || payload.Error.Fields[field] == "" {
		t.Fatalf("error = %#v, want field %q", payload.Error, field)
	}
}

func assertSubmissionError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, status, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Code != code {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, code)
	}
}

func testAuthorReader(*http.Request) (string, error) { return "author@example.test", nil }
