package admins

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

type Actor struct {
	AdminID int64
	Role    Role
}

type ActorReader func(context.Context) (Actor, bool)

type Middleware func(http.Handler) http.Handler

type HTTPService interface {
	Create(context.Context, CreateInput) (Admin, error)
	List(context.Context) ([]Admin, error)
	ChangeRole(context.Context, int64, int64, Role) (Admin, error)
	Delete(context.Context, int64, int64) error
	Update(context.Context, int64, int64, UpdateInput) (Admin, error)
	Get(context.Context, int64) (Admin, error)
	UpdateProfile(context.Context, int64, string, string) (Admin, error)
	ChangePassword(context.Context, int64, string, string) error
}

type Handler struct {
	service     HTTPService
	logger      *slog.Logger
	actorReader ActorReader
}

func NewHandler(service HTTPService, logger *slog.Logger, actorReader ActorReader) *Handler {
	return &Handler{service: service, logger: logger, actorReader: actorReader}
}

// Register: superAdmin закрывает управление модераторами, anyAdmin - собственный профиль.
func (handler *Handler) Register(mux *http.ServeMux, superAdmin, anyAdmin Middleware) {
	mux.Handle("GET /api/v1/admin/admins", superAdmin(http.HandlerFunc(handler.list)))
	mux.Handle("POST /api/v1/admin/admins", superAdmin(http.HandlerFunc(handler.create)))
	mux.Handle("PATCH /api/v1/admin/admins/{id}", superAdmin(http.HandlerFunc(handler.update)))
	mux.Handle("PATCH /api/v1/admin/admins/{id}/role", superAdmin(http.HandlerFunc(handler.changeRole)))
	mux.Handle("DELETE /api/v1/admin/admins/{id}", superAdmin(http.HandlerFunc(handler.delete)))
	mux.Handle("GET /api/v1/admin/profile", anyAdmin(http.HandlerFunc(handler.profile)))
	mux.Handle("PATCH /api/v1/admin/profile", anyAdmin(http.HandlerFunc(handler.updateProfile)))
	mux.Handle("PUT /api/v1/admin/profile/password", anyAdmin(http.HandlerFunc(handler.changePassword)))
}

func (handler *Handler) create(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Login    string `json:"login"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Role     Role   `json:"role"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}

	admin, err := handler.service.Create(r.Context(), CreateInput{
		Login:    request.Login,
		Name:     request.Name,
		Password: request.Password,
		Email:    request.Email,
		Role:     request.Role,
	})
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusCreated, map[string]Admin{"admin": admin})
}

func (handler *Handler) list(w http.ResponseWriter, r *http.Request) {
	admins, err := handler.service.List(r.Context())
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	if admins == nil {
		admins = []Admin{}
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string][]Admin{"items": admins})
}

func (handler *Handler) changeRole(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseAdminID(r.PathValue("id"))
	if !ok {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"id": "Ожидается положительный ID"})
		return
	}
	var request struct {
		Role Role `json:"role"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	actor, ok := handler.actorReader(r.Context())
	if !ok {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
		return
	}

	admin, err := handler.service.ChangeRole(r.Context(), actor.AdminID, targetID, request.Role)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]Admin{"admin": admin})
}

func (handler *Handler) delete(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseAdminID(r.PathValue("id"))
	if !ok {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"id": "Ожидается положительный ID"})
		return
	}
	actor, ok := handler.actorReader(r.Context())
	if !ok {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
		return
	}

	if err := handler.service.Delete(r.Context(), actor.AdminID, targetID); err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) update(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseAdminID(r.PathValue("id"))
	if !ok {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"id": "Ожидается положительный ID"})
		return
	}
	var request struct {
		Name     *string `json:"name"`
		Email    *string `json:"email"`
		Password string  `json:"password"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	actor, ok := handler.actor(w, r)
	if !ok {
		return
	}

	admin, err := handler.service.Update(r.Context(), actor.AdminID, targetID, UpdateInput{Name: request.Name, Email: request.Email, Password: request.Password})
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]Admin{"admin": admin})
}

func (handler *Handler) profile(w http.ResponseWriter, r *http.Request) {
	actor, ok := handler.actor(w, r)
	if !ok {
		return
	}
	admin, err := handler.service.Get(r.Context(), actor.AdminID)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]Admin{"admin": admin})
}

func (handler *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	actor, ok := handler.actor(w, r)
	if !ok {
		return
	}
	admin, err := handler.service.UpdateProfile(r.Context(), actor.AdminID, request.Name, request.Email)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]Admin{"admin": admin})
}

func (handler *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	actor, ok := handler.actor(w, r)
	if !ok {
		return
	}
	if err := handler.service.ChangePassword(r.Context(), actor.AdminID, request.CurrentPassword, request.Password); err != nil {
		handler.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) actor(w http.ResponseWriter, r *http.Request) (Actor, bool) {
	actor, ok := handler.actorReader(r.Context())
	if !ok {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
	}
	return actor, ok
}

func (handler *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var validationError *ValidationError
	switch {
	case errors.As(err, &validationError):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", validationError.Fields)
	case errors.Is(err, ErrLoginTaken):
		_ = httpserver.WriteError(w, http.StatusConflict, "login_taken", "Логин уже занят", nil)
	case errors.Is(err, ErrAdminNotFound):
		_ = httpserver.WriteError(w, http.StatusNotFound, "admin_not_found", "Администратор не найден", nil)
	case errors.Is(err, ErrSelfMutation):
		_ = httpserver.WriteError(w, http.StatusConflict, "self_mutation", "Нельзя изменить или удалить себя", nil)
	case errors.Is(err, ErrLastSuperAdmin):
		_ = httpserver.WriteError(w, http.StatusConflict, "last_super_admin", "Нельзя изменить или удалить последнего суперадминистратора", nil)
	default:
		handler.logger.ErrorContext(r.Context(), "admin request failed",
			"request_id", httpserver.RequestIDFromContext(r.Context()),
			"error", err,
		)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
	}
}

func parseAdminID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}
