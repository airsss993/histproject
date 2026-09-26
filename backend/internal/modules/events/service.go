package events

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

var (
	ErrEventNotFound = errors.New("event not found")
	ErrViewNotFound  = errors.New("view not found")
	ErrUnauthorized  = errors.New("unauthorized")
)

// MaxViewSeconds - потолок активного времени одного просмотра, как CHECK в events.views.
const MaxViewSeconds = 2 * 60 * 60

var visitorIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)

type ValidationError struct {
	Fields map[string]string
}

func (validationError *ValidationError) Error() string {
	return "event validation failed"
}

type Store interface {
	Publish(context.Context, Snapshot) (Event, error)
	List(context.Context, Filter) (Page, error)
	Get(ctx context.Context, id, viewerID int64) (Event, error)
	UpdateCoordinates(context.Context, int64, float64, float64) error
	Delete(context.Context, int64) error
	SetStar(ctx context.Context, eventID, userID int64, on bool) (StarState, error)
	StartView(ctx context.Context, eventID, userID int64, visitorID string) (int64, error)
	TouchView(ctx context.Context, eventID, viewID int64, visitorID string, seconds int) error
	Summaries(ctx context.Context, submissionIDs []int64, viewerID int64) ([]Summary, error)
}

// Authors отдаёт автора заявки: почту из submissions и учётку из users. Адаптер - в internal/app.
type Authors interface {
	Author(ctx context.Context, submissionID int64) (Author, error)
}

type Service struct {
	store   Store
	authors Authors
}

func NewService(store Store, authors Authors) *Service {
	return &Service{store: store, authors: authors}
}

func (service *Service) Publish(ctx context.Context, snapshot Snapshot) (Event, error) {
	fields := make(map[string]string)
	if snapshot.SubmissionID <= 0 {
		fields["submissionId"] = "Ожидается положительный ID"
	}
	if strings.TrimSpace(snapshot.Title) == "" {
		fields["title"] = "Обязательное поле"
	}
	if snapshot.EventDate.IsZero() {
		fields["eventDate"] = "Обязательное поле"
	}
	if !eventTypePattern.MatchString(snapshot.EventType) {
		fields["eventType"] = "Латинские буквы, цифры и _"
	}
	if math.IsNaN(snapshot.Latitude) || math.IsInf(snapshot.Latitude, 0) || snapshot.Latitude < -90 || snapshot.Latitude > 90 {
		fields["latitude"] = "От -90 до 90"
	}
	if math.IsNaN(snapshot.Longitude) || math.IsInf(snapshot.Longitude, 0) || snapshot.Longitude < -180 || snapshot.Longitude > 180 {
		fields["longitude"] = "От -180 до 180"
	}
	if strings.TrimSpace(snapshot.SiteURL) == "" {
		fields["siteUrl"] = "Обязательное поле"
	}
	if len(fields) > 0 {
		return Event{}, &ValidationError{Fields: fields}
	}
	return service.store.Publish(ctx, snapshot)
}

func (service *Service) List(ctx context.Context, input ListInput, viewer Viewer) (Page, error) {
	filter, fields := normalizeFilter(input)
	if len(fields) > 0 {
		return Page{}, &ValidationError{Fields: fields}
	}
	filter.ViewerID = viewer.UserID
	return service.store.List(ctx, filter)
}

func (service *Service) Get(ctx context.Context, id int64, viewer Viewer) (Event, error) {
	if id <= 0 {
		return Event{}, invalidID()
	}
	event, err := service.store.Get(ctx, id, viewer.UserID)
	if err != nil || service.authors == nil {
		return event, err
	}
	event.Author, err = service.authors.Author(ctx, event.SubmissionID)
	return event, err
}

// Star ставит или снимает звезду. Гостю - ErrUnauthorized: звезда привязана к аккаунту.
func (service *Service) Star(ctx context.Context, id int64, viewer Viewer, on bool) (StarState, error) {
	if id <= 0 {
		return StarState{}, invalidID()
	}
	if viewer.UserID == 0 {
		return StarState{}, ErrUnauthorized
	}
	return service.store.SetStar(ctx, id, viewer.UserID, on)
}

