package v2

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	legacymigrations "github.com/airsss993/histproject/backend/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMigrationPreservesLegacyData(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatalf("database %q is not an isolated test database", databaseName)
	}

	resetDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetDatabase(t, cleanupCtx, db)
	})

	if err := legacymigrations.Run(db); err != nil {
		t.Fatalf("run legacy migrations: %v", err)
	}
	seedLegacyData(t, ctx, db)

	before := readSnapshot(t, ctx, db, legacyTables(), legacySequences())
	if err := Run(ctx, db); err != nil {
		t.Fatalf("run V2 migrations: %v", err)
	}
	after := readSnapshot(t, ctx, db, targetTables(), targetSequences())

	if !reflect.DeepEqual(before.Counts, after.Counts) {
		t.Fatalf("counts after migration = %#v, want %#v", after.Counts, before.Counts)
	}
	if !reflect.DeepEqual(before.SequenceValues, after.SequenceValues) {
		t.Fatalf("sequences after migration = %#v, want %#v", after.SequenceValues, before.SequenceValues)
	}

	assertDomainRows(t, ctx, db)
	assertSchemaBoundaries(t, ctx, db)
	assertSubmissionContract(t, ctx, db)

	if err := Run(ctx, db); err != nil {
		t.Fatalf("run V2 migrations twice: %v", err)
	}

	provider, err := newProvider(db, embeddedMigrations)
	if err != nil {
		t.Fatalf("new provider for down check: %v", err)
	}
	if _, err := provider.ApplyVersion(ctx, 20260913000200, false); err == nil || !strings.Contains(err.Error(), "forward-only") {
		t.Fatalf("down error = %v, want forward-only refusal", err)
	}
}

type databaseSnapshot struct {
	Counts         []int64
	SequenceValues []int64
}

func readSnapshot(t *testing.T, ctx context.Context, db *sql.DB, tables, sequences []string) databaseSnapshot {
	t.Helper()

	counts := make([]int64, 0, len(tables))
	for _, table := range tables {
		var count int64
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts = append(counts, count)
	}

	sequenceValues := make([]int64, 0, len(sequences))
	for _, sequence := range sequences {
		var value int64
		if err := db.QueryRowContext(ctx, `SELECT last_value FROM `+sequence).Scan(&value); err != nil {
			t.Fatalf("read sequence %s: %v", sequence, err)
		}
		sequenceValues = append(sequenceValues, value)
	}
	return databaseSnapshot{Counts: counts, SequenceValues: sequenceValues}
}

