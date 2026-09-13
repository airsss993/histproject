package events

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) List(ctx context.Context, filter Filter) (Page, error) {
	where, args := eventWhere(filter)
	var total int64
	if err := repository.pool.QueryRow(ctx, `SELECT COUNT(*) FROM events.events`+where, args...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count events: %w", err)
	}

	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := repository.pool.Query(ctx, `
		SELECT id, submission_id, title, description, latitude, longitude,
			event_date, event_type_code, site_url, preview_image_url, created_at, updated_at
		FROM events.events`+where+fmt.Sprintf(`
		ORDER BY event_date ASC, id ASC
		LIMIT $%d OFFSET $%d`, limitPosition, offsetPosition), args...)
	if err != nil {
		return Page{}, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	items := make([]Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return Page{}, fmt.Errorf("scan event: %w", err)
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate events: %w", err)
	}
	return Page{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (repository *Repository) Get(ctx context.Context, id int64) (Event, error) {
	event, err := scanEvent(repository.pool.QueryRow(ctx, `
		SELECT id, submission_id, title, description, latitude, longitude,
			event_date, event_type_code, site_url, preview_image_url, created_at, updated_at
		FROM events.events
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("get event: %w", err)
	}
	return event, nil
}

func (repository *Repository) UpdateCoordinates(ctx context.Context, id int64, latitude, longitude float64) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE events.events
		SET latitude = $1, longitude = $2, updated_at = NOW()
		WHERE id = $3
	`, latitude, longitude, id)
	if err != nil {
		return fmt.Errorf("update event coordinates: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrEventNotFound
	}
	return nil
}

func (repository *Repository) Delete(ctx context.Context, id int64) error {
	commandTag, err := repository.pool.Exec(ctx, `DELETE FROM events.events WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrEventNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanEvent(row rowScanner) (Event, error) {
	var event Event
	err := row.Scan(
		&event.ID,
		&event.SubmissionID,
		&event.Title,
		&event.Description,
		&event.Latitude,
		&event.Longitude,
		&event.EventDate,
		&event.EventType,
		&event.SiteURL,
		&event.PreviewImageURL,
		&event.CreatedAt,
		&event.UpdatedAt,
	)
	return event, err
}

func eventWhere(filter Filter) (string, []any) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if len(filter.EventTypes) > 0 {
		args = append(args, filter.EventTypes)
		conditions = append(conditions, fmt.Sprintf("event_type_code = ANY($%d)", len(args)))
	}
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		conditions = append(conditions, fmt.Sprintf("event_date >= $%d", len(args)))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		conditions = append(conditions, fmt.Sprintf("event_date <= $%d", len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
