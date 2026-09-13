package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type checkerFunc struct {
	name string
	ping func(context.Context) error
}

func (c checkerFunc) Name() string {
	return c.name
}

func (c checkerFunc) Ping(ctx context.Context) error {
	return c.ping(ctx)
}

func TestHealthzReportsLiveProcess(t *testing.T) {
	handler := NewHandler(time.Second)
	mux := http.NewServeMux()
	handler.Register(mux)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestReadyzReportsNotReadyWithoutChecks(t *testing.T) {
	handler := NewHandler(time.Second)
	mux := http.NewServeMux()
	handler.Register(mux)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	body := decodeReadiness(t, response)
	if body.Status != "not_ready" {
		t.Fatalf("status body = %q, want not_ready", body.Status)
	}
}

func TestReadyzReportsReadyWhenAllChecksPass(t *testing.T) {
	handler := NewHandler(time.Second,
		passingChecker("postgres"),
		passingChecker("redis"),
		passingChecker("minio"),
	)
	mux := http.NewServeMux()
	handler.Register(mux)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	body := decodeReadiness(t, response)
	if body.Status != "ready" {
		t.Errorf("status body = %q, want ready", body.Status)
	}
	want := map[string]string{"postgres": "ok", "redis": "ok", "minio": "ok"}
	for name, status := range want {
		if body.Checks[name] != status {
			t.Errorf("checks[%q] = %q, want %q", name, body.Checks[name], status)
		}
	}
}

func TestReadyzReportsDependencyFailureWithoutLeakingReason(t *testing.T) {
	handler := NewHandler(time.Second,
		passingChecker("postgres"),
		checkerFunc{name: "redis", ping: func(context.Context) error {
			return errors.New("redis password secret-value rejected")
		}},
	)
	mux := http.NewServeMux()
	handler.Register(mux)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret-value") {
		t.Fatal("response leaks dependency error")
	}
	body := decodeReadiness(t, response)
	if body.Status != "not_ready" {
		t.Errorf("status body = %q, want not_ready", body.Status)
	}
	if body.Checks["postgres"] != "ok" || body.Checks["redis"] != "failed" {
		t.Errorf("checks = %#v", body.Checks)
	}
}

func TestReadyzAppliesTimeout(t *testing.T) {
	const timeout = 250 * time.Millisecond
	deadlineSeen := false
	handler := NewHandler(timeout, checkerFunc{name: "postgres", ping: func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		deadlineSeen = ok && time.Until(deadline) > 0 && time.Until(deadline) <= timeout
		return nil
	}})
	mux := http.NewServeMux()
	handler.Register(mux)

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if !deadlineSeen {
		t.Fatal("checker context has no expected deadline")
	}
}

func passingChecker(name string) checkerFunc {
	return checkerFunc{name: name, ping: func(context.Context) error { return nil }}
}

type readinessBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func decodeReadiness(t *testing.T, response *httptest.ResponseRecorder) readinessBody {
	t.Helper()
	var body readinessBody
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}
