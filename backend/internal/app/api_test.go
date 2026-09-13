package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/airsss993/histproject/backend/internal/platform/config"
)

type passingChecker struct{}

func (passingChecker) Name() string {
	return "postgres"
}

func (passingChecker) Ping(context.Context) error {
	return nil
}

func TestNewAPIExposesHealthEndpoints(t *testing.T) {
	handler := NewAPI(testConfig(), testLogger(), Modules{}, passingChecker{})

	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
		})
	}
}

func TestNewAPIAppliesRequestIDAndSecurityHeaders(t *testing.T) {
	handler := NewAPI(testConfig(), testLogger(), Modules{}, passingChecker{})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is empty")
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestNewAPIReturnsJSONForUnknownRoute(t *testing.T) {
	handler := NewAPI(testConfig(), testLogger(), Modules{}, passingChecker{})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", body.Error.Code)
	}
}

func testConfig() config.Config {
	return config.Config{CORSOrigins: []string{"http://localhost:3000"}}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
