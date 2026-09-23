package admins

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

func TestCreateAdminDoesNotReturnPassword(t *testing.T) {
	service := &adminServiceStub{
		createdAdmin: Admin{ID: 8, Login: "new-admin", Email: "admin@example.test", Role: RoleAdmin},
	}
	handler := NewHandler(service, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 1, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, passThrough)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/admins", strings.NewReader(`{"name":"Иван","email":"admin@example.test","role":"admin"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", response.Code)
	}
	if service.createInput.Email != "admin@example.test" || service.createInput.Role != RoleAdmin {
		t.Fatalf("create input = %#v", service.createInput)
	}
	if strings.Contains(response.Body.String(), "long-enough-password") || strings.Contains(response.Body.String(), "password") {
		t.Fatalf("response exposes password: %s", response.Body.String())
	}
}

func TestCreateAdminMapsValidationError(t *testing.T) {
	service := &adminServiceStub{createError: &ValidationError{Fields: map[string]string{"login": "invalid"}}}
	handler := NewHandler(service, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 1, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, passThrough)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/admins", strings.NewReader(`{"name":"Иван","email":"bad","role":"admin"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertAdminError(t, response, http.StatusBadRequest, "validation_failed", "login")
}

func TestListAdminsReturnsItems(t *testing.T) {
	service := &adminServiceStub{admins: []Admin{
		{ID: 1, Login: "root", Role: RoleSuperAdmin, CreatedAt: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)},
	}}
	handler := NewHandler(service, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 1, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, passThrough)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/admins", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body struct {
		Items []Admin `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Login != "root" {
		t.Fatalf("items = %#v", body.Items)
	}
}

func TestChangeRoleUsesCallerIdentity(t *testing.T) {
	service := &adminServiceStub{changedAdmin: Admin{ID: 8, Login: "admin", Role: RoleSuperAdmin}}
	handler := NewHandler(service, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 3, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, passThrough)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/admins/8/role", strings.NewReader(`{"role":"super_admin"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if service.callerID != 3 || service.targetID != 8 || service.role != RoleSuperAdmin {
		t.Fatalf("ChangeRole args = caller:%d target:%d role:%s", service.callerID, service.targetID, service.role)
	}
}

func TestDeleteMapsLastSuperAdminConflict(t *testing.T) {
	service := &adminServiceStub{deleteError: ErrLastSuperAdmin}
	handler := NewHandler(service, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 3, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, passThrough)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/admins/8", nil))

	assertAdminError(t, response, http.StatusConflict, "last_super_admin", "")
}

func TestRoutesUseProtectionMiddleware(t *testing.T) {
	protected := false
	protect := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			protected = true
			next.ServeHTTP(w, r)
		})
	}
	handler := NewHandler(&adminServiceStub{}, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 1, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, protect)

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/admin/admins", nil))

	if !protected {
		t.Fatal("protection middleware was not called")
	}
}

func TestUpdateLeavesMissingFieldsNil(t *testing.T) {
	service := &adminServiceStub{changedAdmin: Admin{ID: 8, Login: "admin", Name: "Пётр", Role: RoleAdmin}}
	handler := NewHandler(service, adminDiscardLogger(), func(context.Context) (Actor, bool) {
		return Actor{AdminID: 3, Role: RoleSuperAdmin}, true
	})
	mux := http.NewServeMux()
	handler.Register(mux, passThrough)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/api/v1/admin/admins/8", strings.NewReader(`{"name":"Пётр"}`)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	in := service.updateInput
	if service.callerID != 3 || service.targetID != 8 || in.Name == nil || *in.Name != "Пётр" || in.Email != nil {
		t.Fatalf("Update args = caller:%d target:%d input:%#v", service.callerID, service.targetID, in)
	}
}

func passThrough(next http.Handler) http.Handler {
	return next
}

func adminDiscardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func assertAdminError(t *testing.T, response *httptest.ResponseRecorder, status int, code, field string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d", response.Code, status)
	}
	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
	if field != "" && body.Error.Fields[field] == "" {
		t.Fatalf("error fields = %#v, want %s", body.Error.Fields, field)
	}
}

type adminServiceStub struct {
	createInput  CreateInput
	createdAdmin Admin
	createError  error
	admins       []Admin
	listError    error
	callerID     int64
	targetID     int64
	role         Role
	changedAdmin Admin
	changeError  error
	deleteError  error
	updateInput  UpdateInput
	updateError  error
}

func (stub *adminServiceStub) Create(_ context.Context, input CreateInput) (Admin, error) {
	stub.createInput = input
	return stub.createdAdmin, stub.createError
}

func (stub *adminServiceStub) List(context.Context) ([]Admin, error) {
	return stub.admins, stub.listError
}

func (stub *adminServiceStub) ChangeRole(_ context.Context, callerID, targetID int64, role Role) (Admin, error) {
	stub.callerID = callerID
	stub.targetID = targetID
	stub.role = role
	return stub.changedAdmin, stub.changeError
}

func (stub *adminServiceStub) Delete(_ context.Context, callerID, targetID int64) error {
	stub.callerID = callerID
	stub.targetID = targetID
	return stub.deleteError
}

func (stub *adminServiceStub) Update(_ context.Context, callerID, targetID int64, input UpdateInput) (Admin, error) {
	stub.callerID = callerID
	stub.targetID = targetID
	stub.updateInput = input
	return stub.changedAdmin, stub.updateError
}
