package submissions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/mail"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gosimple/slug"
)

var (
	ErrSubmissionNotFound = errors.New("submission not found")
	ErrInvalidTransition  = errors.New("invalid submission transition")
)

// maxRejectCommentRunes keeps the rejection notice inside one Telegram message (4096 characters).
const maxRejectCommentRunes = 1000

var telegramUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

type ValidationError struct {
	Fields map[string]string
}

func (validationError *ValidationError) Error() string {
	return "submission validation failed"
}

type Store interface {
	Create(context.Context, CreateRecord) (Submission, error)
	Get(context.Context, int64) (Submission, error)
	List(context.Context, ListFilter) (SubmissionPage, error)
	Transition(context.Context, Transition) (Submission, error)
	ByAuthor(context.Context, string) ([]Submission, error)
	Delete(context.Context, int64) error
	AuditLog(context.Context, AuditFilter) (AuditPage, error)
}

type EventPublisher interface {
	Publish(context.Context, EventSnapshot) error
}

type ArchiveStore interface {
	PutArchive(context.Context, io.Reader, int64) (string, error)
	DeleteArchive(context.Context, string) error
}

type JobQueue interface {
	EnqueueArchive(context.Context, int64, string) error
}

type Notifier interface {
	SubmissionChanged(context.Context, Submission, string) error
}

type Service struct {
	store             Store
	publisher         EventPublisher
	archives          ArchiveStore
	jobs              JobQueue
	notifier          Notifier
	publicSiteBaseURL string
}

func NewService(store Store, publisher EventPublisher, archives ArchiveStore, jobs JobQueue, notifier Notifier, publicSiteBaseURL string) *Service {
	return &Service{
		store: store, publisher: publisher, archives: archives, jobs: jobs,
		notifier: notifier, publicSiteBaseURL: publicSiteBaseURL,
	}
}

// Create expects input already checked by validateCreateInput in the HTTP handler.
func (service *Service) Create(ctx context.Context, input CreateInput) (Submission, error) {
	if service.archives == nil {
		return Submission{}, errors.New("archive store is nil")
	}
	if service.jobs == nil {
		return Submission{}, errors.New("job queue is nil")
	}

	archiveID, err := service.archives.PutArchive(ctx, input.Archive, input.ArchiveSize)
	if err != nil {
		return Submission{}, fmt.Errorf("store submission archive: %w", err)
	}
	title := strings.TrimSpace(input.Title)
	slugValue := slug.Make(title)
	if slugValue == "" {
		slugValue = "object"
	}
	submission, err := service.store.Create(ctx, CreateRecord{
		Title: title, Description: strings.TrimSpace(input.Description), EventDate: input.EventDate,
		EventType: strings.TrimSpace(input.EventType), Email: strings.TrimSpace(input.Email),
		TelegramUsername: strings.TrimPrefix(strings.TrimSpace(input.TelegramUsername), "@"),
		Latitude:         input.Latitude, Longitude: input.Longitude,
		ArchiveID: archiveID, Slug: slugValue + "-" + archiveID,
	})
	if err != nil {
		deleteErr := service.archives.DeleteArchive(ctx, archiveID)
		if deleteErr != nil {
			return Submission{}, errors.Join(err, fmt.Errorf("compensate archive upload: %w", deleteErr))
		}
		return Submission{}, err
	}
	if err := service.jobs.EnqueueArchive(ctx, submission.ID, archiveID); err != nil {
		err = fmt.Errorf("enqueue submission archive: %w", err)
		if deleteErr := service.store.Delete(ctx, submission.ID); deleteErr != nil {
			return Submission{}, errors.Join(err, fmt.Errorf("compensate submission insert: %w", deleteErr))
		}
		if deleteErr := service.archives.DeleteArchive(ctx, archiveID); deleteErr != nil {
			return Submission{}, errors.Join(err, fmt.Errorf("compensate archive upload: %w", deleteErr))
		}
		return Submission{}, err
	}
	service.notify(ctx, submission, "created")
	return submission, nil
}

func (service *Service) List(ctx context.Context, filter ListFilter) (SubmissionPage, error) {
	fields := make(map[string]string)
	if filter.Status != "" && !validStatus(filter.Status) {
		fields["status"] = "Неизвестный статус"
	}
	filter.Page, filter.PageSize = normalizePagination(filter.Page, filter.PageSize, fields)
	if len(fields) > 0 {
		return SubmissionPage{}, &ValidationError{Fields: fields}
	}
	filter.Search = strings.TrimSpace(filter.Search)
	return service.store.List(ctx, filter)
}

// ByAuthor - заявки автора для профиля. Почту приводим к нижнему регистру: в учётке она такая.
func (service *Service) ByAuthor(ctx context.Context, email string) ([]Submission, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil
	}
	return service.store.ByAuthor(ctx, email)
}

