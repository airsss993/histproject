package submissions

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepositorySubmissionLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_SUBMISSIONS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_SUBMISSIONS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := prepareSubmissionsDatabase(t, ctx, databaseURL)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)

	created, err := repository.Create(ctx, CreateRecord{
		Title: "Alpha", Description: "First", EventDate: mustRepositoryDate(t, "2020-01-02"),
		EventType: "military", Email: "alpha@example.test", TelegramUsername: "alpha",
		ArchiveID: "archive-alpha", Slug: "alpha",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != 1 || created.Status != StatusProcessing || created.EventType != "military" || created.ArchiveID != "archive-alpha" {
		t.Fatalf("created submission = %#v", created)
	}

	insertSubmissionFixture(t, ctx, db, 2, "Beta", StatusReview, "political", "2021-01-01")
	insertSubmissionFixture(t, ctx, db, 3, "Gamma", StatusRejected, "military", "2022-01-01")
	insertSubmissionFixture(t, ctx, db, 4, "Delta", StatusNew, "military", "2023-01-01")

	detail, err := repository.Get(ctx, 2)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if detail.Title != "Beta" || detail.Status != StatusReview || detail.EventDate.Format(time.DateOnly) != "2021-01-01" {
		t.Fatalf("detail = %#v", detail)
	}
	if _, err := repository.Get(ctx, 999); !errors.Is(err, ErrSubmissionNotFound) {
		t.Fatalf("missing Get() error = %v", err)
	}

	page, err := repository.List(ctx, ListFilter{Status: StatusReview, Search: "bet", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != 2 || page.Page != 1 || page.PageSize != 1 {
		t.Fatalf("page = %#v", page)
	}

	ready, err := repository.Transition(ctx, Transition{
		ID: 1, From: StatusProcessing, To: StatusNew,
		SiteURL: "https://example.test/preview/alpha", ScreenshotURL: "https://example.test/alpha.png",
	})
	if err != nil {
		t.Fatalf("processing transition error = %v", err)
	}
	if ready.Status != StatusNew || ready.SiteURL == "" || ready.ScreenshotURL == "" {
		t.Fatalf("ready submission = %#v", ready)
	}
	reviewed, err := repository.Transition(ctx, Transition{
		ID: 1, From: StatusNew, To: StatusReview,
		Audit: Audit{AdminLogin: "editor", AdminRole: "admin", Action: "reviewed"},
	})
	if err != nil {
		t.Fatalf("review transition error = %v", err)
	}
	if reviewed.Status != StatusReview {
		t.Fatalf("reviewed status = %q", reviewed.Status)
	}

	auditPage, err := repository.AuditLog(ctx, AuditFilter{Search: "Alpha", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("AuditLog() error = %v", err)
	}
	if auditPage.Total != 1 || len(auditPage.Items) != 1 || auditPage.Items[0].SubmissionID != 1 || auditPage.Items[0].Action != "reviewed" {
		t.Fatalf("audit page = %#v", auditPage)
	}

	results := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for attempt := 0; attempt < 2; attempt++ {
		go func() {
			start.Wait()
			_, transitionErr := repository.Transition(t.Context(), Transition{
				ID: 4, From: StatusNew, To: StatusReview,
				Audit: Audit{AdminLogin: "editor", AdminRole: "admin", Action: "reviewed"},
			})
			results <- transitionErr
		}()
	}
	start.Done()
	var succeeded, conflicted int
	for attempt := 0; attempt < 2; attempt++ {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrInvalidTransition):
			conflicted++
		default:
			t.Fatalf("concurrent transition error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent transitions succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	var auditCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM submissions.audit_log WHERE submission_id = 4`).Scan(&auditCount); err != nil {
		t.Fatalf("count concurrent audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("concurrent audit count = %d, want 1", auditCount)
	}

	if err := repository.Delete(ctx, 3); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repository.Get(ctx, 3); !errors.Is(err, ErrSubmissionNotFound) {
		t.Fatalf("Get() after Delete() error = %v", err)
	}
}

func prepareSubmissionsDatabase(t *testing.T, ctx context.Context, databaseURL string) *sql.DB {
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
	resetSubmissionsDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetSubmissionsDatabase(t, cleanupCtx, db)
		_ = db.Close()
	})
	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

func insertSubmissionFixture(t *testing.T, ctx context.Context, db *sql.DB, id int, title string, status Status, eventType, eventDate string) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO submissions.submissions (
			id, title, description, event_date, event_type_code,
			email, telegram_username, archive_id, slug, status
		) VALUES ($1, $2, 'description', $3, $4,
			'author@example.test', 'author', $5, $6, $7)
	`, id, title, eventDate, eventType, "archive-"+strings.ToLower(title), strings.ToLower(title), status)
	if err != nil {
		t.Fatalf("insert fixture %d: %v", id, err)
	}
}

func resetSubmissionsDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		DROP SCHEMA IF EXISTS auth CASCADE;
		DROP SCHEMA IF EXISTS submissions CASCADE;
		DROP SCHEMA IF EXISTS events CASCADE;
		DROP SCHEMA IF EXISTS notifications CASCADE;
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset database: %v", err)
	}
}

func mustRepositoryDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return date
}
