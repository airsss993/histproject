package events

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Publish(ctx context.Context, snapshot Snapshot) (Event, error) {
	event, err := scanEvent(repository.pool.QueryRow(ctx, `
		INSERT INTO events.events (
			submission_id, title, description, latitude, longitude, event_date,
			event_type_code, site_url, preview_image_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''))
		ON CONFLICT (submission_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			event_date = EXCLUDED.event_date,
			event_type_code = EXCLUDED.event_type_code,
			site_url = EXCLUDED.site_url,
			preview_image_url = EXCLUDED.preview_image_url,
			updated_at = NOW()
		RETURNING `+eventColumns+`
	`, snapshot.SubmissionID, snapshot.Title, snapshot.Description, snapshot.Latitude,
		snapshot.Longitude, snapshot.EventDate, snapshot.EventType,
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
	viewerPosition := len(args) + 1
	args = append(args, filter.ViewerID)
	rows, err := repository.pool.Query(ctx, `
		SELECT `+eventColumns+`, `+fmt.Sprintf(starColumns, viewerPosition)+`
		FROM events.events`+where+fmt.Sprintf(`
		ORDER BY event_date ASC, id ASC
		LIMIT $%d OFFSET $%d`, limitPosition, offsetPosition), args...)
	if err != nil {
		return Page{}, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	items := make([]Event, 0)
	for rows.Next() {
		var stars int64
		var starred bool
		event, err := scanEvent(rows, &stars, &starred)
		if err != nil {
			return Page{}, fmt.Errorf("scan event: %w", err)
		}
		event.Stars, event.Starred = stars, starred
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate events: %w", err)
	}
	return Page{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (repository *Repository) Get(ctx context.Context, id, viewerID int64) (Event, error) {
	var stars int64
	var starred bool
	event, err := scanEvent(repository.pool.QueryRow(ctx, `
		SELECT `+eventColumns+`, `+fmt.Sprintf(starColumns, 2)+`
		FROM events.events
		WHERE id = $1
	`, id, viewerID), &stars, &starred)
	event.Stars, event.Starred = stars, starred
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

const eventColumns = `id, submission_id, title, description, latitude, longitude,
	event_date, event_type_code, site_url, preview_image_url, created_at, updated_at`

// starColumns - число звёзд и звезда смотрящего; %d - номер параметра с user_id (0 у гостя).
// Обе выборки идут по первичному ключу events.stars (event_id, user_id).
const starColumns = `(SELECT COUNT(*) FROM events.stars star WHERE star.event_id = events.id),
	EXISTS (SELECT 1 FROM events.stars star WHERE star.event_id = events.id AND star.user_id = $%d)`

// scanEvent читает eventColumns, extra - колонки после них.
func scanEvent(row rowScanner, extra ...any) (Event, error) {
	var event Event
	err := row.Scan(append([]any{
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
	}, extra...)...)
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

// SetStar ставит (on) или снимает звезду пользователя и отдаёт состояние после этого.
// Повтор ничего не меняет: вставку глушит первичный ключ, удаление отсутствующей строки - пустое.
func (repository *Repository) SetStar(ctx context.Context, eventID, userID int64, on bool) (StarState, error) {
	query := `DELETE FROM events.stars WHERE event_id = $1 AND user_id = $2`
	if on {
		query = `INSERT INTO events.stars (event_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	if _, err := repository.pool.Exec(ctx, query, eventID, userID); err != nil {
		if isForeignKeyViolation(err) {
			return StarState{}, ErrEventNotFound
		}
		return StarState{}, fmt.Errorf("set star: %w", err)
	}
	var state StarState
	err := repository.pool.QueryRow(ctx, `SELECT `+fmt.Sprintf(starColumns, 2)+` FROM events.events WHERE id = $1`,
		eventID, userID).Scan(&state.Stars, &state.Starred)
	if errors.Is(err, pgx.ErrNoRows) {
		return StarState{}, ErrEventNotFound
	}
	if err != nil {
		return StarState{}, fmt.Errorf("read stars: %w", err)
	}
	return state, nil
}

// StartView записывает начало просмотра. userID 0 - гость.
func (repository *Repository) StartView(ctx context.Context, eventID, userID int64, visitorID string) (int64, error) {
	var id int64
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO events.views (event_id, user_id, visitor_id)
		VALUES ($1, NULLIF($2, 0::bigint), $3)
		RETURNING id
	`, eventID, userID, visitorID).Scan(&id)
	if isForeignKeyViolation(err) {
		return 0, ErrEventNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("start view: %w", err)
	}
	return id, nil
}

// TouchView дописывает активное время просмотра. Время только растёт и не больше прошедшего
// с начала просмотра (плюс запас на округление): так повтор или подделанное число его не раздуют.
func (repository *Repository) TouchView(ctx context.Context, eventID, viewID int64, visitorID string, seconds int) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE events.views
		SET active_seconds = GREATEST(active_seconds,
			LEAST($4, EXTRACT(EPOCH FROM NOW() - started_at)::int + 5))
		WHERE id = $1 AND event_id = $2 AND visitor_id = $3
	`, viewID, eventID, visitorID, seconds)
	if err != nil {
		return fmt.Errorf("touch view: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return ErrViewNotFound
	}
	return nil
}

// Summaries - звёзды и просмотры событий по заявкам. Уникальный посетитель - аккаунт, а у гостя - браузер.
func (repository *Repository) Summaries(ctx context.Context, submissionIDs []int64, viewerID int64) ([]Summary, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT events.id, events.submission_id, `+fmt.Sprintf(starColumns, 2)+`,
			COUNT(view.id), COUNT(DISTINCT COALESCE(view.user_id::text, view.visitor_id::text)),
			COALESCE(SUM(view.active_seconds), 0)
		FROM events.events
		LEFT JOIN events.views view ON view.event_id = events.id
		WHERE events.submission_id = ANY($1)
		GROUP BY events.id
	`, submissionIDs, viewerID)
	if err != nil {
		return nil, fmt.Errorf("event summaries: %w", err)
	}
	defer rows.Close()
	items := make([]Summary, 0)
	for rows.Next() {
		var item Summary
		if err := rows.Scan(&item.EventID, &item.SubmissionID, &item.Stars, &item.Starred,
			&item.Views, &item.Visitors, &item.ActiveSeconds); err != nil {
			return nil, fmt.Errorf("scan event summary: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event summaries: %w", err)
	}
	return items, nil
}

func isForeignKeyViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23503"
}
