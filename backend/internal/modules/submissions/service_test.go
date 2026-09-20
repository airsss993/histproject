package submissions

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReviewTransitionsNewSubmissionWithAudit(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 7, Status: StatusReview}}
	notifier := &notifierStub{}
	service := NewService(store, nil, nil, nil, notifier, "https://example.test/sites")

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
	if notifier.calls != 1 || notifier.action != "reviewed" {
		t.Fatalf("notifier = calls:%d action:%q", notifier.calls, notifier.action)
	}
}

func TestRejectRequiresComment(t *testing.T) {
	store := &submissionStoreStub{}
	service := NewService(store, nil, nil, nil, nil, "https://example.test/sites")

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
	notifier := &notifierStub{}
	service := NewService(store, nil, nil, nil, notifier, "https://example.test/sites")

	_, err := service.Reject(t.Context(), 8, Actor{Login: "editor", Role: "admin"}, " duplicate ")
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if store.transition.From != StatusReview || store.transition.To != StatusRejected || store.transition.Comment != "duplicate" || store.transition.Audit.Comment != "duplicate" {
		t.Fatalf("transition = %#v", store.transition)
	}
	if notifier.calls != 1 || notifier.action != "rejected" {
		t.Fatalf("notifier = calls:%d action:%q", notifier.calls, notifier.action)
	}
}

func TestMarkReadyTransitionsProcessingSubmission(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 4, Status: StatusNew}}
	service := NewService(store, nil, nil, nil, nil, "https://example.test/sites")

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
	notifier := &notifierStub{}
	service := NewService(store, nil, nil, nil, notifier, "https://example.test/sites")

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
	if notifier.calls != 1 || notifier.action != "rejected" {
		t.Fatalf("notifier = calls:%d action:%q", notifier.calls, notifier.action)
	}
}

