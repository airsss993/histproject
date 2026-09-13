package admins

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	legacymigrations "github.com/airsss993/histproject/backend/migrations"
	v2migrations "github.com/airsss993/histproject/backend/migrations/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepositoryFindsCredentialsByLogin(t *testing.T) {
	databaseURL := os.Getenv("TEST_ADMINS_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_ADMINS_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	db := openTestDatabase(t, ctx, databaseURL)
	resetTestDatabase(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetTestDatabase(t, cleanupCtx, db)
		_ = db.Close()
	})

	if err := legacymigrations.Up(ctx, db); err != nil {
		t.Fatalf("run legacy migrations: %v", err)
	}
	if err := v2migrations.Run(ctx, db); err != nil {
		t.Fatalf("run V2 migrations: %v", err)
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO auth.admins (login, password_hash, role, email)
		VALUES ('root', '$2a$12$test-hash', 'super_admin', 'root@example.test')
	`)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := NewRepository(pool)

	adminID, login, passwordHash, role, err := repository.FindCredentialsByLogin(ctx, "root")
	if err != nil {
		t.Fatalf("FindCredentialsByLogin() error = %v", err)
	}
	if adminID != 1 || login != "root" || passwordHash != "$2a$12$test-hash" || role != "super_admin" {
		t.Fatalf("credentials = (%d, %q, %q, %q)", adminID, login, passwordHash, role)
	}

	_, _, _, _, err = repository.FindCredentialsByLogin(ctx, "missing")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing credentials error = %v, want pgx.ErrNoRows", err)
	}
}

func openTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *sql.DB {
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
	return db
}

func resetTestDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
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
