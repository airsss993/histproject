package submissions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

var (
	ErrSubmissionNotFound = errors.New("submission not found")
	ErrInvalidTransition  = errors.New("invalid submission transition")
)

type ValidationError struct {
	Fields map[string]string
}

func (validationError *ValidationError) Error() string {
	return "submission validation failed"
}

type Store interface {
	Get(context.Context, int64) (Submission, error)
	Transition(context.Context, Transition) (Submission, error)
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
	store     Store
	publisher EventPublisher
}

func NewService(store Store, publisher EventPublisher) *Service {
	return &Service{store: store, publisher: publisher}
}

func (service *Service) Review(ctx context.Context, id int64, actor Actor) (Submission, error) {
	if id <= 0 {
		return Submission{}, invalidSubmissionID()
	}
	return service.store.Transition(ctx, Transition{
		ID: id, From: StatusNew, To: StatusReview,
		Audit: Audit{AdminLogin: actor.Login, AdminRole: actor.Role, Action: "reviewed"},
	})
}

func (service *Service) Reject(ctx context.Context, id int64, actor Actor, comment string) (Submission, error) {
	comment = strings.TrimSpace(comment)
	fields := make(map[string]string)
	if id <= 0 {
		fields["id"] = "Ожидается положительный ID"
	}
	if comment == "" {
		fields["comment"] = "Обязательное поле"
	}
	if len(fields) > 0 {
		return Submission{}, &ValidationError{Fields: fields}
	}
	return service.store.Transition(ctx, Transition{
		ID: id, From: StatusReview, To: StatusRejected, Comment: comment,
		Audit: Audit{AdminLogin: actor.Login, AdminRole: actor.Role, Action: "rejected", Comment: comment},
	})
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
	return service.store.Transition(ctx, Transition{
		ID: id, From: StatusProcessing, To: StatusNew,
		SiteURL: strings.TrimSpace(siteURL), ScreenshotURL: strings.TrimSpace(screenshotURL),
	})
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
	return service.store.Transition(ctx, Transition{
		ID: id, From: StatusProcessing, To: StatusRejected, Comment: reason,
	})
}

func (service *Service) Publish(ctx context.Context, id int64, actor Actor, latitude, longitude float64) (Submission, error) {
	if fields := validatePublication(id, latitude, longitude); len(fields) > 0 {
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
		publicSiteURL := strings.Replace(submission.SiteURL, "/preview/", "/sites/", 1)
		submission, err = service.store.Transition(ctx, Transition{
			ID: id, From: StatusReview, To: StatusPublished, SiteURL: publicSiteURL,
			Audit: Audit{AdminLogin: actor.Login, AdminRole: actor.Role, Action: "published"},
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
		return Submission{}, errors.New("event publisher is nil")
	}
	if err := service.publisher.Publish(ctx, EventSnapshot{
		SubmissionID: submission.ID, Title: submission.Title, Description: submission.Description,
		Latitude: latitude, Longitude: longitude, EventDate: submission.EventDate,
		EventType: submission.EventType, SiteURL: submission.SiteURL, PreviewImageURL: submission.ScreenshotURL,
	}); err != nil {
		return Submission{}, fmt.Errorf("publish submission event: %w", err)
	}
	return submission, nil
}

func validatePublication(id int64, latitude, longitude float64) map[string]string {
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
	return fields
}

func invalidSubmissionID() *ValidationError {
	return &ValidationError{Fields: map[string]string{"id": "Ожидается положительный ID"}}
}
