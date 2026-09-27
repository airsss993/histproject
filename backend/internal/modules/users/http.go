package users

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

// Общая сессия пользователя для публичного профиля и панели модерации.
const SessionCookieName = "histproject_user"

type Handler struct {
	service      *Service
	logger       *slog.Logger
	cookieDomain string
}

func NewHandler(service *Service, logger *slog.Logger, cookieDomain string) *Handler {
	return &Handler{service: service, logger: logger, cookieDomain: cookieDomain}
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/users/register", handler.register)
	mux.HandleFunc("POST /api/v1/users/login", handler.login)
	mux.HandleFunc("POST /api/v1/users/verify", handler.verify)
	mux.HandleFunc("GET /api/v1/users/me", handler.me)
	mux.HandleFunc("DELETE /api/v1/users/me", handler.deleteMe)
	mux.HandleFunc("PUT /api/v1/users/me/distribution", handler.distribution)
	mux.HandleFunc("GET /api/v1/users/{id}", handler.profile)
	mux.HandleFunc("POST /api/v1/users/logout", handler.logout)
}

func (handler *Handler) register(w http.ResponseWriter, r *http.Request) {
	var request RegisterInput
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	handler.codeSent(w, r, handler.service.Register(r.Context(), request, httpserver.ClientKey(r)))
}

func (handler *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	handler.codeSent(w, r, handler.service.RequestLogin(r.Context(), request.Email, httpserver.ClientKey(r)))
}

func (handler *Handler) codeSent(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	handler.writeError(w, r, err)
}

func (handler *Handler) verify(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Remember bool   `json:"remember"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	token, user, ttl, err := handler.service.Verify(r.Context(), request.Email, request.Code, request.Remember)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	cookie := &http.Cookie{
		Name: SessionCookieName, Value: token, Path: "/", Domain: handler.cookieDomain,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	}
	// Без "запомнить" кука живёт до закрытия браузера, сессия в Redis - SESSION_TTL.
	if request.Remember {
		cookie.MaxAge = int(ttl / time.Second)
		cookie.Expires = time.Now().Add(ttl)
	}
	http.SetCookie(w, cookie)
	handler.clearLegacyAdminCookie(w)
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (handler *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := handler.service.Authenticate(r.Context(), sessionToken(r))
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

// distribution - переключатель "показывать фамилию полностью": выдача или отзыв согласия на распространение.
func (handler *Handler) distribution(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	user, err := handler.service.SetDistribution(r.Context(), sessionToken(r), request.Enabled)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (handler *Handler) deleteMe(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteAccount(r.Context(), sessionToken(r)); err != nil {
		handler.writeError(w, r, err)
		return
	}
	handler.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

// profile - публичный профиль автора. Заявки в нём режет сервис: гостю только опубликованные.
func (handler *Handler) profile(w http.ResponseWriter, r *http.Request) {
	profile, err := handler.service.Profile(r.Context(), r.PathValue("id"), sessionToken(r))
	if errors.Is(err, ErrUserNotFound) {
		_ = httpserver.WriteError(w, http.StatusNotFound, "user_not_found", "Аккаунт не найден", nil)
		return
	}
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, profile)
}

func (handler *Handler) logout(w http.ResponseWriter, r *http.Request) {
	err := handler.service.Logout(r.Context(), sessionToken(r))
	handler.clearSession(w)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName, Path: "/", Domain: handler.cookieDomain, Expires: time.Unix(1, 0), MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	handler.clearLegacyAdminCookie(w)
}

func (handler *Handler) clearLegacyAdminCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "histproject_session", Path: "/", Domain: handler.cookieDomain, Expires: time.Unix(1, 0), MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}

func (handler *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var fields ValidationError
	switch {
	case errors.As(err, &fields):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
	case errors.Is(err, ErrEmailTaken):
		_ = httpserver.WriteError(w, http.StatusConflict, "email_taken", "Эта почта уже зарегистрирована", nil)
	case errors.Is(err, ErrUserNotFound):
		_ = httpserver.WriteError(w, http.StatusNotFound, "user_not_found", "Аккаунт с этой почтой не найден", nil)
	case errors.Is(err, ErrInvalidCode):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_code", "Неверный код", nil)
	case errors.Is(err, ErrCodeExpired):
		_ = httpserver.WriteError(w, http.StatusGone, "code_expired", "Код устарел, запросите новый", nil)
	case errors.Is(err, ErrRateLimited):
		w.Header().Set("Retry-After", strconv.Itoa(int(codeCooldown/time.Second)))
		_ = httpserver.WriteError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов", nil)
	case errors.Is(err, ErrNoSession):
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
	default:
		handler.logger.ErrorContext(r.Context(), "users request failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()),
			"error", err,
		)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
	}
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
