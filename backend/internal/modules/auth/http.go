package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

const SessionCookieName = "histproject_session"

type HTTPService interface {
	Login(context.Context, string, string) (string, Identity, error)
	Authenticate(context.Context, string) (Identity, error)
	Logout(context.Context, string) error
}

type LoginLimiter interface {
	Allow(context.Context, string, string) (bool, error)
}

type Handler struct {
	service      HTTPService
	loginLimiter LoginLimiter
	logger       *slog.Logger
	cookieDomain string
	sessionTTL   time.Duration
}

type identityContextKey struct{}

func NewHandler(service HTTPService, loginLimiter LoginLimiter, logger *slog.Logger, cookieDomain string, sessionTTL time.Duration) *Handler {
	return &Handler{
		service:      service,
		loginLimiter: loginLimiter,
		logger:       logger,
		cookieDomain: cookieDomain,
		sessionTTL:   sessionTTL,
	}
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.HandleFunc("POST /api/v1/auth/logout", handler.logout)
	mux.Handle("GET /api/v1/auth/session", handler.Authenticate(http.HandlerFunc(handler.session)))
}

func (handler *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
			return
		}

		identity, err := handler.service.Authenticate(r.Context(), cookie.Value)
		if errors.Is(err, ErrInvalidSession) {
			_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
			return
		}
		if err != nil {
			handler.logger.ErrorContext(r.Context(), "authenticate session failed",
				"request_id", httpserver.RequestIDFromContext(r.Context()),
				"error", err,
			)
			_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
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

func (handler *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	fields := loginFields(request.Login, request.Password)
	if len(fields) > 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return
	}
	if handler.loginLimiter != nil {
		allowed, err := handler.loginLimiter.Allow(r.Context(), request.Login, r.RemoteAddr)
		if err != nil {
			handler.logger.ErrorContext(r.Context(), "auth rate limit failed",
				"request_id", httpserver.RequestIDFromContext(r.Context()),
				"error", err,
			)
			_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", "60")
			_ = httpserver.WriteError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много попыток", nil)
			return
		}
	}

	token, identity, err := handler.service.Login(r.Context(), request.Login, request.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный логин или пароль", nil)
		return
	}
	if err != nil {
		handler.logger.ErrorContext(r.Context(), "auth login failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()),
			"error", err,
		)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
		return
	}

	handler.setSessionCookie(w, token)
	_ = httpserver.WriteJSON(w, http.StatusOK, sessionResponse(identity))
}

func (handler *Handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(SessionCookieName)
	if err == nil {
		err = handler.service.Logout(r.Context(), cookie.Value)
	}
	handler.clearSessionCookie(w)
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		handler.logger.ErrorContext(r.Context(), "auth logout failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()),
			"error", err,
		)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) session(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, sessionResponse(identity))
}

func (handler *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	maxAge := int(handler.sessionTTL / time.Second)
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Domain:   handler.cookieDomain,
		Expires:  time.Now().Add(handler.sessionTTL),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Path:     "/",
		Domain:   handler.cookieDomain,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func loginFields(login, password string) map[string]string {
	fields := make(map[string]string)
	login = strings.TrimSpace(login)
	if login == "" {
		fields["login"] = "Обязательное поле"
	} else if len(login) > 100 {
		fields["login"] = "Не более 100 символов"
	}
	if password == "" {
		fields["password"] = "Обязательное поле"
	} else if len(password) > 128 {
		fields["password"] = "Не более 128 символов"
	}
	return fields
}

func sessionResponse(identity Identity) map[string]any {
	return map[string]any{
		"admin": map[string]any{
			"id":    identity.AdminID,
			"login": identity.Login,
			"role":  identity.Role,
		},
	}
}
