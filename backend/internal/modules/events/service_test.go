package events

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestListNormalizesFiltersAndPagination(t *testing.T) {
	store := &eventStoreStub{page: Page{Total: 2}}
	service := NewService(store, nil)

	page, err := service.List(t.Context(), ListInput{
		EventTypes: []string{" military ", "political"},
		DateFrom:   "2020-01-02",
		DateTo:     "2021-03-04",
	}, Viewer{UserID: 8})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.filter.ViewerID != 8 {
		t.Fatalf("viewer = %d, want 8", store.filter.ViewerID)
	}
	if page.Total != 2 {
		t.Fatalf("List() = %#v", page)
	}
	if store.filter.Page != 1 || store.filter.PageSize != 50 {
		t.Fatalf("pagination = (%d, %d), want (1, 50)", store.filter.Page, store.filter.PageSize)
	}
	if len(store.filter.EventTypes) != 2 || store.filter.EventTypes[0] != "military" {
		t.Fatalf("event types = %#v", store.filter.EventTypes)
	}
	if store.filter.DateFrom.Format(time.DateOnly) != "2020-01-02" || store.filter.DateTo.Format(time.DateOnly) != "2021-03-04" {
		t.Fatalf("date range = (%s, %s)", store.filter.DateFrom, store.filter.DateTo)
	}
}

func TestListRejectsInvalidFilters(t *testing.T) {
	tests := []struct {
		name  string
		input ListInput
		field string
	}{
		{name: "event type", input: ListInput{EventTypes: []string{"Military history"}}, field: "eventType"},
		{name: "date from", input: ListInput{DateFrom: "02.01.2020"}, field: "dateFrom"},
		{name: "date to", input: ListInput{DateTo: "tomorrow"}, field: "dateTo"},
		{name: "reversed range", input: ListInput{DateFrom: "2021-01-02", DateTo: "2021-01-01"}, field: "dateTo"},
		{name: "page", input: ListInput{Page: -1}, field: "page"},
		{name: "page size", input: ListInput{PageSize: 101}, field: "pageSize"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &eventStoreStub{}
			service := NewService(store, nil)
			_, err := service.List(t.Context(), test.input, Viewer{})
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Fields[test.field] == "" {
				t.Fatalf("List() error = %v, want %s validation", err, test.field)
			}
			if store.listCalled {
				t.Fatal("store.List was called for invalid input")
			}
		})
	}
}

func TestGetRejectsNonPositiveID(t *testing.T) {
	store := &eventStoreStub{}
	service := NewService(store, nil)

	_, err := service.Get(t.Context(), 0, Viewer{})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["id"] == "" {
		t.Fatalf("Get() error = %v, want id validation", err)
	}
	if store.getCalled {
		t.Fatal("store.Get was called for invalid ID")
	}
}

func TestUpdateCoordinatesValidatesRanges(t *testing.T) {
	tests := []struct {
		name      string
		latitude  float64
		longitude float64
		field     string
	}{
		{name: "latitude below", latitude: -91, field: "latitude"},
		{name: "latitude above", latitude: 91, field: "latitude"},
		{name: "latitude NaN", latitude: math.NaN(), field: "latitude"},
		{name: "longitude below", longitude: -181, field: "longitude"},
		{name: "longitude above", longitude: 181, field: "longitude"},
		{name: "longitude infinity", longitude: math.Inf(1), field: "longitude"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &eventStoreStub{}
			service := NewService(store, nil)
			err := service.UpdateCoordinates(t.Context(), 1, test.latitude, test.longitude)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Fields[test.field] == "" {
				t.Fatalf("UpdateCoordinates() error = %v, want %s validation", err, test.field)
			}
			if store.updateCalled {
				t.Fatal("store.UpdateCoordinates was called for invalid coordinates")
			}
		})
	}
}

func TestDeleteDelegatesPositiveID(t *testing.T) {
	store := &eventStoreStub{}
	service := NewService(store, nil)

	if err := service.Delete(t.Context(), 9); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.deletedID != 9 {
		t.Fatalf("deleted ID = %d, want 9", store.deletedID)
	}
}

func TestPublishDelegatesSnapshot(t *testing.T) {
	store := &eventStoreStub{event: Event{ID: 12, SubmissionID: 7}}
	service := NewService(store, nil)
	snapshot := Snapshot{
		SubmissionID: 7, Title: "Event", Description: "Description",
		Latitude: 55.5, Longitude: 60.5, EventDate: mustServiceDate(t, "2020-01-02"),
		EventType: "military", SiteURL: "https://example.test/sites/event",
		PreviewImageURL: "https://example.test/sites/event/screenshot.png",
	}

	event, err := service.Publish(t.Context(), snapshot)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if event.ID != 12 || store.snapshot != snapshot {
		t.Fatalf("event = %#v, snapshot = %#v", event, store.snapshot)
	}
}