func seedLegacyData(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var adminID int
	err := db.QueryRowContext(ctx, `
		INSERT INTO admins (login, password_hash, role, email)
		VALUES ('root', '$2a$12$test-hash', 'super_admin', 'root@example.test')
		RETURNING id
	`).Scan(&adminID)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO admin_sessions (admin_id, refresh_token, expires_at)
		VALUES ($1, 'test-refresh-token', NOW() + INTERVAL '1 hour')
	`, adminID); err != nil {
		t.Fatalf("insert legacy session: %v", err)
	}

	statuses := []string{"В обработке", "Новая", "На проверке", "Отклонена", "Опубликована"}
	for index, status := range statuses {
		var submissionID int
		err := db.QueryRowContext(ctx, `
			INSERT INTO requests (
				title, description, event_date, event_type_id, email,
				telegram_username, archive_id, site_url, screenshot_url, slug, status
			)
			VALUES ($1, 'description', DATE '2020-01-02', $2, 'author@example.test',
				'author', $3, $4, $5, $6, $7::request_status)
			RETURNING id
		`, fmt.Sprintf("Submission %d", index+1), eventTypeID(index),
			fmt.Sprintf("archives/%d/site.zip", index+1),
			fmt.Sprintf("https://storage.test/sites/%d", index+1),
			fmt.Sprintf("https://storage.test/previews/%d.png", index+1),
			fmt.Sprintf("submission-%d", index+1), status,
		).Scan(&submissionID)
		if err != nil {
			t.Fatalf("insert request %d: %v", index+1, err)
		}

		if index < 3 {
			_, err = db.ExecContext(ctx, `
				INSERT INTO objects (
					request_id, title, description, latitude, longitude,
					event_date, event_type_id, site_url, preview_image_url
				)
				VALUES ($1, $2, 'description', 55.0, 60.0,
					DATE '2020-01-02', $3, $4, $5)
			`, submissionID, fmt.Sprintf("Event %d", index+1), eventTypeID(index),
				fmt.Sprintf("https://storage.test/sites/%d", index+1),
				fmt.Sprintf("https://storage.test/previews/%d.png", index+1),
			)
			if err != nil {
				t.Fatalf("insert object %d: %v", index+1, err)
			}
		}
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO request_audit_log (request_id, admin_login, admin_role, action, comment)
		VALUES (1, 'root', 'super_admin', 'published', 'test comment')
	`); err != nil {
		t.Fatalf("insert audit log: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO bot_subscriptions (username, chat_id)
		VALUES ('test-user', 123456)
	`); err != nil {
		t.Fatalf("insert subscription: %v", err)
	}
}

func eventTypeID(index int) int {
	values := []int{1, 2, 9, 1, 2}
	return values[index]
}

func assertDomainRows(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var adminID int
	var passwordHash string
	if err := db.QueryRowContext(ctx, `SELECT id, password_hash FROM auth.admins WHERE login = 'root'`).Scan(&adminID, &passwordHash); err != nil {
		t.Fatalf("read migrated admin: %v", err)
	}
	if adminID != 1 || passwordHash != "$2a$12$test-hash" {
		t.Fatalf("migrated admin = (%d, %q)", adminID, passwordHash)
	}
	var sessionAdminID int
	var refreshToken string
	if err := db.QueryRowContext(ctx, `SELECT admin_id, refresh_token FROM auth.legacy_sessions`).Scan(&sessionAdminID, &refreshToken); err != nil {
		t.Fatalf("read migrated legacy session: %v", err)
	}
	if sessionAdminID != adminID || refreshToken != "test-refresh-token" {
		t.Fatalf("migrated legacy session = (%d, %q)", sessionAdminID, refreshToken)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, status::TEXT, event_type_id, event_type_code, archive_id, site_url, screenshot_url
		FROM submissions.submissions
		ORDER BY id
	`)
	if err != nil {
		t.Fatalf("read migrated submissions: %v", err)
	}
	wantStatuses := []string{"processing", "new", "review", "rejected", "published"}
	wantCodes := []string{"political", "military", "legacy_9", "political", "military"}
	for index := range wantStatuses {
		var id, eventType int
		var status, code, archiveID, siteURL, screenshotURL string
		if !rows.Next() {
			t.Fatalf("submission row %d is missing", index+1)
		}
		if err := rows.Scan(&id, &status, &eventType, &code, &archiveID, &siteURL, &screenshotURL); err != nil {
			t.Fatalf("scan submission %d: %v", index+1, err)
		}
		wantSiteURL := fmt.Sprintf("https://storage.test/sites/%d", index+1)
		wantScreenshotURL := fmt.Sprintf("https://storage.test/previews/%d.png", index+1)
		if id != index+1 || status != wantStatuses[index] || eventType != eventTypeID(index) || code != wantCodes[index] || archiveID != fmt.Sprintf("archives/%d/site.zip", index+1) || siteURL != wantSiteURL || screenshotURL != wantScreenshotURL {
			t.Errorf("submission %d = id:%d status:%s event_type:%d code:%s archive:%s site:%s screenshot:%s", index+1, id, status, eventType, code, archiveID, siteURL, screenshotURL)
		}
	}
	if rows.Next() {
		t.Fatal("unexpected extra submission row")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate submissions: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close submissions: %v", err)
	}

	eventRows, err := db.QueryContext(ctx, `
		SELECT id, submission_id, event_type_id, event_type_code, site_url, preview_image_url
		FROM events.events
		ORDER BY id
	`)
	if err != nil {
		t.Fatalf("read migrated events: %v", err)
	}
	for index, wantCode := range []string{"political", "military", "legacy_9"} {
		if !eventRows.Next() {
			t.Fatalf("event row %d is missing", index+1)
		}
		var id, submissionID, eventType int
		var code, siteURL, previewURL string
		if err := eventRows.Scan(&id, &submissionID, &eventType, &code, &siteURL, &previewURL); err != nil {
			t.Fatalf("scan event %d: %v", index+1, err)
		}
		if id != index+1 || submissionID != index+1 || eventType != eventTypeID(index) || code != wantCode || siteURL != fmt.Sprintf("https://storage.test/sites/%d", index+1) || previewURL != fmt.Sprintf("https://storage.test/previews/%d.png", index+1) {
			t.Errorf("event %d = id:%d submission:%d event_type:%d code:%s site:%s preview:%s", index+1, id, submissionID, eventType, code, siteURL, previewURL)
		}
	}
	if eventRows.Next() {
		t.Fatal("unexpected extra event row")
	}
	if err := eventRows.Err(); err != nil {
		t.Fatalf("iterate events: %v", err)
	}
	if err := eventRows.Close(); err != nil {
		t.Fatalf("close events: %v", err)
	}

	var auditSubmissionID int
	var auditComment string
	if err := db.QueryRowContext(ctx, `SELECT submission_id, comment FROM submissions.audit_log`).Scan(&auditSubmissionID, &auditComment); err != nil {
		t.Fatalf("read migrated audit log: %v", err)
	}
	if auditSubmissionID != 1 || auditComment != "test comment" {
		t.Fatalf("migrated audit log = (%d, %q)", auditSubmissionID, auditComment)
	}

	var username string
	var chatID int64
	if err := db.QueryRowContext(ctx, `SELECT username, chat_id FROM notifications.telegram_subscriptions`).Scan(&username, &chatID); err != nil {
		t.Fatalf("read migrated subscription: %v", err)
	}
	if username != "test-user" || chatID != 123456 {
		t.Fatalf("migrated subscription = (%q, %d)", username, chatID)
	}
}

