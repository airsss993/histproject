package submissions

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

const multipartOverheadBytes int64 = 1 << 20

// uploadTimeout replaces the server-wide read and write timeouts for archive uploads: 20 MB over a 1 Mbit/s uplink takes under 3 minutes.
const uploadTimeout = 10 * time.Minute

type HTTPService interface {
	Create(context.Context, CreateInput) (Submission, error)
	List(context.Context, ListFilter) (SubmissionPage, error)
	Get(context.Context, int64) (Submission, error)
	Review(context.Context, int64, Actor) (Submission, error)
	Reject(context.Context, int64, Actor, string) (Submission, error)
	Publish(context.Context, int64, Actor, float64, float64, string) (Submission, error)
	AuditLog(context.Context, AuditFilter) (AuditPage, error)
}

type CreateLimiter interface {
	Allow(context.Context, string) (bool, error)
}

type ActorReader func(context.Context) (Actor, bool)

// AuthorReader отдаёт email вошедшего посетителя. Без сессии - ErrUnauthorized.
type AuthorReader func(*http.Request) (string, error)

var ErrUnauthorized = errors.New("visitor session required")

type Middleware func(http.Handler) http.Handler

type Handler struct {
	service        HTTPService
	limiter        CreateLimiter
	logger         *slog.Logger
	actorReader    ActorReader
	authorReader   AuthorReader
	maxUploadBytes int64
}

func NewHandler(service HTTPService, limiter CreateLimiter, logger *slog.Logger, actorReader ActorReader, authorReader AuthorReader, maxUploadBytes int64) *Handler {
	return &Handler{service: service, limiter: limiter, logger: logger, actorReader: actorReader, authorReader: authorReader, maxUploadBytes: maxUploadBytes}
}

func (handler *Handler) Register(mux *http.ServeMux, protectAdmin, protectSuperAdmin Middleware) {
	mux.HandleFunc("POST /api/v1/submissions", handler.create)
	mux.Handle("GET /api/v1/admin/audit-log", protectSuperAdmin(http.HandlerFunc(handler.auditLog)))
	mux.Handle("GET /api/v1/admin/submissions", protectAdmin(http.HandlerFunc(handler.list)))
	mux.Handle("GET /api/v1/admin/submissions/{id}", protectAdmin(http.HandlerFunc(handler.get)))
	mux.Handle("POST /api/v1/admin/submissions/{id}/review", protectAdmin(http.HandlerFunc(handler.review)))
	mux.Handle("POST /api/v1/admin/submissions/{id}/reject", protectAdmin(http.HandlerFunc(handler.reject)))
	mux.Handle("POST /api/v1/admin/submissions/{id}/publish", protectAdmin(http.HandlerFunc(handler.publish)))
}

