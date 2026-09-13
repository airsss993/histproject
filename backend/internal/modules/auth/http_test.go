package auth

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

func TestLoginSetsSecureSessionCookie(t *testing.T) {
	service := &authServiceStub{
		loginToken:    "session-token",
		loginIdentity: Identity{AdminID: 7, Login: "root", Role: "super_admin"},
	}
	handler := NewHandler(service, nil, discardLogger(), "example.test", time.Hour)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"login":"root","password":"correct"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if service.login != "root" || service.password != "correct" {
		t.Fatalf("credentials passed to service = (%q, %q)", service.login, service.password)
	}
	result := response.Result()
	t.Cleanup(func() { _ = result.Body.Close() })
	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != SessionCookieName || cookie.Value != "session-token" || cookie.Path != "/" || cookie.Domain != "example.test" {
		t.Fatalf("cookie = %#v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 3600 {
		t.Fatalf("cookie flags = HttpOnly:%t Secure:%t SameSite:%d MaxAge:%d", cookie.HttpOnly, cookie.Secure, cookie.SameSite, cookie.MaxAge)
	}

	var body struct {
		Admin struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
			Role  string `json:"role"`
		} `json:"admin"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Admin.ID != 7 || body.Admin.Login != "root" || body.Admin.Role != "super_admin" {
		t.Fatalf("response admin = %#v", body.Admin)
	}
}

func TestLoginRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(&authServiceStub{}, nil, discardLogger(), "", time.Hour)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"login":"root","unexpected":true}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertErrorResponse(t, response, http.StatusBadRequest, "invalid_json")
}

func TestLoginUsesGenericInvalidCredentialsError(t *testing.T) {
	service := &authServiceStub{loginError: ErrInvalidCredentials}
	handler := NewHandler(service, nil, discardLogger(), "", time.Hour)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"login":"missing","password":"wrong"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertErrorResponse(t, response, http.StatusUnauthorized, "invalid_credentials")
	if strings.Contains(response.Body.String(), "missing") || strings.Contains(response.Body.String(), "wrong") {
		t.Fatalf("response exposes credentials: %s", response.Body.String())
	}
}

func TestSessionRequiresValidCookie(t *testing.T) {
	handler := NewHandler(&authServiceStub{authenticateError: ErrInvalidSession}, nil, discardLogger(), "", time.Hour)
	mux := http.NewServeMux()
	handler.Register(mux)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil))

	assertErrorResponse(t, response, http.StatusUnauthorized, "unauthorized")
}

func TestSessionReturnsAuthenticatedIdentity(t *testing.T) {
	service := &authServiceStub{
		authenticateIdentity: Identity{AdminID: 9, Login: "admin", Role: "admin"},
	}
	handler := NewHandler(service, nil, discardLogger(), "", time.Hour)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if service.authenticateToken != "session-token" {
		t.Fatalf("authenticate token = %q", service.authenticateToken)
	}
	var body struct {
		Admin struct {
			ID int64 `json:"id"`
		} `json:"admin"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Admin.ID != 9 {
		t.Fatalf("admin ID = %d, want 9", body.Admin.ID)
	}
}

func TestLogoutDeletesSessionAndClearsCookie(t *testing.T) {
	service := &authServiceStub{}
	handler := NewHandler(service, nil, discardLogger(), "example.test", time.Hour)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if service.logoutToken != "session-token" {
		t.Fatalf("logout token = %q", service.logoutToken)
	}
	result := response.Result()
	t.Cleanup(func() { _ = result.Body.Close() })
	cookies := result.Cookies()
	if len(cookies) != 1 || cookies[0].Name != SessionCookieName || cookies[0].MaxAge >= 0 {
		t.Fatalf("cleared cookies = %#v", cookies)
	}
}

func TestRequireRoleUsesIdentityFromContext(t *testing.T) {
	called := false
	handler := RequireRole("super_admin")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityContextKey{}, Identity{AdminID: 1, Role: "admin", Login: "admin"}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if called {
		t.Fatal("protected handler was called")
	}
	assertErrorResponse(t, response, http.StatusForbidden, "forbidden")
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d", response.Code, status)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

type authServiceStub struct {
	login                string
	password             string
	loginToken           string
	loginIdentity        Identity
	loginError           error
	authenticateToken    string
	authenticateIdentity Identity
	authenticateError    error
	logoutToken          string
	logoutError          error
}

func (stub *authServiceStub) Login(_ context.Context, login, password string) (string, Identity, error) {
	stub.login = login
	stub.password = password
	return stub.loginToken, stub.loginIdentity, stub.loginError
}

func (stub *authServiceStub) Authenticate(_ context.Context, token string) (Identity, error) {
	stub.authenticateToken = token
	return stub.authenticateIdentity, stub.authenticateError
}

func (stub *authServiceStub) Logout(_ context.Context, token string) error {
	stub.logoutToken = token
	return stub.logoutError
}