func assertSubmissionContract(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `
		INSERT INTO events.events (
			submission_id, title, description, latitude, longitude,
			event_date, event_type_id, event_type_code, site_url
		)
		VALUES (1, 'Duplicate', 'description', 55, 60,
			DATE '2020-01-02', 1, 'political', 'https://storage.test/sites/duplicate')
	`)
	if err == nil {
		t.Fatal("duplicate event submission_id was accepted")
	}
}

func assertSchemaBoundaries(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	for _, table := range legacyTables() {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check legacy table %s: %v", table, err)
		}
		if exists {
			t.Errorf("legacy table %s still exists", table)
		}
	}

	var crossModuleForeignKeys int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pg_constraint AS constraint_info
		JOIN pg_class AS child_table ON child_table.oid = constraint_info.conrelid
		JOIN pg_namespace AS child_schema ON child_schema.oid = child_table.relnamespace
		JOIN pg_class AS parent_table ON parent_table.oid = constraint_info.confrelid
		JOIN pg_namespace AS parent_schema ON parent_schema.oid = parent_table.relnamespace
		WHERE constraint_info.contype = 'f'
			AND child_schema.nspname IN ('auth', 'submissions', 'events', 'notifications')
			AND child_schema.nspname <> parent_schema.nspname
	`).Scan(&crossModuleForeignKeys)
	if err != nil {
		t.Fatalf("count cross-module foreign keys: %v", err)
	}
	if crossModuleForeignKeys != 0 {
		t.Fatalf("cross-module foreign key count = %d, want 0", crossModuleForeignKeys)
	}
}

func resetDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `
		DROP SCHEMA IF EXISTS auth CASCADE;
		DROP SCHEMA IF EXISTS submissions CASCADE;
		DROP SCHEMA IF EXISTS events CASCADE;
		DROP SCHEMA IF EXISTS notifications CASCADE;
		DROP TABLE IF EXISTS public.request_audit_log CASCADE;
		DROP TABLE IF EXISTS public.bot_subscriptions CASCADE;
		DROP TABLE IF EXISTS public.objects CASCADE;
		DROP TABLE IF EXISTS public.requests CASCADE;
		DROP TABLE IF EXISTS public.admin_sessions CASCADE;
		DROP TABLE IF EXISTS public.admins CASCADE;
		DROP TYPE IF EXISTS public.request_status CASCADE;
		DROP TYPE IF EXISTS public.admin_role CASCADE;
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}

func legacyTables() []string {
	return []string{
		"public.admins",
		"public.admin_sessions",
		"public.requests",
		"public.objects",
		"public.request_audit_log",
		"public.bot_subscriptions",
	}
}

func targetTables() []string {
	return []string{
		"auth.admins",
		"auth.legacy_sessions",
		"submissions.submissions",
		"events.events",
		"submissions.audit_log",
		"notifications.telegram_subscriptions",
	}
}

func legacySequences() []string {
	return []string{
		"public.admins_id_seq",
		"public.admin_sessions_id_seq",
		"public.requests_id_seq",
		"public.objects_id_seq",
		"public.request_audit_log_id_seq",
		"public.bot_subscriptions_id_seq",
	}
}

func targetSequences() []string {
	return []string{
		"auth.admins_id_seq",
		"auth.legacy_sessions_id_seq",
		"submissions.submissions_id_seq",
		"events.events_id_seq",
		"submissions.audit_log_id_seq",
		"notifications.telegram_subscriptions_id_seq",
	}
}