func (handler *Handler) create(w http.ResponseWriter, r *http.Request) {
	// Сессию проверяем до чтения тела: гость не должен заливать архив, который всё равно отвергнем.
	email, err := handler.authorReader(r)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(uploadTimeout))
	_ = controller.SetWriteDeadline(time.Now().Add(uploadTimeout))
	r.Body = http.MaxBytesReader(w, r.Body, handler.maxUploadBytes+multipartOverheadBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			_ = httpserver.WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Слишком большой файл", nil)
			return
		}
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_multipart", "Некорректная форма", nil)
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	archive, archiveHeader, archiveErr := r.FormFile("archive")
	if archiveErr == nil {
		defer func() { _ = archive.Close() }()
	}
	eventDate, dateErr := time.Parse(time.DateOnly, strings.TrimSpace(r.FormValue("eventDate")))
	latitude, latitudeErr := strconv.ParseFloat(strings.TrimSpace(r.FormValue("latitude")), 64)
	longitude, longitudeErr := strconv.ParseFloat(strings.TrimSpace(r.FormValue("longitude")), 64)
	input := CreateInput{
		Title: r.FormValue("title"), Description: r.FormValue("description"), EventDate: eventDate,
		EventType: r.FormValue("eventType"), Email: email,
		TelegramUsername: r.FormValue("telegramUsername"), Latitude: latitude, Longitude: longitude, Archive: archive,
		Consent: r.FormValue("consent") == "true", Rules: r.FormValue("rules") == "true",
	}
	if archiveHeader != nil {
		input.ArchiveName = archiveHeader.Filename
		input.ArchiveSize = archiveHeader.Size
	}
	fields := validateCreateInput(input, handler.maxUploadBytes)
	if dateErr != nil {
		fields["eventDate"] = "Ожидается дата YYYY-MM-DD"
	}
	if latitudeErr != nil {
		fields["latitude"] = "Поставьте точку на карте"
	}
	if longitudeErr != nil {
		fields["longitude"] = "Поставьте точку на карте"
	}
	if archiveErr != nil {
		fields["archive"] = "Обязательное поле"
	}
	if len(fields) > 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return
	}

	if handler.limiter != nil {
		allowed, err := handler.limiter.Allow(r.Context(), httpserver.ClientKey(r))
		if err != nil {
			handler.writeError(w, r, err)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.FormatInt(int64(submissionRateWindow/time.Second), 10))
			_ = httpserver.WriteError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много заявок", nil)
			return
		}
	}

	submission, err := handler.service.Create(r.Context(), input)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusCreated, map[string]any{
		"submission": map[string]any{"id": submission.ID, "publicId": submission.PublicID, "status": submission.Status},
	})
}

func (handler *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := paginationParams(w, r)
	if !ok {
		return
	}
	result, err := handler.service.List(r.Context(), ListFilter{
		Status: Status(r.URL.Query().Get("status")), Search: r.URL.Query().Get("q"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	items := make([]submissionResponse, 0, len(result.Items))
	for _, submission := range result.Items {
		items = append(items, newSubmissionResponse(submission))
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": result.Page, "pageSize": result.PageSize, "total": result.Total,
	})
}

func (handler *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := submissionID(r.PathValue("id"))
	if !ok {
		handler.writeError(w, r, invalidSubmissionID())
		return
	}
	submission, err := handler.service.Get(r.Context(), id)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]submissionResponse{"submission": newSubmissionResponse(submission)})
}

func (handler *Handler) review(w http.ResponseWriter, r *http.Request) {
	handler.moderate(w, r, func(ctx context.Context, id int64, actor Actor) (Submission, error) {
		return handler.service.Review(ctx, id, actor)
	})
}