func (service *Service) Get(ctx context.Context, id int64) (Submission, error) {
	if id <= 0 {
		return Submission{}, invalidSubmissionID()
	}
	return service.store.Get(ctx, id)
}

func (service *Service) AuditLog(ctx context.Context, filter AuditFilter) (AuditPage, error) {
	fields := make(map[string]string)
	filter.Page, filter.PageSize = normalizePagination(filter.Page, filter.PageSize, fields)
	if len(fields) > 0 {
		return AuditPage{}, &ValidationError{Fields: fields}
	}
	filter.Search = strings.TrimSpace(filter.Search)
	return service.store.AuditLog(ctx, filter)
}

func (service *Service) Review(ctx context.Context, id int64, actor Actor) (Submission, error) {
	if id <= 0 {
		return Submission{}, invalidSubmissionID()
	}
	submission, err := service.store.Transition(ctx, Transition{
		ID: id, From: StatusNew, To: StatusReview,
		Audit: Audit{AdminLogin: actor.Login, AdminRole: actor.Role, Action: "reviewed"},
	})
	if err != nil {
		return Submission{}, err
	}
	service.notify(ctx, submission, "reviewed")
	return submission, nil
}

func (service *Service) Reject(ctx context.Context, id int64, actor Actor, comment string) (Submission, error) {
	comment = strings.TrimSpace(comment)
	fields := make(map[string]string)
	if id <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if comment == "" {
		fields["comment"] = "Обязательное поле"
	} else if utf8.RuneCountInString(comment) > maxRejectCommentRunes {
		fields["comment"] = fmt.Sprintf("Не более %d символов", maxRejectCommentRunes)
	}
	if len(fields) > 0 {
		return Submission{}, &ValidationError{Fields: fields}
	}
	submission, err := service.store.Transition(ctx, Transition{
		ID: id, From: StatusReview, To: StatusRejected, Comment: comment,
		Audit: Audit{AdminLogin: actor.Login, AdminRole: actor.Role, Action: "rejected", Comment: comment},
	})
	if err != nil {
		return Submission{}, err
	}
	service.notify(ctx, submission, "rejected")
	return submission, nil
}

func (service *Service) MarkReady(ctx context.Context, id int64, siteURL, screenshotURL string) (Submission, error) {
	fields := make(map[string]string)
	if id <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if strings.TrimSpace(siteURL) == "" {
		fields["siteUrl"] = "Обязательное поле"
	}
	if strings.TrimSpace(screenshotURL) == "" {
		fields["screenshotUrl"] = "Обязательное поле"
	}
	if len(fields) > 0 {
		return Submission{}, &ValidationError{Fields: fields}
	}
	submission, err := service.store.Transition(ctx, Transition{
		ID: id, From: StatusProcessing, To: StatusNew,
		SiteURL: strings.TrimSpace(siteURL), ScreenshotURL: strings.TrimSpace(screenshotURL),
	})
	if err != nil {
		return Submission{}, err
	}
	service.notify(ctx, submission, "ready")
	return submission, nil
}

func (service *Service) RejectProcessing(ctx context.Context, id int64, reason string) (Submission, error) {
	reason = strings.TrimSpace(reason)
	fields := make(map[string]string)
	if id <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if reason == "" {
		fields["reason"] = "Обязательное поле"
	}
	if len(fields) > 0 {
		return Submission{}, &ValidationError{Fields: fields}
	}
	submission, err := service.store.Transition(ctx, Transition{
		ID: id, From: StatusProcessing, To: StatusRejected, Comment: reason,
	})
	if err != nil {
		return Submission{}, err
	}
	service.notify(ctx, submission, "rejected")
	return submission, nil
}

// Publish takes the coordinates and event type confirmed by the moderator; an empty eventType keeps the author's one.
func (service *Service) Publish(ctx context.Context, id int64, actor Actor, latitude, longitude float64, eventType string) (Submission, error) {
	eventType = strings.TrimSpace(eventType)
	if fields := validatePublication(id, latitude, longitude, eventType); len(fields) > 0 {
		return Submission{}, &ValidationError{Fields: fields}
	}

	submission, err := service.store.Get(ctx, id)
	if err != nil {
		return Submission{}, err
	}
	if submission.Status != StatusPublished {
		if submission.Status != StatusReview {
			return Submission{}, ErrInvalidTransition
		}
		publicSiteURL := strings.TrimRight(service.publicSiteBaseURL, "/") + "/" + url.PathEscape(submission.Slug) + "/index.html"
		if eventType == "" {
			eventType = submission.EventType
		}
		submission, err = service.store.Transition(ctx, Transition{
			ID: id, From: StatusReview, To: StatusPublished, SiteURL: publicSiteURL, EventType: eventType,
			Audit: Audit{
				AdminLogin: actor.Login, AdminRole: actor.Role, Action: "published",
				Comment: fmt.Sprintf("Точка %.6f, %.6f; тип: %s", latitude, longitude, eventTypeLabel[eventType]),
			},
		})
		if errors.Is(err, ErrInvalidTransition) {
			submission, err = service.store.Get(ctx, id)
		}
		if err != nil {
			return Submission{}, err
		}
		if submission.Status != StatusPublished {
			return Submission{}, ErrInvalidTransition
		}
	}
	if service.publisher == nil {
		return submission, errors.New("event publisher is nil")
	}
	if err := service.publisher.Publish(ctx, EventSnapshot{
		SubmissionID: submission.ID, Title: submission.Title, Description: submission.Description,
		Latitude: latitude, Longitude: longitude, EventDate: submission.EventDate,
		EventType: submission.EventType, SiteURL: submission.SiteURL, PreviewImageURL: submission.ScreenshotURL,
	}); err != nil {
		return submission, fmt.Errorf("publish submission event: %w", err)
	}
	service.notify(ctx, submission, "published")
	return submission, nil
}

