package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteErrorUsesUnifiedContract(t *testing.T) {
	response := httptest.NewRecorder()

	if err := WriteError(response, http.StatusBadRequest, "validation_failed", "Invalid request", map[string]string{
		"login": "required",
	}); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}

	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "validation_failed" || body.Error.Message != "Invalid request" {
		t.Fatalf("error body = %#v", body.Error)
	}
	if body.Error.Fields["login"] != "required" {
		t.Fatalf("fields = %#v", body.Error.Fields)
	}
}

func TestDecodeJSONAcceptsSingleObject(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":"admin"}`))
	response := httptest.NewRecorder()
	var body struct {
		Login string `json:"login"`
	}

	if err := DecodeJSON(response, request, &body); err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if body.Login != "admin" {
		t.Fatalf("login = %q", body.Login)
	}
}

func TestDecodeJSONRejectsUnknownField(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":"admin","role":"root"}`))
	response := httptest.NewRecorder()
	var body struct {
		Login string `json:"login"`
	}

	err := DecodeJSON(response, request, &body)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("DecodeJSON() error = %v, want ErrInvalidJSON", err)
	}
}

func TestDecodeJSONRejectsSecondValue(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":"admin"}{"login":"root"}`))
	response := httptest.NewRecorder()
	var body map[string]string

	err := DecodeJSON(response, request, &body)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("DecodeJSON() error = %v, want ErrInvalidJSON", err)
	}
}

func TestDecodeJSONLimitsBody(t *testing.T) {
	body := bytes.Repeat([]byte("a"), MaxJSONBodyBytes+1)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	response := httptest.NewRecorder()
	var value any

	err := DecodeJSON(response, request, &value)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("DecodeJSON() error = %v, want ErrInvalidJSON", err)
	}
}
