package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

type HTTPService interface {
	Resolve(context.Context, string) (Identity, error)
}

type UserEmail func(*http.Request) (string, error)

type Handler struct {
	service   HTTPService
	userEmail UserEmail
	logger    *slog.Logger
}

type identityContextKey struct{}

func NewHandler(service HTTPService, userEmail UserEmail, logger *slog.Logger) *Handler {
	return &Handler{service: service, userEmail: userEmail, logger: logger}
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/auth/session", handler.Authenticate(http.HandlerFunc(handler.session)))
}

func (handler *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, err := handler.userEmail(r)
		if errors.Is(err, ErrUnauthenticated) {
			_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
			return
		}
		if err != nil {
			handler.writeInternal(w, r, "authenticate user", err)
			return
		}

		identity, err := handler.service.Resolve(r.Context(), email)
		if errors.Is(err, ErrNotAdmin) {
			_ = httpserver.WriteError(w, http.StatusForbidden, "forbidden", "Недостаточно прав", nil)
			return
		}
		if err != nil {
			handler.writeInternal(w, r, "resolve admin role", err)
			return
		}

		ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok && identity.Valid()
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFromContext(r.Context())
			if !ok {
				_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
				return
			}
			if _, ok := allowed[identity.Role]; !ok {
				_ = httpserver.WriteError(w, http.StatusForbidden, "forbidden", "Недостаточно прав", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (handler *Handler) session(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, sessionResponse(identity))
}

func sessionResponse(identity Identity) map[string]any {
	return map[string]any{"admin": map[string]any{
		"id": identity.AdminID, "login": identity.Login, "name": identity.Name, "role": identity.Role,
	}}
}

func (handler *Handler) writeInternal(w http.ResponseWriter, r *http.Request, message string, err error) {
	handler.logger.ErrorContext(r.Context(), message,
		"request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
	_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
}
