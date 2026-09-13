package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDeliveryNotClaimed = errors.New("notification delivery is not claimed")

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
		return false, nil
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
