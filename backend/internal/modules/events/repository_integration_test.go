package events

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepositoryEventLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_EVENTS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_EVENTS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := prepareEventsDatabase(t, ctx, databaseURL)
	seedEvents(t, ctx, db)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)

	dateFrom := mustDate(t, "2019-01-01")
	dateTo := mustDate(t, "2021-12-31")
	firstPage, err := repository.List(ctx, Filter{
		EventTypes: []string{"political"},
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
		Page:       1,
		PageSize:   1,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if firstPage.Total != 2 || firstPage.Page != 1 || firstPage.PageSize != 1 || len(firstPage.Items) != 1 || firstPage.Items[0].ID != 1 {
		t.Fatalf("first page = %#v", firstPage)
	}
	secondPage, err := repository.List(ctx, Filter{EventTypes: []string{"political"}, DateFrom: &dateFrom, DateTo: &dateTo, Page: 2, PageSize: 1})
	if err != nil {
		t.Fatalf("List() second page error = %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].ID != 3 {
		t.Fatalf("second page = %#v", secondPage)
	}

	event, err := repository.Get(ctx, 2, 0)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if event.SubmissionID != 2 || event.EventType != "military" || event.PreviewImageURL == nil || *event.PreviewImageURL != "https://storage.test/previews/2.png" {
		t.Fatalf("event = %#v", event)
	}
	if _, err := repository.Get(ctx, 999, 0); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("missing Get() error = %v, want ErrEventNotFound", err)
	}

	if err := repository.UpdateCoordinates(ctx, 2, 90, -180); err != nil {
		t.Fatalf("UpdateCoordinates() error = %v", err)
	}
	event, err = repository.Get(ctx, 2, 0)
	if err != nil || event.Latitude != 90 || event.Longitude != -180 {
		t.Fatalf("updated event = %#v, error = %v", event, err)
	}
	if err := repository.UpdateCoordinates(ctx, 999, 1, 1); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("missing UpdateCoordinates() error = %v", err)
	}

	if err := repository.Delete(ctx, 2); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repository.Get(ctx, 2, 0); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("deleted Get() error = %v", err)
	}
	var submissionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM submissions.submissions WHERE id = 2`).Scan(&submissionCount); err != nil {
		t.Fatalf("count preserved submission: %v", err)
	}
	if submissionCount != 1 {
		t.Fatalf("submission count = %d, want 1", submissionCount)
	}
	if err := repository.Delete(ctx, 999); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("missing Delete() error = %v", err)
	}
}

func TestRepositoryPublishUpsertsBySubmissionID(t *testing.T) {
	databaseURL := os.Getenv("TEST_EVENTS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_EVENTS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := prepareEventsDatabase(t, ctx, databaseURL)
	seedEvents(t, ctx, db)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)

	first, err := repository.Publish(ctx, Snapshot{
		SubmissionID: 2, Title: "Updated event", Description: "Updated description",
		Latitude: 50.1, Longitude: 60.2, EventDate: mustDate(t, "1945-05-09"),
		EventType: "military", SiteURL: "https://storage.test/sites/updated/index.html",
		PreviewImageURL: "https://storage.test/sites/updated/screenshot.png",
	})
	if err != nil {
		t.Fatalf("Publish() first error = %v", err)
	}
	second, err := repository.Publish(ctx, Snapshot{
		SubmissionID: 2, Title: "Final event", Description: "Final description",
		Latitude: 51.1, Longitude: 61.2, EventDate: mustDate(t, "1945-05-10"),
		EventType: "military", SiteURL: "https://storage.test/sites/final/index.html",
		PreviewImageURL: "https://storage.test/sites/final/screenshot.png",
	})
	if err != nil {
		t.Fatalf("Publish() retry error = %v", err)
	}
	if first.ID != second.ID || second.SubmissionID != 2 || second.Title != "Final event" || second.Latitude != 51.1 {
		t.Fatalf("upsert results = first:%#v second:%#v", first, second)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events.events WHERE submission_id = 2`).Scan(&count); err != nil {
		t.Fatalf("count published events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
}

func TestRepositoryStarsAndViews(t *testing.T) {
	databaseURL := os.Getenv("TEST_EVENTS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_EVENTS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := prepareEventsDatabase(t, ctx, databaseURL)
	seedEvents(t, ctx, db)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users.users (id, email, first_name, last_name)
		VALUES (1, 'reader@example.test', 'Анна', 'Петрова'), (2, 'other@example.test', 'Иван', 'Иванов')
	`); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)

	// Повторная звезда не удваивает рейтинг, снятие отсутствующей - не ошибка.
	for range 2 {
		state, err := repository.SetStar(ctx, 1, 1, true)
		if err != nil || state != (StarState{Stars: 1, Starred: true}) {
			t.Fatalf("SetStar(on) = %#v, %v", state, err)
		}
	}
	if state, err := repository.SetStar(ctx, 1, 2, true); err != nil || state != (StarState{Stars: 2, Starred: true}) {
		t.Fatalf("second user SetStar = %#v, %v", state, err)
	}
	for range 2 {
		state, err := repository.SetStar(ctx, 1, 2, false)
		if err != nil || state != (StarState{Stars: 1, Starred: false}) {
			t.Fatalf("SetStar(off) = %#v, %v", state, err)
		}
	}
	if _, err := repository.SetStar(ctx, 999, 1, true); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("missing event SetStar(on) error = %v", err)
	}
	if _, err := repository.SetStar(ctx, 999, 1, false); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("missing event SetStar(off) error = %v", err)
	}

	event, err := repository.Get(ctx, 1, 1)
	if err != nil || event.Stars != 1 || !event.Starred {
		t.Fatalf("Get() by starring user = %#v, %v", event, err)
	}
	if event, err = repository.Get(ctx, 1, 0); err != nil || event.Stars != 1 || event.Starred {
		t.Fatalf("Get() by guest = %#v, %v", event, err)
	}
	page, err := repository.List(ctx, Filter{Page: 1, PageSize: 10, ViewerID: 1})
	if err != nil || len(page.Items) != 4 {
		t.Fatalf("List() = %#v, %v", page, err)
	}
	for _, item := range page.Items {
		if want := item.ID == 1; item.Starred != want || (item.Stars == 1) != want {
			t.Fatalf("List() item %d stars = %d starred = %v", item.ID, item.Stars, item.Starred)
		}
	}

	const guest = "1b4e28ba-2fa1-41d2-883f-0016d3cca427"
	const stranger = "9f0c2b7e-5d1a-4c3b-8e6f-7a8b9c0d1e2f"
	first, err := repository.StartView(ctx, 1, 0, guest)
	if err != nil {
		t.Fatalf("StartView() guest error = %v", err)
	}
	if _, err := repository.StartView(ctx, 1, 0, guest); err != nil {
		t.Fatalf("StartView() repeat error = %v", err)
	}
	if _, err := repository.StartView(ctx, 1, 1, stranger); err != nil {
		t.Fatalf("StartView() user error = %v", err)
	}
	if _, err := repository.StartView(ctx, 999, 0, guest); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("missing event StartView() error = %v", err)
	}
	// Время просмотра не больше прошедшего с начала: только что открытый просмотр не наберёт час.
	if _, err := db.ExecContext(ctx, `UPDATE events.views SET started_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, first); err != nil {
		t.Fatalf("age view: %v", err)
	}
	if err := repository.TouchView(ctx, 1, first, guest, 40, 0); err != nil {
		t.Fatalf("TouchView() error = %v", err)
	}
	if err := repository.TouchView(ctx, 1, first, guest, 30, 0); err != nil {
		t.Fatalf("TouchView() smaller error = %v", err)
	}
	if err := repository.TouchView(ctx, 1, first, stranger, 50, 0); !errors.Is(err, ErrViewNotFound) {
		t.Fatalf("TouchView() by other visitor error = %v", err)
	}
	if err := repository.TouchView(ctx, 2, first, guest, 50, 0); !errors.Is(err, ErrViewNotFound) {
		t.Fatalf("TouchView() with other event error = %v", err)
	}
	if err := repository.TouchView(ctx, 1, first+1, guest, 3600, 0); err != nil {
		t.Fatalf("TouchView() fresh view error = %v", err)
	}

	summaries, err := repository.Summaries(ctx, []int64{1, 2, 404}, 1)
	if err != nil || len(summaries) != 2 {
		t.Fatalf("Summaries() = %#v, %v", summaries, err)
	}
	for _, summary := range summaries {
		switch summary.SubmissionID {
		case 1:
			// 40 секунд первого просмотра + не больше ~5 у свежего второго; два браузера-гостя = один, плюс аккаунт.
			if summary.EventID != 1 || summary.Stars != 1 || !summary.Starred || summary.Views != 3 || summary.Visitors != 2 ||
				summary.ActiveSeconds < 40 || summary.ActiveSeconds > 50 {
				t.Fatalf("summary 1 = %#v", summary)
			}
		case 2:
			if summary.Stars != 0 || summary.Starred || summary.Views != 0 || summary.Visitors != 0 || summary.ActiveSeconds != 0 {
				t.Fatalf("summary 2 = %#v", summary)
			}
		default:
			t.Fatalf("unexpected summary %#v", summary)
		}
	}
	if event, err := repository.Get(ctx, 1, 0); err != nil || event.Visitors != 2 {
		t.Fatalf("Get() visitors = %d, %v", event.Visitors, err)
	}
	if page, err = repository.List(ctx, Filter{Page: 1, PageSize: 10}); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, item := range page.Items {
		if want := map[bool]int64{true: 2}[item.ID == 1]; item.Visitors != want {
			t.Fatalf("List() item %d visitors = %d, want %d", item.ID, item.Visitors, want)
		}
	}

	if err := repository.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	var left int
	if err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM events.stars) + (SELECT COUNT(*) FROM events.views)`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("stars and views after event delete = %d, %v", left, err)
	}
}

func prepareEventsDatabase(t *testing.T, ctx context.Context, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		_ = db.Close()
		t.Fatalf("read database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		_ = db.Close()
		t.Fatalf("database %q is not an isolated test database", databaseName)
	}
	resetEventsDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetEventsDatabase(t, cleanupCtx, db)
		_ = db.Close()
	})
	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

func seedEvents(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	eventTypes := []string{"political", "military", "political", "military"}
	dates := []string{"2020-01-01", "2020-01-01", "2021-06-15", "2022-01-01"}
	for index := range eventTypes {
		id := index + 1
		_, err := db.ExecContext(ctx, `
			INSERT INTO submissions.submissions (
				id, title, description, event_date, event_type_code,
				email, telegram_username, archive_id, slug, status
			) VALUES ($1, $2, 'description', $3, $4,
				'author@example.test', 'author', $5, $6, 'published')
		`, id, fmt.Sprintf("Submission %d", id), dates[index], eventTypes[index], fmt.Sprintf("archives/%d.zip", id), fmt.Sprintf("submission-%d", id))
		if err != nil {
			t.Fatalf("insert submission %d: %v", id, err)
		}
		_, err = db.ExecContext(ctx, `
			INSERT INTO events.events (
				id, submission_id, title, description, latitude, longitude,
				event_date, event_type_code, site_url, preview_image_url
			) VALUES ($1::bigint, $1::bigint, $2, 'description', 55 + $1::bigint, 60 + $1::bigint, $3, $4, $5, $6)
		`, id, fmt.Sprintf("Event %d", id), dates[index], eventTypes[index], fmt.Sprintf("https://storage.test/sites/%d", id), fmt.Sprintf("https://storage.test/previews/%d.png", id))
		if err != nil {
			t.Fatalf("insert event %d: %v", id, err)
		}
	}
}

func resetEventsDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		DROP SCHEMA IF EXISTS auth CASCADE;
		DROP SCHEMA IF EXISTS submissions CASCADE;
		DROP SCHEMA IF EXISTS events CASCADE;
		DROP SCHEMA IF EXISTS notifications CASCADE;
		DROP SCHEMA IF EXISTS users CASCADE;
		DROP FUNCTION IF EXISTS public.rh_code();
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	return parsed
}