func TestCreateEnqueuesNotificationAfterArchiveTask(t *testing.T) {
	store := &submissionStoreStub{createdSubmission: Submission{ID: 11, Status: StatusProcessing}}
	archives := &archiveStoreStub{archiveID: "archive-id"}
	jobs := &jobQueueStub{}
	notifier := &notifierStub{}
	service := NewService(store, nil, archives, jobs, notifier, "https://example.test/sites")

	result, err := service.Create(t.Context(), CreateInput{
		Title: "Event", Description: "Description", EventDate: mustSubmissionDate(t, "2020-01-02"),
		EventType: "military", Email: "author@example.test", TelegramUsername: "author",
		ArchiveName: "site.zip", ArchiveSize: 3, Archive: strings.NewReader("zip"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ID != 11 || !jobs.called {
		t.Fatalf("result = %#v, archive task called = %v", result, jobs.called)
	}
	if notifier.calls != 1 || notifier.action != "created" || notifier.submission.ID != 11 {
		t.Fatalf("notifier = calls:%d action:%q submission:%#v", notifier.calls, notifier.action, notifier.submission)
	}
}

func TestPublishValidatesCoordinatesBeforeTransition(t *testing.T) {
	store := &submissionStoreStub{}
	service := NewService(store, &eventPublisherStub{}, nil, nil, nil, "https://example.test/sites")

	_, err := service.Publish(t.Context(), 1, Actor{Login: "editor", Role: "admin"}, 91, 60, "")
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
			EventType: "military", SiteURL: "https://example.test/preview/event/index.html", Slug: "event",
			ScreenshotURL: "https://example.test/sites/event/screenshot.png", Status: StatusReview,
		},
		submission: Submission{
			ID: 9, Title: "Event", Description: "Description", EventDate: date,
			EventType: "military", SiteURL: "https://example.test/sites/event/index.html",
			ScreenshotURL: "https://example.test/sites/event/screenshot.png", Status: StatusPublished,
		},
	}
	publisher := &eventPublisherStub{}
	notifier := &notifierStub{}
	service := NewService(store, publisher, nil, nil, notifier, "https://example.test/sites")

	result, err := service.Publish(t.Context(), 9, Actor{Login: "editor", Role: "admin"}, 55.5, 60.5, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.transition.SiteURL != "https://example.test/sites/event/index.html" {
		t.Fatalf("public site URL = %q", store.transition.SiteURL)
	}
	if result.Status != StatusPublished || store.transition.From != StatusReview || store.transition.To != StatusPublished || store.transition.Audit.Action != "published" {
		t.Fatalf("transition = %#v, result = %#v", store.transition, result)
	}
	if publisher.snapshot.SubmissionID != 9 || publisher.snapshot.SiteURL != "https://example.test/sites/event/index.html" || publisher.snapshot.Latitude != 55.5 || publisher.snapshot.EventType != "military" {
		t.Fatalf("snapshot = %#v", publisher.snapshot)
	}
	if notifier.calls != 1 || notifier.action != "published" {
		t.Fatalf("notifier calls = %d, action = %q", notifier.calls, notifier.action)
	}
	if store.transition.EventType != "military" || store.transition.Audit.Comment != "Точка 55.500000, 60.500000; тип: военное" {
		t.Fatalf("transition type = %q, audit comment = %q", store.transition.EventType, store.transition.Audit.Comment)
	}
}

func TestPublishAppliesEventTypeConfirmedByModerator(t *testing.T) {
	date := mustSubmissionDate(t, "2020-01-02")
	store := &submissionStoreStub{
		getSubmission: Submission{ID: 9, EventDate: date, EventType: "military", Slug: "event", Status: StatusReview},
		submission:    Submission{ID: 9, EventDate: date, EventType: "political", Status: StatusPublished},
	}
	service := NewService(store, &eventPublisherStub{}, nil, nil, &notifierStub{}, "https://example.test/sites")

	if _, err := service.Publish(t.Context(), 9, Actor{Login: "editor", Role: "admin"}, 55, 60, "political"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.transition.EventType != "political" || !strings.Contains(store.transition.Audit.Comment, "тип: политическое") {
		t.Fatalf("transition = %#v", store.transition)
	}

	_, err := service.Publish(t.Context(), 9, Actor{Login: "editor", Role: "admin"}, 55, 60, "cultural")
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["eventType"] == "" {
		t.Fatalf("Publish() error = %v, want eventType validation", err)
	}
}

func TestPublishDoesNotNotifyBeforeEventPublicationSucceeds(t *testing.T) {
	date := mustSubmissionDate(t, "2020-01-02")
	store := &submissionStoreStub{
		getSubmission: Submission{ID: 9, Title: "Event", EventDate: date, EventType: "military", Status: StatusReview},
		submission:    Submission{ID: 9, Title: "Event", EventDate: date, EventType: "military", Status: StatusPublished},
	}
	publisher := &eventPublisherStub{err: errors.New("events unavailable")}
	notifier := &notifierStub{}
	service := NewService(store, publisher, nil, nil, notifier, "https://example.test/sites")

	result, err := service.Publish(t.Context(), 9, Actor{Login: "editor", Role: "admin"}, 55, 60, "")
	if err == nil {
		t.Fatal("Publish() error = nil")
	}
	if result.Status != StatusPublished || !store.transitionCalled {
		t.Fatalf("published result = %#v, transition called = %v", result, store.transitionCalled)
	}
	if notifier.calls != 0 {
		t.Fatalf("notifier calls = %d, want 0", notifier.calls)
	}
}

func TestPublishRetriesEventForAlreadyPublishedSubmission(t *testing.T) {
	date := mustSubmissionDate(t, "2020-01-02")
	store := &submissionStoreStub{
		transitionError: ErrInvalidTransition,
		getSubmission:   Submission{ID: 10, Title: "Event", EventDate: date, EventType: "military", SiteURL: "https://example.test/sites/event", Status: StatusPublished},
	}
	publisher := &eventPublisherStub{}
	notifier := &notifierStub{}
	service := NewService(store, publisher, nil, nil, notifier, "https://example.test/sites")

	_, err := service.Publish(t.Context(), 10, Actor{Login: "editor", Role: "admin"}, 55, 60, "")
	if err != nil {
		t.Fatalf("Publish() retry error = %v", err)
	}
	if publisher.snapshot.SubmissionID != 10 || store.getID != 10 {
		t.Fatalf("retry snapshot = %#v, get ID = %d", publisher.snapshot, store.getID)
	}
	if notifier.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1", notifier.calls)
	}
}

func TestCreateDeletesUploadedArchiveWhenDatabaseInsertFails(t *testing.T) {
	want := errors.New("database unavailable")
	store := &submissionStoreStub{createError: want}
	archives := &archiveStoreStub{archiveID: "archive-id"}
	jobs := &jobQueueStub{}
	service := NewService(store, nil, archives, jobs, nil, "https://example.test/sites")
	ctx := t.Context()

	_, err := service.Create(ctx, CreateInput{
		Title: "Event", Description: "Description", EventDate: mustSubmissionDate(t, "2020-01-02"),
		EventType: "military", Email: "author@example.test", TelegramUsername: "author",
		ArchiveName: "site.zip", ArchiveSize: 3, Archive: strings.NewReader("zip"),
	})

	if !errors.Is(err, want) {
		t.Fatalf("Create() error = %v, want database error", err)
	}
	if archives.deleteContext != ctx || archives.deletedID != "archive-id" {
		t.Fatalf("deleted archive = context:%v id:%q", archives.deleteContext == ctx, archives.deletedID)
	}
	if jobs.called {
		t.Fatal("queue called after database failure")
	}
}

func TestCreateRemovesSubmissionWhenArchiveTaskEnqueueFails(t *testing.T) {
	want := errors.New("redis unavailable")
	store := &submissionStoreStub{createdSubmission: Submission{ID: 12, Status: StatusProcessing}}
	archives := &archiveStoreStub{archiveID: "archive-id"}
	notifier := &notifierStub{}
	service := NewService(store, nil, archives, &jobQueueStub{err: want}, notifier, "https://example.test/sites")

	_, err := service.Create(t.Context(), CreateInput{
		Title: "Event", Description: "Description", EventDate: mustSubmissionDate(t, "2020-01-02"),
		EventType: "military", Email: "author@example.test", TelegramUsername: "author",
		ArchiveName: "site.zip", ArchiveSize: 3, Archive: strings.NewReader("zip"),
	})

	if !errors.Is(err, want) || store.deletedID != 12 || archives.deletedID != "archive-id" || notifier.calls != 0 {
		t.Fatalf("error = %v, deleted submission = %d, deleted archive = %q, notifier calls = %d", err, store.deletedID, archives.deletedID, notifier.calls)
	}
}

func TestReviewSucceedsWhenNotificationEnqueueFails(t *testing.T) {
	store := &submissionStoreStub{submission: Submission{ID: 7, Status: StatusReview}}
	service := NewService(store, nil, nil, nil, &notifierStub{err: errors.New("redis unavailable")}, "https://example.test/sites")

	if _, err := service.Review(t.Context(), 7, Actor{Login: "editor", Role: "admin"}); err != nil {
		t.Fatalf("Review() error = %v, want committed transition to succeed", err)
	}
}

type submissionStoreStub struct {
	createRecord      CreateRecord
	createdSubmission Submission
	createError       error
	transition        Transition
	transitionCalled  bool
	submission        Submission
	transitionError   error
	getID             int64
	getSubmission     Submission
	getError          error
	deletedID         int64
	authorEmail       string
	authorSubmissions []Submission
}

func (stub *submissionStoreStub) Create(_ context.Context, record CreateRecord) (Submission, error) {
	stub.createRecord = record
	return stub.createdSubmission, stub.createError
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

func (stub *submissionStoreStub) Delete(_ context.Context, id int64) error {
	stub.deletedID = id
	return nil
}

func (stub *submissionStoreStub) List(context.Context, ListFilter) (SubmissionPage, error) {
	return SubmissionPage{}, nil
}

func (stub *submissionStoreStub) ByAuthor(_ context.Context, email string) ([]Submission, error) {
	stub.authorEmail = email
	return stub.authorSubmissions, nil
}

func (stub *submissionStoreStub) AuditLog(context.Context, AuditFilter) (AuditPage, error) {
	return AuditPage{}, nil
}

type eventPublisherStub struct {
	snapshot EventSnapshot
	err      error
}

type notifierStub struct {
	calls      int
	action     string
	submission Submission
	err        error
}

func (stub *notifierStub) SubmissionChanged(_ context.Context, submission Submission, action string) error {
	stub.calls++
	stub.action = action
	stub.submission = submission
	return stub.err
}

func (stub *eventPublisherStub) Publish(_ context.Context, snapshot EventSnapshot) error {
	stub.snapshot = snapshot
	return stub.err
}

type archiveStoreStub struct {
	archiveID     string
	deleteContext context.Context
	deletedID     string
}

func (stub *archiveStoreStub) PutArchive(context.Context, io.Reader, int64) (string, error) {
	return stub.archiveID, nil
}

func (stub *archiveStoreStub) DeleteArchive(ctx context.Context, archiveID string) error {
	stub.deleteContext = ctx
	stub.deletedID = archiveID
	return nil
}

type jobQueueStub struct {
	called bool
	err    error
}

func (stub *jobQueueStub) EnqueueArchive(context.Context, int64, string) error {
	stub.called = true
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

func TestServiceByAuthorNormalizesEmail(t *testing.T) {
	store := &submissionStoreStub{authorSubmissions: []Submission{{ID: 7}}}
	service := NewService(store, nil, nil, nil, nil, "")

	items, err := service.ByAuthor(context.Background(), "  Ivan@Example.RU ")
	if err != nil {
		t.Fatalf("ByAuthor: %v", err)
	}
	if store.authorEmail != "ivan@example.ru" {
		t.Fatalf("store got email %q, want lowercase trimmed", store.authorEmail)
	}
	if len(items) != 1 || items[0].ID != 7 {
		t.Fatalf("got %v, want one submission with ID 7", items)
	}

	store.authorEmail = "not called"
	if items, err = service.ByAuthor(context.Background(), "   "); err != nil || items != nil {
		t.Fatalf("empty email: got %v, %v; want nil, nil", items, err)
	}
	if store.authorEmail != "not called" {
		t.Fatal("empty email must not reach the store")
	}
}
