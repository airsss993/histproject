package events

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

func (repository *Repository) Publish(ctx context.Context, snapshot Snapshot) (Event, error) {
	eventTypeID, err := eventTypeID(snapshot.EventType)
	if err != nil {
		return Event{}, err
	}
	event, err := scanEvent(repository.pool.QueryRow(ctx, `
		INSERT INTO events.events (
			submission_id, title, description, latitude, longitude, event_date,
			event_type_id, event_type_code, site_url, preview_image_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''))
		ON CONFLICT (submission_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			event_date = EXCLUDED.event_date,
			event_type_id = EXCLUDED.event_type_id,
			event_type_code = EXCLUDED.event_type_code,
			site_url = EXCLUDED.site_url,
			preview_image_url = EXCLUDED.preview_image_url,
			updated_at = NOW()
		RETURNING id, submission_id, title, description, latitude, longitude,
			event_date, event_type_code, site_url, preview_image_url, created_at, updated_at
	`, snapshot.SubmissionID, snapshot.Title, snapshot.Description, snapshot.Latitude,
		snapshot.Longitude, snapshot.EventDate, eventTypeID, snapshot.EventType,
		snapshot.SiteURL, snapshot.PreviewImageURL))
	if err != nil {
		return Event{}, fmt.Errorf("publish event: %w", err)
	}
	return event, nil
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

func eventTypeID(eventType string) (int, error) {
	switch eventType {
	case "political":
		return 1, nil
	case "military":
		return 2, nil
	}
	if !strings.HasPrefix(eventType, "legacy_") {
		return 0, fmt.Errorf("unsupported event type")
	}
	eventTypeID, err := strconv.Atoi(strings.TrimPrefix(eventType, "legacy_"))
	if err != nil || eventTypeID <= 0 {
		return 0, fmt.Errorf("unsupported event type")
	}
	return eventTypeID, nil
}
