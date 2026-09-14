package notifications

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
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepositoryClaimsEachDeliveryUntilSent(t *testing.T) {
	repository, pool := notificationRepositoryTestDatabase(t)
	ctx := t.Context()

	claimed, err := repository.Claim(ctx, "submission:7:reviewed", ChannelEmail)
	if err != nil || !claimed {
		t.Fatalf("first Claim() = %v, %v", claimed, err)
	}
	claimed, err = repository.Claim(ctx, "submission:7:reviewed", ChannelEmail)
	if !errors.Is(err, ErrDeliveryInProgress) || claimed {
		t.Fatalf("concurrent Claim() = %v, %v, want ErrDeliveryInProgress", claimed, err)
	}
	if err := repository.MarkFailed(ctx, "submission:7:reviewed", ChannelEmail, "temporary error"); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}
	claimed, err = repository.Claim(ctx, "submission:7:reviewed", ChannelEmail)
	if err != nil || !claimed {
		t.Fatalf("retry Claim() = %v, %v", claimed, err)
	}
	if err := repository.MarkSent(ctx, "submission:7:reviewed", ChannelEmail); err != nil {
		t.Fatalf("MarkSent() error = %v", err)
	}
	claimed, err = repository.Claim(ctx, "submission:7:reviewed", ChannelEmail)
	if err != nil || claimed {
		t.Fatalf("sent Claim() = %v, %v", claimed, err)
	}

	var status string
	var attempts int
	var deliveredAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, attempts, delivered_at
		FROM notifications.deliveries
		WHERE event_key = $1 AND channel = $2
	`, "submission:7:reviewed", ChannelEmail).Scan(&status, &attempts, &deliveredAt); err != nil {
		t.Fatalf("read delivery: %v", err)
	}
	if status != "sent" || attempts != 2 || deliveredAt == nil {
		t.Fatalf("delivery = status:%q attempts:%d deliveredAt:%v", status, attempts, deliveredAt)
	}
}

func TestRepositoryRequiresClaimBeforeCompletion(t *testing.T) {
	repository, _ := notificationRepositoryTestDatabase(t)

	err := repository.MarkSent(t.Context(), "missing", ChannelTelegram)
	if !errors.Is(err, ErrDeliveryNotClaimed) {
		t.Fatalf("MarkSent() error = %v, want ErrDeliveryNotClaimed", err)
	}
}

func notificationRepositoryTestDatabase(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	adminDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var databaseName string
	if err := adminDB.QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatalf("database %q is not an isolated test database", databaseName)
	}
	resetNotificationDatabase(t, ctx, adminDB)
	if err := legacymigrations.Run(adminDB); err != nil {
		t.Fatalf("run legacy migrations: %v", err)
	}
	if err := v2migrations.Run(ctx, adminDB); err != nil {
		t.Fatalf("run V2 migrations: %v", err)
	}
	if err := adminDB.Close(); err != nil {
		t.Fatalf("close migration database: %v", err)
	}

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupDB, openErr := sql.Open("pgx", databaseURL)
		if openErr != nil {
			t.Errorf("open cleanup database: %v", openErr)
			return
		}
		defer func() {
			if closeErr := cleanupDB.Close(); closeErr != nil {
				t.Errorf("close cleanup database: %v", closeErr)
			}
		}()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetNotificationDatabase(t, cleanupCtx, cleanupDB)
	})
	return NewRepository(pool), pool
}

func resetNotificationDatabase(t *testing.T, ctx context.Context, db *sql.DB) {
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
		DROP TABLE IF EXISTS public.event_types CASCADE;
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset database: %v", err)
	}
}
