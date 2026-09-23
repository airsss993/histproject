package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
)

type adminReaderStub struct {
	email string
	err   error
}

func (stub *adminReaderStub) FindIdentityByEmail(_ context.Context, email string) (int64, string, string, string, error) {
	stub.email = email
	if stub.err != nil {
		return 0, "", "", "", stub.err
	}
	return 7, "root", "Иван", "super_admin", nil
}

func TestResolveUsesUserEmail(t *testing.T) {
	store := &adminReaderStub{}
	identity, err := NewService(store).Resolve(t.Context(), " ROOT@Example.Test ")
	if err != nil || store.email != "root@example.test" || identity.AdminID != 7 {
		t.Fatalf("identity = %#v, email = %q, err = %v", identity, store.email, err)
	}
	store.err = pgx.ErrNoRows
	if _, err := NewService(store).Resolve(t.Context(), "user@example.test"); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("error = %v", err)
	}
}

func TestUnifiedSessionAuthorization(t *testing.T) {
	store := &adminReaderStub{}
	email := ""
	handler := NewHandler(NewService(store), func(*http.Request) (string, error) {
		if email == "" {
			return "", ErrUnauthenticated
		}
		return email, nil
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	handler.Register(mux)
	check := func(want int) {
		t.Helper()
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil))
		if response.Code != want {
			t.Fatalf("status = %d, want %d", response.Code, want)
		}
	}
	check(http.StatusUnauthorized)
	email = "user@example.test"
	store.err = pgx.ErrNoRows
	check(http.StatusForbidden)
	store.err = nil
	check(http.StatusOK)
}
