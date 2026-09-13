package submissions

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReviewTransitionsNewSubmissionWithAudit(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 7, Status: StatusReview}}
	service := NewService(store, nil)

	result, err := service.Review(t.Context(), 7, Actor{Login: "editor", Role: "admin"})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Status != StatusReview {
		t.Fatalf("Review() status = %q", result.Status)
	}
	if store.transition.ID != 7 || store.transition.From != StatusNew || store.transition.To != StatusReview {
		t.Fatalf("transition = %#v", store.transition)
	}
	if store.transition.Audit.Action != "reviewed" || store.transition.Audit.AdminLogin != "editor" || store.transition.Audit.AdminRole != "admin" {
		t.Fatalf("audit = %#v", store.transition.Audit)
	}
}

func TestRejectRequiresComment(t *testing.T) {
	store := &submissionStoreStub{}
	service := NewService(store, nil)

	_, err := service.Reject(t.Context(), 1, Actor{Login: "editor", Role: "admin"}, "  ")
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["comment"] == "" {
		t.Fatalf("Reject() error = %v, want comment validation", err)
	}
	if store.transitionCalled {
		t.Fatal("store.Transition was called without comment")
	}
}

func TestRejectTransitionsReviewSubmission(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 8, Status: StatusRejected}}
	service := NewService(store, nil)

	_, err := service.Reject(t.Context(), 8, Actor{Login: "editor", Role: "admin"}, " duplicate ")
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if store.transition.From != StatusReview || store.transition.To != StatusRejected || store.transition.Comment != "duplicate" || store.transition.Audit.Comment != "duplicate" {
		t.Fatalf("transition = %#v", store.transition)
	}
}

func TestMarkReadyTransitionsProcessingSubmission(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 4, Status: StatusNew}}
	service := NewService(store, nil)

	_, err := service.MarkReady(t.Context(), 4, "https://example.test/preview/event", "https://example.test/screenshot.png")
	if err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}
	if store.transition.From != StatusProcessing || store.transition.To != StatusNew || store.transition.SiteURL == "" || store.transition.ScreenshotURL == "" {
		t.Fatalf("transition = %#v", store.transition)
	}
}

func TestRejectProcessingStoresReasonWithoutAdminAudit(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 5, Status: StatusRejected}}
	service := NewService(store, nil)

	_, err := service.RejectProcessing(t.Context(), 5, " archive rejected ")
	if err != nil {
		t.Fatalf("RejectProcessing() error = %v", err)
	}
	if store.transition.From != StatusProcessing || store.transition.To != StatusRejected || store.transition.Comment != "archive rejected" {
		t.Fatalf("transition = %#v", store.transition)
	}
	if store.transition.Audit.Action != "" {
		t.Fatalf("worker transition audit = %#v", store.transition.Audit)
	}
}

func TestPublishValidatesCoordinatesBeforeTransition(t *testing.T) {
	store := &submissionStoreStub{}
	service := NewService(store, &eventPublisherStub{})

	_, err := service.Publish(t.Context(), 1, Actor{Login: "editor", Role: "admin"}, 91, 60)
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["latitude"] == "" {
		t.Fatalf("Publish() error = %v, want latitude validation", err)
	}
	if store.transitionCalled {
		t.Fatal("store.Transition was called with invalid coordinates")
	}
}

func TestPublishSendsSnapshotAfterTransition(t *testing.T) {
	date := mustSubmissionDate(t, "2020-01-02")
	store := &submissionStoreStub{
		getSubmission: Submission{
			ID: 9, Title: "Event", Description: "Description", EventDate: date,
			EventType: "military", SiteURL: "https://example.test/preview/event/index.html",
			ScreenshotURL: "https://example.test/sites/event/screenshot.png", Status: StatusReview,
		},
		submission: Submission{
			ID: 9, Title: "Event", Description: "Description", EventDate: date,
			EventType: "military", SiteURL: "https://example.test/sites/event/index.html",
			ScreenshotURL: "https://example.test/sites/event/screenshot.png", Status: StatusPublished,
		},
	}
	publisher := &eventPublisherStub{}
	service := NewService(store, publisher)

	result, err := service.Publish(t.Context(), 9, Actor{Login: "editor", Role: "admin"}, 55.5, 60.5)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.Status != StatusPublished || store.transition.From != StatusReview || store.transition.To != StatusPublished || store.transition.Audit.Action != "published" {
		t.Fatalf("transition = %#v, result = %#v", store.transition, result)
	}
	if publisher.snapshot.SubmissionID != 9 || publisher.snapshot.SiteURL != "https://example.test/sites/event/index.html" || publisher.snapshot.Latitude != 55.5 || publisher.snapshot.EventType != "military" {
		t.Fatalf("snapshot = %#v", publisher.snapshot)
	}
}

func TestPublishRetriesEventForAlreadyPublishedSubmission(t *testing.T) {
	date := mustSubmissionDate(t, "2020-01-02")
	store := &submissionStoreStub{
		transitionError: ErrInvalidTransition,
		getSubmission:   Submission{ID: 10, Title: "Event", EventDate: date, EventType: "military", SiteURL: "https://example.test/sites/event", Status: StatusPublished},
	}
	publisher := &eventPublisherStub{}
	service := NewService(store, publisher)

	_, err := service.Publish(t.Context(), 10, Actor{Login: "editor", Role: "admin"}, 55, 60)
	if err != nil {
		t.Fatalf("Publish() retry error = %v", err)
	}
	if publisher.snapshot.SubmissionID != 10 || store.getID != 10 {
		t.Fatalf("retry snapshot = %#v, get ID = %d", publisher.snapshot, store.getID)
	}
}

type submissionStoreStub struct {
	transition       Transition
	transitionCalled bool
	submission       Submission
	transitionError  error
	getID            int64
	getSubmission    Submission
	getError         error
}

func (stub *submissionStoreStub) Transition(_ context.Context, transition Transition) (Submission, error) {
	stub.transitionCalled = true
	stub.transition = transition
	return stub.submission, stub.transitionError
}

func (stub *submissionStoreStub) Get(_ context.Context, id int64) (Submission, error) {
	stub.getID = id
	return stub.getSubmission, stub.getError
}

type eventPublisherStub struct {
	snapshot EventSnapshot
	err      error
}

func (stub *eventPublisherStub) Publish(_ context.Context, snapshot EventSnapshot) error {
	stub.snapshot = snapshot
	return stub.err
}

func mustSubmissionDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return date
}