// StartView записывает открытие страницы просмотра. Автор своего сайта в статистику не попадает:
// recorded false, записи нет. Автор узнаётся по почте заявки, как в профиле.
func (service *Service) StartView(ctx context.Context, id int64, viewer Viewer, visitorID string) (viewID int64, recorded bool, err error) {
	if id <= 0 {
		return 0, false, invalidID()
	}
	if !visitorIDPattern.MatchString(visitorID) {
		return 0, false, &ValidationError{Fields: map[string]string{"visitorId": "Ожидается UUID"}}
	}
	if viewer.Email != "" && service.authors != nil {
		event, err := service.store.Get(ctx, id, 0)
		if err != nil {
			return 0, false, err
		}
		author, err := service.authors.Author(ctx, event.SubmissionID)
		if err != nil {
			return 0, false, err
		}
		if strings.EqualFold(author.Email, viewer.Email) {
			return 0, false, nil
		}
	}
	viewID, err = service.store.StartView(ctx, id, viewer.UserID, visitorID)
	return viewID, err == nil, err
}

// TouchView дописывает активное время просмотра, seconds - сколько всего с начала просмотра.
func (service *Service) TouchView(ctx context.Context, id, viewID int64, visitorID string, seconds int) error {
	fields := make(map[string]string)
	if id <= 0 || viewID <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if !visitorIDPattern.MatchString(visitorID) {
		fields["visitorId"] = "Ожидается UUID"
	}
	if seconds < 0 {
		fields["activeSeconds"] = "Не меньше 0"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return service.store.TouchView(ctx, id, viewID, visitorID, min(seconds, MaxViewSeconds))
}

// Summaries - звёзды и просмотры по заявкам, ключ - id заявки. Заявки без события в ответ не попадают.
func (service *Service) Summaries(ctx context.Context, submissionIDs []int64, viewerID int64) (map[int64]Summary, error) {
	result := make(map[int64]Summary, len(submissionIDs))
	if len(submissionIDs) == 0 {
		return result, nil
	}
	items, err := service.store.Summaries(ctx, submissionIDs, viewerID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		result[item.SubmissionID] = item
	}
	return result, nil
}

func (service *Service) UpdateCoordinates(ctx context.Context, id int64, latitude, longitude float64) error {
	fields := make(map[string]string)
	if id <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		fields["latitude"] = "От -90 до 90"
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		fields["longitude"] = "От -180 до 180"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return service.store.UpdateCoordinates(ctx, id, latitude, longitude)
}

func (service *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidID()
	}
	return service.store.Delete(ctx, id)
}

func normalizeFilter(input ListInput) (Filter, map[string]string) {
	fields := make(map[string]string)
	filter := Filter{Page: input.Page, PageSize: input.PageSize}
	if filter.Page == 0 {
		filter.Page = 1
	} else if filter.Page < 0 {
		fields["page"] = "Ожидается положительное число"
	}
	if filter.PageSize == 0 {
		filter.PageSize = 50
	} else if filter.PageSize < 0 || filter.PageSize > 100 {
		fields["pageSize"] = "От 1 до 100"
	}

	for _, value := range input.EventTypes {
		eventType := strings.TrimSpace(value)
		if !eventTypePattern.MatchString(eventType) {
			fields["eventType"] = "Латинские буквы, цифры и _"
			continue
		}
		filter.EventTypes = append(filter.EventTypes, eventType)
	}
	filter.DateFrom = parseDate(input.DateFrom, "dateFrom", fields)
	filter.DateTo = parseDate(input.DateTo, "dateTo", fields)
	if filter.DateFrom != nil && filter.DateTo != nil && filter.DateFrom.After(*filter.DateTo) {
		fields["dateTo"] = "Не раньше даты начала"
	}
	return filter, fields
}

func parseDate(value, field string, fields map[string]string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		fields[field] = "Используйте YYYY-MM-DD"
		return nil
	}
	return &parsed
}

func invalidID() *ValidationError {
	return &ValidationError{Fields: map[string]string{"id": "Ожидается положительный ID"}}
}
