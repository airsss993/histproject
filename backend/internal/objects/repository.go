package objects

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository — доступ к данным объектов.
type Repository interface {
	GetByID(ctx context.Context, id int) (*SingleObjectInfo, error)
	List(ctx context.Context, eventTypeIDs []int, dateFrom, dateTo string) ([]ObjectInfo, error)
	CreateObject(ctx context.Context, data ObjectData) error
	DeleteObject(ctx context.Context, requestID int) error
	UpdateCoordinates(ctx context.Context, requestID int, latitude, longitude float64) error
	IsPublished(ctx context.Context, slug string) (bool, error)
}

type repository struct {
	db *sqlx.DB
}

// NewRepository создаёт реализацию Repository для PostgreSQL.
func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetByID(ctx context.Context, id int) (*SingleObjectInfo, error) {
	var obj SingleObjectInfo
	query := `
		SELECT id, title, description, event_date, preview_image_url, site_url
		FROM objects WHERE id = $1`
	err := r.db.GetContext(ctx, &obj, query, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации об объекте %v с БД: %w", id, err)
	}
	return &obj, nil
}

func (r *repository) List(ctx context.Context, eventTypeIDs []int, dateFrom, dateTo string) ([]ObjectInfo, error) {
	query := `
		SELECT id, title, description, latitude, longitude, event_date, event_type_id, preview_image_url
		FROM objects`
	var args []interface{}
	argPos := 1

	if len(eventTypeIDs) > 0 {
		query += fmt.Sprintf(" WHERE event_type_id = ANY($%d)", argPos)
		args = append(args, pq.Array(eventTypeIDs))
		argPos++
	}
	if dateFrom != "" {
		if argPos > 1 {
			query += fmt.Sprintf(" AND event_date >= $%d", argPos)
		} else {
			query += fmt.Sprintf(" WHERE event_date >= $%d", argPos)
		}
		args = append(args, dateFrom)
		argPos++
	}
	if dateTo != "" {
		if argPos > 1 {
			query += fmt.Sprintf(" AND event_date <= $%d", argPos)
		} else {
			query += fmt.Sprintf(" WHERE event_date <= $%d", argPos)
		}
		args = append(args, dateTo)
		argPos++
	}

	var list []ObjectInfo
	err := r.db.SelectContext(ctx, &list, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения объектов с БД: %w", err)
	}
	if list == nil {
		return []ObjectInfo{}, nil
	}
	return list, nil
}

func (r *repository) CreateObject(ctx context.Context, data ObjectData) error {
	query := `
		INSERT INTO objects
			(request_id, title, description, latitude, longitude, event_date, event_type_id, site_url, preview_image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.ExecContext(ctx, query,
		data.RequestID, data.Title, data.Description, data.Latitude, data.Longitude,
		data.EventDate, data.EventTypeID, data.SiteURL, data.PreviewImageURL,
	)
	if err != nil {
		return fmt.Errorf("ошибка создания объекта на карте: %w", err)
	}
	return nil
}

func (r *repository) DeleteObject(ctx context.Context, requestID int) error {
	// Проверяем, что заявка находится в статусе 'Опубликована'
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM requests WHERE id = $1`, requestID).Scan(&status)
	if err != nil {
		return fmt.Errorf("заявка не найдена")
	}
	if status != "Опубликована" {
		return fmt.Errorf("удаление доступно только для опубликованных заявок")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Удаляем метку с карты
	res, err := tx.ExecContext(ctx, `DELETE FROM objects WHERE request_id = $1`, requestID)
	if err != nil {
		return fmt.Errorf("ошибка удаления объекта: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("объект для заявки не найден")
	}

	// Удаляем саму заявку (аудит-лог удалится каскадно)
	if _, err := tx.ExecContext(ctx, `DELETE FROM requests WHERE id = $1`, requestID); err != nil {
		return fmt.Errorf("ошибка удаления заявки: %w", err)
	}

	return tx.Commit()
}

func (r *repository) IsPublished(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1 FROM objects o
			JOIN requests r ON r.id = o.request_id
			WHERE r.slug = $1
		)`, slug)
	return exists, err
}

func (r *repository) UpdateCoordinates(ctx context.Context, requestID int, latitude, longitude float64) error {
	// Проверяем, что заявка находится в статусе 'Опубликована'
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM requests WHERE id = $1`, requestID).Scan(&status)
	if err != nil {
		return fmt.Errorf("заявка не найдена")
	}
	if status != "Опубликована" {
		return fmt.Errorf("редактирование координат доступно только для опубликованных заявок")
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE objects SET latitude = $1, longitude = $2 WHERE request_id = $3`,
		latitude, longitude, requestID,
	)
	if err != nil {
		return fmt.Errorf("ошибка обновления координат: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("объект для заявки не найден")
	}
	return nil
}