// notify runs after the status change is committed, so a failed enqueue is logged instead of failing the request.
// ponytail: a lost notification stays lost, move to an outbox table in the Transition transaction if that matters.
func (service *Service) notify(ctx context.Context, submission Submission, action string) {
	if service.notifier == nil {
		return
	}
	if err := service.notifier.SubmissionChanged(ctx, submission, action); err != nil {
		slog.ErrorContext(ctx, "enqueue submission notification failed",
			"submission_id", submission.ID, "action", action, "error", err)
	}
}

func validatePublication(id int64, latitude, longitude float64, eventType string) map[string]string {
	fields := make(map[string]string)
	if id <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if eventType != "" && eventType != "political" && eventType != "military" {
		fields["eventType"] = "Ожидается political или military"
	}
	validateCoordinates(fields, latitude, longitude)
	return fields
}

func validateCoordinates(fields map[string]string, latitude, longitude float64) {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		fields["latitude"] = "От -90 до 90"
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		fields["longitude"] = "От -180 до 180"
	}
}

func invalidSubmissionID() *ValidationError {
	return &ValidationError{Fields: map[string]string{"id": "Ожидается положительный ID"}}
}

func validateCreateInput(input CreateInput, maxUploadBytes int64) map[string]string {
	fields := make(map[string]string)
	title := strings.TrimSpace(input.Title)
	description := strings.TrimSpace(input.Description)
	email := strings.TrimSpace(input.Email)
	telegramUsername := strings.TrimPrefix(strings.TrimSpace(input.TelegramUsername), "@")
	if title == "" {
		fields["title"] = "Обязательное поле"
	} else if utf8.RuneCountInString(title) > 100 {
		fields["title"] = "Не более 100 символов"
	}
	if description == "" {
		fields["description"] = "Обязательное поле"
	} else if utf8.RuneCountInString(description) > 450 {
		fields["description"] = "Не более 450 символов"
	}
	if input.EventDate.IsZero() {
		fields["eventDate"] = "Ожидается дата YYYY-MM-DD"
	}
	if input.EventType != "political" && input.EventType != "military" {
		fields["eventType"] = "Ожидается political или military"
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		fields["email"] = "Некорректный email"
	} else if utf8.RuneCountInString(email) > 254 {
		fields["email"] = "Не более 254 символов"
	}
	if !telegramUsernamePattern.MatchString(telegramUsername) {
		fields["telegramUsername"] = "От 5 до 32 латинских букв, цифр или _"
	}
	validateCoordinates(fields, input.Latitude, input.Longitude)
	if input.Archive == nil {
		fields["archive"] = "Обязательное поле"
	} else if strings.ToLower(filepath.Ext(strings.TrimSpace(input.ArchiveName))) != ".zip" {
		fields["archive"] = "Ожидается ZIP-архив"
	} else if input.ArchiveSize <= 0 || maxUploadBytes <= 0 || input.ArchiveSize > maxUploadBytes {
		fields["archive"] = fmt.Sprintf("Не более %d МБ", maxUploadBytes>>20)
	}
	return fields
}

func normalizePagination(page, pageSize int, fields map[string]string) (int, int) {
	if page == 0 {
		page = 1
	} else if page < 1 {
		fields["page"] = "Ожидается положительное число"
	}
	if pageSize == 0 {
		pageSize = 20
	} else if pageSize < 1 || pageSize > 100 {
		fields["pageSize"] = "От 1 до 100"
	}
	return page, pageSize
}

func validStatus(status Status) bool {
	switch status {
	case StatusProcessing, StatusNew, StatusReview, StatusRejected, StatusPublished:
		return true
	default:
		return false
	}
}

// eventTypeLabel - как тип называется в истории модерации.
var eventTypeLabel = map[string]string{"political": "политическое", "military": "военное"}
