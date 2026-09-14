package events

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

type HTTPService interface {
	List(context.Context, ListInput) (Page, error)
	Get(context.Context, int64) (Event, error)
	UpdateCoordinates(context.Context, int64, float64, float64) error
	Delete(context.Context, int64) error
}

type Middleware func(http.Handler) http.Handler

type Handler struct {
	service HTTPService
	logger  *slog.Logger
}

func NewHandler(service HTTPService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (handler *Handler) Register(mux *http.ServeMux, protect Middleware) {
	mux.HandleFunc("GET /api/v1/events", handler.list)
	mux.HandleFunc("GET /api/v1/events/{id}", handler.get)
	mux.Handle("PATCH /api/v1/admin/events/{id}/coordinates", protect(http.HandlerFunc(handler.updateCoordinates)))
	mux.Handle("DELETE /api/v1/admin/events/{id}", protect(http.HandlerFunc(handler.delete)))
}

func (handler *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, pageOK := optionalPositiveInt(r.URL.Query().Get("page"))
	pageSize, pageSizeOK := optionalPositiveInt(r.URL.Query().Get("pageSize"))
	if !pageOK || !pageSizeOK {
		fields := make(map[string]string)
		if !pageOK {
			fields["page"] = "Ожидается положительное число"
		}
		if !pageSizeOK {
			fields["pageSize"] = "Ожидается положительное число"
		}
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return
	}

	result, err := handler.service.List(r.Context(), ListInput{
		EventTypes: r.URL.Query()["eventType"],
		DateFrom:   r.URL.Query().Get("dateFrom"),
		DateTo:     r.URL.Query().Get("dateTo"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	items := make([]eventResponse, 0, len(result.Items))
	for _, event := range result.Items {
		items = append(items, newEventResponse(event))
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": result.Page, "pageSize": result.PageSize, "total": result.Total,
	})
}

func (handler *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := positiveID(r.PathValue("id"))
	if !ok {
		handler.writeError(w, r, invalidID())
		return
	}
	event, err := handler.service.Get(r.Context(), id)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]eventResponse{"event": newEventResponse(event)})
}

func (handler *Handler) updateCoordinates(w http.ResponseWriter, r *http.Request) {
	id, ok := positiveID(r.PathValue("id"))
	if !ok {
		handler.writeError(w, r, invalidID())
		return
	}
	var request struct {
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	fields := make(map[string]string)
	if request.Latitude == nil {
		fields["latitude"] = "Обязательное поле"
	}
	if request.Longitude == nil {
		fields["longitude"] = "Обязательное поле"
	}
	if len(fields) > 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return
	}
	if err := handler.service.UpdateCoordinates(r.Context(), id, *request.Latitude, *request.Longitude); err != nil {
		handler.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := positiveID(r.PathValue("id"))
	if !ok {
		handler.writeError(w, r, invalidID())
		return
	}
	if err := handler.service.Delete(r.Context(), id); err != nil {
		handler.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var validationError *ValidationError
	switch {
	case errors.As(err, &validationError):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", validationError.Fields)
	case errors.Is(err, ErrEventNotFound):
		_ = httpserver.WriteError(w, http.StatusNotFound, "event_not_found", "Событие не найдено", nil)
	default:
		handler.logger.ErrorContext(r.Context(), "event request failed", "request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
	}
}

type eventResponse struct {
	ID              int64   `json:"id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	EventDate       string  `json:"eventDate"`
	EventType       string  `json:"eventType"`
	SiteURL         string  `json:"siteUrl"`
	PreviewImageURL *string `json:"previewImageUrl"`
}

func newEventResponse(event Event) eventResponse {
	return eventResponse{
		ID: event.ID, Title: event.Title, Description: event.Description,
		Latitude: event.Latitude, Longitude: event.Longitude,
		EventDate: event.EventDate.Format(time.DateOnly), EventType: event.EventType,
		SiteURL: event.SiteURL, PreviewImageURL: event.PreviewImageURL,
	}
}

func positiveID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

func optionalPositiveInt(value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}
