package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimitRejectsOverLimitAndSkipsNonAPIPaths(t *testing.T) {
	counts := make(map[string]int64)
	limiter := func(_ context.Context, key string, limit int64, _ time.Duration) (bool, error) {
		counts[key]++
		return counts[key] <= limit, nil
	}
	handler := RateLimit(limiter, 2, time.Minute, slog.New(slog.NewJSONHandler(io.Discard, nil)))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	)
	serve := func(path, remoteAddr string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = remoteAddr
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	for hit := 1; hit <= 3; hit++ {
		response := serve("/api/v1/events", "203.0.113.9:1000")
		want := http.StatusNoContent
		if hit == 3 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("hit %d status = %d, want %d", hit, response.Code, want)
		}
	}
	if response := serve("/api/v1/events", "203.0.113.9:1000"); response.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q", response.Header().Get("Retry-After"))
	}
	if response := serve("/api/v1/events", "198.51.100.7:1000"); response.Code != http.StatusNoContent {
		t.Fatalf("other client status = %d", response.Code)
	}
	for _, path := range []string{"/healthz", "/api/v1/auth/session"} {
		for hit := 0; hit < 3; hit++ {
			if response := serve(path, "203.0.113.9:1000"); response.Code != http.StatusNoContent {
				t.Fatalf("%s status = %d", path, response.Code)
			}
		}
	}
	for key := range counts {
		if strings.Contains(key, "203.0.113.9") {
			t.Fatalf("key exposes client IP: %q", key)
		}
	}
}

func TestRateLimitPassesRequestWhenLimiterFails(t *testing.T) {
	limiter := func(context.Context, string, int64, time.Duration) (bool, error) {
		return false, errors.New("redis down")
	}
	handler := RateLimit(limiter, 1, time.Minute, slog.New(slog.NewJSONHandler(io.Discard, nil)))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
}
