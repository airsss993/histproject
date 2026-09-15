package events

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

var ErrEventNotFound = errors.New("event not found")

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
	Get(context.Context, int64) (Event, error)
	UpdateCoordinates(context.Context, int64, float64, float64) error
	Delete(context.Context, int64) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
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

func (service *Service) List(ctx context.Context, input ListInput) (Page, error) {
	filter, fields := normalizeFilter(input)
	if len(fields) > 0 {
		return Page{}, &ValidationError{Fields: fields}
	}
	return service.store.List(ctx, filter)
}

func (service *Service) Get(ctx context.Context, id int64) (Event, error) {
	if id <= 0 {
		return Event{}, invalidID()
	}
	return service.store.Get(ctx, id)
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