type eventStoreStub struct {
	filter       Filter
	page         Page
	listCalled   bool
	listError    error
	getCalled    bool
	event        Event
	getError     error
	updateCalled bool
	updateError  error
	deletedID    int64
	deleteError  error
	snapshot     Snapshot
	publishError error
	starCalled   bool
	viewUserID   int64
	viewStarted  bool
	touchSeconds int
}

func (stub *eventStoreStub) Publish(_ context.Context, snapshot Snapshot) (Event, error) {
	stub.snapshot = snapshot
	return stub.event, stub.publishError
}

func (stub *eventStoreStub) List(_ context.Context, filter Filter) (Page, error) {
	stub.listCalled = true
	stub.filter = filter
	return stub.page, stub.listError
}

func (stub *eventStoreStub) Get(context.Context, int64, int64) (Event, error) {
	stub.getCalled = true
	return stub.event, stub.getError
}

func (stub *eventStoreStub) UpdateCoordinates(context.Context, int64, float64, float64) error {
	stub.updateCalled = true
	return stub.updateError
}

func (stub *eventStoreStub) Delete(_ context.Context, id int64) error {
	stub.deletedID = id
	return stub.deleteError
}

func (stub *eventStoreStub) SetStar(_ context.Context, _, _ int64, on bool) (StarState, error) {
	stub.starCalled = true
	return StarState{Stars: 1, Starred: on}, nil
}

func (stub *eventStoreStub) StartView(_ context.Context, _, userID int64, _ string) (int64, error) {
	stub.viewStarted, stub.viewUserID = true, userID
	return 5, nil
}

func (stub *eventStoreStub) TouchView(_ context.Context, _, _ int64, _ string, seconds int) error {
	stub.touchSeconds = seconds
	return nil
}

func (stub *eventStoreStub) Summaries(context.Context, []int64, int64) ([]Summary, error) {
	return nil, nil
}

type authorsStub string

func (stub authorsStub) AuthorEmail(context.Context, int64) (string, error) { return string(stub), nil }

func TestServiceStarRequiresAccount(t *testing.T) {
	store := &eventStoreStub{}
	service := NewService(store, nil)
	if _, err := service.Star(t.Context(), 3, Viewer{}, true); !errors.Is(err, ErrUnauthorized) || store.starCalled {
		t.Fatalf("guest Star() error = %v, store called = %v", err, store.starCalled)
	}
	state, err := service.Star(t.Context(), 3, Viewer{UserID: 9}, true)
	if err != nil || !state.Starred {
		t.Fatalf("Star() = %#v, %v", state, err)
	}
}

func TestServiceStartViewSkipsAuthor(t *testing.T) {
	const visitor = "1b4e28ba-2fa1-41d2-883f-0016d3cca427"
	store := &eventStoreStub{event: Event{ID: 3, SubmissionID: 7}}
	service := NewService(store, authorsStub("Author@Example.ru"))

	if _, recorded, err := service.StartView(t.Context(), 3, Viewer{UserID: 2, Email: "author@example.ru"}, visitor); err != nil || recorded || store.viewStarted {
		t.Fatalf("author StartView() recorded = %v, error = %v, store called = %v", recorded, err, store.viewStarted)
	}
	if viewID, recorded, err := service.StartView(t.Context(), 3, Viewer{UserID: 4, Email: "reader@example.ru"}, visitor); err != nil || !recorded || viewID != 5 || store.viewUserID != 4 {
		t.Fatalf("reader StartView() = %d, %v, %v, user %d", viewID, recorded, err, store.viewUserID)
	}
	var validationError *ValidationError
	if _, _, err := service.StartView(t.Context(), 3, Viewer{}, "not-a-uuid"); !errors.As(err, &validationError) {
		t.Fatalf("bad visitor StartView() error = %v", err)
	}
}

func TestServiceTouchViewCapsSeconds(t *testing.T) {
	const visitor = "1b4e28ba-2fa1-41d2-883f-0016d3cca427"
	store := &eventStoreStub{}
	service := NewService(store, nil)
	if err := service.TouchView(t.Context(), 3, 5, visitor, 10*MaxViewSeconds); err != nil || store.touchSeconds != MaxViewSeconds {
		t.Fatalf("TouchView() error = %v, seconds = %d", err, store.touchSeconds)
	}
	var validationError *ValidationError
	if err := service.TouchView(t.Context(), 3, 5, visitor, -1); !errors.As(err, &validationError) {
		t.Fatalf("negative TouchView() error = %v", err)
	}
}

func mustServiceDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return date
}
