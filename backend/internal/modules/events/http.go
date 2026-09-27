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
	List(context.Context, ListInput, Viewer) (Page, error)
	Get(context.Context, int64, Viewer) (Event, error)
	UpdateCoordinates(context.Context, int64, float64, float64) error
	Delete(context.Context, int64) error
	Star(context.Context, int64, Viewer, bool) (StarState, error)
	StartView(context.Context, int64, Viewer, string) (int64, bool, error)
	TouchView(ctx context.Context, id, viewID int64, visitorID string, seconds int) error
}

type Middleware func(http.Handler) http.Handler

// ViewerReader - вошедший пользователь из сессии. Гость - нулевой Viewer без ошибки.
type ViewerReader func(*http.Request) (Viewer, error)

type Handler struct {
	service HTTPService
	logger  *slog.Logger
	viewer  ViewerReader
}

func NewHandler(service HTTPService, logger *slog.Logger, viewer ViewerReader) *Handler {
	return &Handler{service: service, logger: logger, viewer: viewer}
}

func (handler *Handler) Register(mux *http.ServeMux, protect Middleware) {
	mux.HandleFunc("GET /api/v1/events", handler.list)
	mux.HandleFunc("GET /api/v1/events/{id}", handler.get)
	mux.HandleFunc("PUT /api/v1/events/{id}/star", handler.star(true))
	mux.HandleFunc("DELETE /api/v1/events/{id}/star", handler.star(false))
	mux.HandleFunc("POST /api/v1/events/{id}/views", handler.startView)
	// POST, а не PATCH: при закрытии вкладки длительность уходит через sendBeacon, а он умеет только POST.
	mux.HandleFunc("POST /api/v1/events/{id}/views/{viewId}", handler.touchView)
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

	viewer, err := handler.viewer(r)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	result, err := handler.service.List(r.Context(), ListInput{
		EventTypes: r.URL.Query()["eventType"],
		DateFrom:   r.URL.Query().Get("dateFrom"),
		DateTo:     r.URL.Query().Get("dateTo"),
		Page:       page,
		PageSize:   pageSize,
	}, viewer)
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
	viewer, err := handler.viewer(r)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	event, err := handler.service.Get(r.Context(), id, viewer)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	response := newEventResponse(event)
	if event.Author.PublicID != "" {
		response.Author = &authorResponse{PublicID: event.Author.PublicID, FirstName: event.Author.FirstName, LastName: event.Author.LastName}
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]eventResponse{"event": response})
}

func (handler *Handler) star(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := positiveID(r.PathValue("id"))
		if !ok {
			handler.writeError(w, r, invalidID())
			return
		}
		viewer, err := handler.viewer(r)
		if err != nil {
			handler.writeError(w, r, err)
			return
		}
		state, err := handler.service.Star(r.Context(), id, viewer, on)
		if err != nil {
			handler.writeError(w, r, err)
			return
		}
		_ = httpserver.WriteJSON(w, http.StatusOK, state)
	}
}

func (handler *Handler) startView(w http.ResponseWriter, r *http.Request) {
	id, ok := positiveID(r.PathValue("id"))
	if !ok {
		handler.writeError(w, r, invalidID())
		return
	}
	var request struct {
		VisitorID string `json:"visitorId"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	viewer, err := handler.viewer(r)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	viewID, recorded, err := handler.service.StartView(r.Context(), id, viewer, request.VisitorID)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	// Автор смотрит свой сайт: просмотр не записан, дописывать нечего.
	if !recorded {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusCreated, map[string]int64{"viewId": viewID})
}

func (handler *Handler) touchView(w http.ResponseWriter, r *http.Request) {
	id, idOK := positiveID(r.PathValue("id"))
	viewID, viewOK := positiveID(r.PathValue("viewId"))
	if !idOK || !viewOK {
		handler.writeError(w, r, invalidID())
		return
	}
	var request struct {
		VisitorID     string `json:"visitorId"`
		ActiveSeconds *int   `json:"activeSeconds"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	if request.ActiveSeconds == nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"activeSeconds": "Обязательное поле"})
		return
	}
	if err := handler.service.TouchView(r.Context(), id, viewID, request.VisitorID, *request.ActiveSeconds); err != nil {
		handler.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	case errors.Is(err, ErrViewNotFound):
		_ = httpserver.WriteError(w, http.StatusNotFound, "view_not_found", "Просмотр не найден", nil)
	case errors.Is(err, ErrUnauthorized):
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
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
	Stars           int64   `json:"stars"`
	Starred         bool    `json:"starred"`
	Visitors        int64   `json:"visitors"`
	// Author есть только в GET /events/{id} и только у автора с учёткой.
	Author *authorResponse `json:"author,omitempty"`
}

type authorResponse struct {
	PublicID  string `json:"publicId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func newEventResponse(event Event) eventResponse {
	return eventResponse{
		ID: event.ID, Title: event.Title, Description: event.Description,
		Latitude: event.Latitude, Longitude: event.Longitude,
		EventDate: event.EventDate.Format(time.DateOnly), EventType: event.EventType,
		SiteURL: event.SiteURL, PreviewImageURL: event.PreviewImageURL,
		Stars: event.Stars, Starred: event.Starred, Visitors: event.Visitors,
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
