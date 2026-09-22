package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDeliveryInProgress = errors.New("notification delivery is already in progress")
	ErrDeliveryNotClaimed = errors.New("notification delivery is not claimed")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Claim(ctx context.Context, eventKey string, channel Channel) (bool, error) {
	var claimedKey string
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO notifications.deliveries (event_key, channel, status, attempts)
		VALUES ($1, $2, 'processing', 1)
		ON CONFLICT (event_key, channel) DO UPDATE
		SET status = 'processing',
			attempts = notifications.deliveries.attempts + 1,
			last_error = '',
			updated_at = NOW()
		WHERE notifications.deliveries.status = 'failed'
			OR (
				notifications.deliveries.status = 'processing'
				AND notifications.deliveries.updated_at < NOW() - INTERVAL '20 minutes'
			)
		RETURNING event_key
	`, eventKey, channel).Scan(&claimedKey)
	if errors.Is(err, pgx.ErrNoRows) {
		var status string
		if statusErr := repository.pool.QueryRow(ctx, `
			SELECT status
			FROM notifications.deliveries
			WHERE event_key = $1 AND channel = $2
		`, eventKey, channel).Scan(&status); statusErr != nil {
			return false, fmt.Errorf("read notification delivery after claim conflict: %w", statusErr)
		}
		if status == "sent" {
			return false, nil
		}
		return false, ErrDeliveryInProgress
	}
	if err != nil {
		return false, fmt.Errorf("claim notification delivery: %w", err)
	}
	return true, nil
}

func (repository *Repository) MarkSent(ctx context.Context, eventKey string, channel Channel) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE notifications.deliveries
		SET status = 'sent', last_error = '', delivered_at = NOW(), updated_at = NOW()
		WHERE event_key = $1 AND channel = $2 AND status = 'processing'
	`, eventKey, channel)
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrDeliveryNotClaimed
	}
	return nil
}

func (repository *Repository) MarkFailed(ctx context.Context, eventKey string, channel Channel, lastError string) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE notifications.deliveries
		SET status = 'failed', last_error = $3, updated_at = NOW()
		WHERE event_key = $1 AND channel = $2 AND status = 'processing'
	`, eventKey, channel, truncateError(lastError))
	if err != nil {
		return fmt.Errorf("mark notification failed: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrDeliveryNotClaimed
	}
	return nil
}

func truncateError(message string) string {
	runes := []rune(strings.ToValidUTF8(strings.TrimSpace(message), "?"))
	if len(runes) > 500 {
		runes = runes[:500]
	}
	return string(runes)
}

func (repository *Repository) BotToken(ctx context.Context) (string, string, error) {
	var token, username string
	err := repository.pool.QueryRow(ctx, `SELECT token, username FROM notifications.telegram_bot`).Scan(&token, &username)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read bot token: %w", err)
	}
	return token, username, nil
}

func (repository *Repository) SaveBot(ctx context.Context, token, username string) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO notifications.telegram_bot (token, username) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET token = EXCLUDED.token, username = EXCLUDED.username, updated_at = NOW()
	`, token, username)
	if err != nil {
		return fmt.Errorf("save bot token: %w", err)
	}
	return nil
}

func (repository *Repository) DeleteBot(ctx context.Context) error {
	if _, err := repository.pool.Exec(ctx, `DELETE FROM notifications.telegram_bot`); err != nil {
		return fmt.Errorf("delete bot token: %w", err)
	}
	return nil
}

func (repository *Repository) Recipients(ctx context.Context) ([]TelegramChat, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT chat_id, title, username FROM notifications.telegram_recipients ORDER BY created_at, chat_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list telegram recipients: %w", err)
	}
	recipients, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (TelegramChat, error) {
		var chat TelegramChat
		err := row.Scan(&chat.ID, &chat.Title, &chat.Username)
		return chat, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan telegram recipients: %w", err)
	}
	return recipients, nil
}

func (repository *Repository) SaveRecipient(ctx context.Context, chat TelegramChat) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO notifications.telegram_recipients (chat_id, title, username) VALUES ($1, $2, $3)
		ON CONFLICT (chat_id) DO UPDATE SET title = EXCLUDED.title, username = EXCLUDED.username
	`, chat.ID, chat.Title, chat.Username)
	if err != nil {
		return fmt.Errorf("save telegram recipient: %w", err)
	}
	return nil
}

func (repository *Repository) DeleteRecipient(ctx context.Context, chatID int64) error {
	commandTag, err := repository.pool.Exec(ctx, `DELETE FROM notifications.telegram_recipients WHERE chat_id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("delete telegram recipient: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrRecipientNotFound
	}
	return nil
}
