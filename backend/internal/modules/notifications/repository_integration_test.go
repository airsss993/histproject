package notifications

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/migrations"
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
	if err := migrations.Up(ctx, adminDB); err != nil {
		t.Fatalf("run migrations: %v", err)
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
		DROP SCHEMA IF EXISTS users CASCADE;
		DROP FUNCTION IF EXISTS public.rh_code();
		DROP TABLE IF EXISTS public.goose_db_version CASCADE;
	`)
	if err != nil {
		t.Fatalf("reset database: %v", err)
	}
}

func TestRepositoryStoresBotAndRecipients(t *testing.T) {
	repository, _ := notificationRepositoryTestDatabase(t)
	ctx := t.Context()

	if token, username, err := repository.BotToken(ctx); err != nil || token != "" || username != "" {
		t.Fatalf("empty BotToken() = %q, %q, %v", token, username, err)
	}
	if err := repository.SaveBot(ctx, "1:old", "old_bot"); err != nil {
		t.Fatalf("SaveBot() error = %v", err)
	}
	if err := repository.SaveBot(ctx, "1:new", "new_bot"); err != nil {
		t.Fatalf("second SaveBot() error = %v", err)
	}
	if token, username, err := repository.BotToken(ctx); err != nil || token != "1:new" || username != "new_bot" {
		t.Fatalf("BotToken() = %q, %q, %v", token, username, err)
	}

	for _, chat := range []TelegramChat{{ID: 5, Title: "Иван"}, {ID: -100, Title: "Группа"}, {ID: 5, Title: "Иван Петров", Username: "ivan"}} {
		if err := repository.SaveRecipient(ctx, chat); err != nil {
			t.Fatalf("SaveRecipient(%d) error = %v", chat.ID, err)
		}
	}
	recipients, err := repository.Recipients(ctx)
	if err != nil || len(recipients) != 2 || recipients[0] != (TelegramChat{ID: 5, Title: "Иван Петров", Username: "ivan"}) {
		t.Fatalf("Recipients() = %#v, %v", recipients, err)
	}
	if err := repository.DeleteRecipient(ctx, 5); err != nil {
		t.Fatalf("DeleteRecipient() error = %v", err)
	}
	if err := repository.DeleteRecipient(ctx, 5); !errors.Is(err, ErrRecipientNotFound) {
		t.Fatalf("repeat DeleteRecipient() error = %v", err)
	}
	if err := repository.DeleteBot(ctx); err != nil {
		t.Fatalf("DeleteBot() error = %v", err)
	}
	if token, _, err := repository.BotToken(ctx); err != nil || token != "" {
		t.Fatalf("BotToken() after delete = %q, %v", token, err)
	}
}

func TestRepositoryStoresMailSender(t *testing.T) {
	repository, _ := notificationRepositoryTestDatabase(t)
	ctx := t.Context()

	if from, err := repository.MailFrom(ctx); err != nil || from != "" {
		t.Fatalf("empty MailFrom() = %q, %v", from, err)
	}
	if err := repository.SaveMailSender(ctx, "Старое", "old@example.test"); err != nil {
		t.Fatalf("SaveMailSender() error = %v", err)
	}
	if err := repository.SaveMailSender(ctx, "", "news@example.test"); err != nil {
		t.Fatalf("SaveMailSender() second error = %v", err)
	}
	if name, address, err := repository.MailSender(ctx); err != nil || name != "" || address != "news@example.test" {
		t.Fatalf("MailSender() = %q, %q, %v", name, address, err)
	}
	if from, err := repository.MailFrom(ctx); err != nil || from != "<news@example.test>" {
		t.Fatalf("MailFrom() = %q, %v", from, err)
	}
}
