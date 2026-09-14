package migrations

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestInitialSchema(t *testing.T) {
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
	assertTestDatabase(t, ctx, db)
	resetDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetDatabase(t, cleanupCtx, db)
	})

	if err := Up(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := Up(ctx, db); err != nil {
		t.Fatalf("run migrations twice: %v", err)
	}

	for _, table := range []string{
		"auth.admins",
		"submissions.submissions",
		"submissions.audit_log",
		"events.events",
		"notifications.deliveries",
	} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist", table)
		}
	}

	for _, column := range []string{"event_type_id", "request_id"} {
		var count int
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema IN ('submissions', 'events') AND column_name = $1
		`, column).Scan(&count); err != nil {
			t.Fatalf("count obsolete column %s: %v", column, err)
		}
		if count != 0 {
			t.Errorf("obsolete column %s count = %d", column, count)
		}
	}

	assertSchemaConstraints(t, ctx, db)
}

func assertTestDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatalf("database %q is not an isolated test database", databaseName)
	}
}

func assertSchemaConstraints(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO submissions.submissions (
			title, description, event_date, event_type_code,
			email, telegram_username, archive_id, slug
		) VALUES ('Invalid', 'description', DATE '2020-01-02', 'unknown',
			'author@example.test', 'author', 'archive-id', 'invalid')
	`)
	if err == nil {
		t.Fatal("unsupported event type was accepted")
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO events.events (
			submission_id, title, description, latitude, longitude,
			event_date, event_type_code, site_url
		) VALUES
			(1, 'First', 'description', 55, 60, DATE '2020-01-02', 'political', 'https://example.test/first'),
			(1, 'Duplicate', 'description', 55, 60, DATE '2020-01-02', 'political', 'https://example.test/duplicate')
	`); err == nil {
		t.Fatal("duplicate event submission_id was accepted")
	}

	var crossModuleForeignKeys int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pg_constraint AS constraint_info
		JOIN pg_class AS child_table ON child_table.oid = constraint_info.conrelid
		JOIN pg_namespace AS child_schema ON child_schema.oid = child_table.relnamespace
		JOIN pg_class AS parent_table ON parent_table.oid = constraint_info.confrelid
		JOIN pg_namespace AS parent_schema ON parent_schema.oid = parent_table.relnamespace
		WHERE constraint_info.contype = 'f'
			AND child_schema.nspname IN ('auth', 'submissions', 'events', 'notifications')
			AND child_schema.nspname <> parent_schema.nspname
	`).Scan(&crossModuleForeignKeys); err != nil {
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
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}
