package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDAddsHeaderAndContext(t *testing.T) {
	var contextID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	headerID := response.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("X-Request-ID is empty")
	}
	if contextID != headerID {
		t.Fatalf("context request ID = %q, header = %q", contextID, headerID)
	}
}

func TestRecoverReturnsJSONAndKeepsRequestID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := Chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}),
		RequestID,
		Recover(logger),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is empty")
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error code = %q, want internal_error", body.Error.Code)
	}
	if body.Error.Message != "Внутренняя ошибка" {
		t.Errorf("error message = %q", body.Error.Message)
	}
}

func TestCORSMatchesWholeOrigin(t *testing.T) {
	handler := CORS([]string{"https://example.com"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q", got)
	}
	if got := response.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}

func TestCORSRejectsSubstringOrigin(t *testing.T) {
	handler := CORS([]string{"https://example.com"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://evil-example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORSHandlesAllowedPreflight(t *testing.T) {
	called := false
	handler := CORS([]string{"https://example.com"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if called {
		t.Fatal("next handler was called for preflight")
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"Referrer-Policy":         "no-referrer",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
	}
	for header, value := range want {
		if got := response.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestAccessLogRecordsResponse(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("ok"))
		}),
		RequestID,
		AccessLog(logger),
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/submissions", nil))

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if entry["request_id"] == "" {
		t.Error("request_id is empty")
	}
	if entry["method"] != http.MethodPost {
		t.Errorf("method = %#v", entry["method"])
	}
	if entry["path"] != "/api/v1/submissions" {
		t.Errorf("path = %#v", entry["path"])
	}
	if entry["status"] != float64(http.StatusCreated) {
		t.Errorf("status = %#v", entry["status"])
	}
	if entry["response_bytes"] != float64(2) {
		t.Errorf("response_bytes = %#v", entry["response_bytes"])
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Error("duration_ms is missing")
	}
}

func TestAccessLogKeepsFirstResponseStatus(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := AccessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if entry["status"] != float64(http.StatusOK) {
		t.Errorf("status = %#v, want 200", entry["status"])
	}
}