func (handler *Handler) reject(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Comment string `json:"comment"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	handler.moderate(w, r, func(ctx context.Context, id int64, actor Actor) (Submission, error) {
		return handler.service.Reject(ctx, id, actor, request.Comment)
	})
}

func (handler *Handler) publish(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
		EventType string   `json:"eventType"`
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
	handler.moderate(w, r, func(ctx context.Context, id int64, actor Actor) (Submission, error) {
		return handler.service.Publish(ctx, id, actor, *request.Latitude, *request.Longitude, request.EventType)
	})
}

func (handler *Handler) moderate(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, Actor) (Submission, error)) {
	id, ok := submissionID(r.PathValue("id"))
	if !ok {
		handler.writeError(w, r, invalidSubmissionID())
		return
	}
	actor, ok := handler.readActor(r.Context())
	if !ok {
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация", nil)
		return
	}
	submission, err := action(r.Context(), id, actor)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]submissionResponse{"submission": newSubmissionResponse(submission)})
}

func (handler *Handler) auditLog(w http.ResponseWriter, r *http.Request) {
	page, pageSize, ok := paginationParams(w, r)
	if !ok {
		return
	}
	result, err := handler.service.AuditLog(r.Context(), AuditFilter{Search: r.URL.Query().Get("q"), Page: page, PageSize: pageSize})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	items := make([]auditResponse, 0, len(result.Items))
	for _, entry := range result.Items {
		items = append(items, newAuditResponse(entry))
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": result.Page, "pageSize": result.PageSize, "total": result.Total,
	})
}

func (handler *Handler) readActor(ctx context.Context) (Actor, bool) {
	if handler.actorReader == nil {
		return Actor{}, false
	}
	return handler.actorReader(ctx)
}

func (handler *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var validationError *ValidationError
	switch {
	case errors.As(err, &validationError):
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", validationError.Fields)
	case errors.Is(err, ErrUnauthorized):
		_ = httpserver.WriteError(w, http.StatusUnauthorized, "unauthorized", "Войдите, чтобы предложить событие", nil)
	case errors.Is(err, ErrSubmissionNotFound):
		_ = httpserver.WriteError(w, http.StatusNotFound, "submission_not_found", "Заявка не найдена", nil)
	case errors.Is(err, ErrInvalidTransition):
		_ = httpserver.WriteError(w, http.StatusConflict, "invalid_transition", "Переход статуса недоступен", nil)
	default:
		handler.logger.ErrorContext(r.Context(), "submission request failed", "request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
		_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
	}
}

type submissionResponse struct {
	ID               int64    `json:"id"`
	PublicID         string   `json:"publicId"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	EventDate        string   `json:"eventDate"`
	EventType        string   `json:"eventType"`
	Email            string   `json:"email"`
	TelegramUsername string   `json:"telegramUsername"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	SiteURL          string   `json:"siteUrl,omitempty"`
	ScreenshotURL    string   `json:"screenshotUrl,omitempty"`
	Slug             string   `json:"slug,omitempty"`
	Status           Status   `json:"status"`
	AdminComment     string   `json:"adminComment,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

func newSubmissionResponse(submission Submission) submissionResponse {
	return submissionResponse{
		ID: submission.ID, PublicID: submission.PublicID, Title: submission.Title, Description: submission.Description,
		EventDate: submission.EventDate.Format(time.DateOnly), EventType: submission.EventType,
		Email: submission.Email, TelegramUsername: submission.TelegramUsername,
		Latitude: submission.Latitude, Longitude: submission.Longitude,
		SiteURL: submission.SiteURL, ScreenshotURL: submission.ScreenshotURL, Slug: submission.Slug,
		Status: submission.Status, AdminComment: submission.AdminComment,
		CreatedAt: submission.CreatedAt.Format(time.RFC3339), UpdatedAt: submission.UpdatedAt.Format(time.RFC3339),
	}
}

type auditResponse struct {
	ID                 int64  `json:"id"`
	SubmissionID       int64  `json:"submissionId"`
	SubmissionPublicID string `json:"submissionPublicId"`
	SubmissionTitle    string `json:"submissionTitle"`
	AdminLogin         string `json:"adminLogin"`
	AdminRole          string `json:"adminRole"`
	Action             string `json:"action"`
	Comment            string `json:"comment,omitempty"`
	CreatedAt          string `json:"createdAt"`
}

func newAuditResponse(entry AuditEntry) auditResponse {
	return auditResponse{
		ID: entry.ID, SubmissionID: entry.SubmissionID, SubmissionPublicID: entry.SubmissionPublicID, SubmissionTitle: entry.SubmissionTitle,
		AdminLogin: entry.AdminLogin, AdminRole: entry.AdminRole, Action: entry.Action,
		Comment: entry.Comment, CreatedAt: entry.CreatedAt.Format(time.RFC3339),
	}
}

func submissionID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

// paginationParams rejects non-positive numbers; normalizePagination owns defaults and the pageSize cap.
func paginationParams(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	fields := make(map[string]string)
	page, pageOK := optionalSubmissionInt(r.URL.Query().Get("page"))
	if !pageOK {
		fields["page"] = "Ожидается положительное число"
	}
	pageSize, pageSizeOK := optionalSubmissionInt(r.URL.Query().Get("pageSize"))
	if !pageSizeOK {
		fields["pageSize"] = "Ожидается положительное число"
	}
	if len(fields) > 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return 0, 0, false
	}
	return page, pageSize, true
}

func optionalSubmissionInt(value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}
