package requests

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// RequestRepository — доступ к заявкам (создание и обновление статуса).
type RequestRepository interface {
	Create(ctx context.Context, data RequestData) (int, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	UpdateStatus(ctx context.Context, id int, status, comment, siteURL, screenshotURL string) error
	List(ctx context.Context, status, q string, page, limit int) (*RequestListResult, error)
	GetByID(ctx context.Context, id int) (*RequestDetail, error)
	LogHistory(ctx context.Context, requestID int, adminLogin, adminRole, action, comment string) error
	GetHistory(ctx context.Context, q string, page, limit int) (*HistoryResult, error)
}

// RequestData — данные заявки для сохранения в БД.
type RequestData struct {
	Title            string
	Description      string
	Email            string
	TelegramUsername string
	ArchiveId        string
	SiteURL          string
	ScreenshotURL    string
	EventDate        string
	EventTypeId      int
	Slug             string
}

type repository struct {
	db *sqlx.DB
}

// NewRepository создаёт реализацию RequestRepository для PostgreSQL.
func NewRepository(db *sqlx.DB) RequestRepository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, data RequestData) (int, error) {
	var id int
	query := `
		INSERT INTO requests
			(title, description, email, telegram_username, archive_id, site_url, screenshot_url, event_date, event_type_id, slug)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`
	err := r.db.QueryRowContext(ctx, query,
		data.Title, data.Description, data.Email, data.TelegramUsername, data.ArchiveId,
		data.SiteURL, data.ScreenshotURL, data.EventDate, data.EventTypeId, data.Slug,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ошибка вставки новой записи о заявке в БД: %w", err)
	}
	return id, nil
}

func (r *repository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM requests WHERE slug = $1)`, slug)
	return exists, err
}

func (r *repository) UpdateStatus(ctx context.Context, id int, status, comment, siteURL, screenshotURL string) error {
	query := `UPDATE requests SET status = $1, admin_comment = $2, site_url = $3, screenshot_url = $4, updated_at = NOW() WHERE id = $5`
	_, err := r.db.ExecContext(ctx, query, status, comment, siteURL, screenshotURL, id)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении статуса заявки: %w", err)
	}
	return nil
}

func (r *repository) List(ctx context.Context, status, q string, page, limit int) (*RequestListResult, error) {
	args := []interface{}{"В обработке"}
	clauses := []string{"status != $1"}

	if status != "" {
		args = append(args, status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		clauses = append(clauses, fmt.Sprintf("(title ILIKE $%d OR id::text ILIKE $%d)", n, n))
	}

	where := " WHERE " + strings.Join(clauses, " AND ")

	// Считаем общее количество записей с тем же фильтром
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM requests`+where, args...); err != nil {
		return nil, fmt.Errorf("ошибка подсчёта заявок: %w", err)
	}

	// Основной запрос с пагинацией
	offset := (page - 1) * limit
	query := fmt.Sprintf(
		`SELECT id, title, status, created_at FROM requests%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	args = append(args, limit, offset)

	var list []RequestListItem
	if err := r.db.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, fmt.Errorf("ошибка получения списка заявок: %w", err)
	}
	if list == nil {
		list = []RequestListItem{}
	}
	return &RequestListResult{Items: list, Total: total}, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*RequestDetail, error) {
	var req RequestDetail
	query := `
		SELECT id, title, description, event_date::text, event_type_id, email, telegram_username,
		       status, admin_comment, slug, site_url, screenshot_url, created_at
		FROM requests WHERE id = $1`
	err := r.db.GetContext(ctx, &req, query, id)
	if err != nil {
		return nil, fmt.Errorf("заявка не найдена: %w", err)
	}
	return &req, nil
}

func (r *repository) LogHistory(ctx context.Context, requestID int, adminLogin, adminRole, action, comment string) error {
	query := `INSERT INTO request_audit_log (request_id, admin_login, admin_role, action, comment) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.ExecContext(ctx, query, requestID, adminLogin, adminRole, action, comment)
	if err != nil {
		return fmt.Errorf("ошибка записи в аудит-лог: %w", err)
	}
	return nil
}

func (r *repository) GetHistory(ctx context.Context, q string, page, limit int) (*HistoryResult, error) {
	var args []interface{}
	where := ""
	if q != "" {
		args = append(args, "%"+q+"%")
		where = `WHERE (r.title ILIKE $1 OR l.action ILIKE $1 OR l.admin_login ILIKE $1 OR l.request_id::text ILIKE $1) `
	}

	// Считаем общее количество записей с тем же фильтром
	var total int
	countQuery := `SELECT COUNT(*) FROM request_audit_log l JOIN requests r ON r.id = l.request_id ` + where
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, fmt.Errorf("ошибка подсчёта истории: %w", err)
	}

	// Основной запрос с пагинацией
	offset := (page - 1) * limit
	query := fmt.Sprintf(`
		SELECT l.id, l.request_id, r.title AS request_title, l.action, l.comment, l.admin_login, l.admin_role, l.created_at
		FROM request_audit_log l
		JOIN requests r ON r.id = l.request_id
		%sORDER BY l.created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	args = append(args, limit, offset)

	var items []RequestHistoryItem
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, fmt.Errorf("ошибка получения истории действий: %w", err)
	}
	if items == nil {
		items = []RequestHistoryItem{}
	}
	return &HistoryResult{Items: items, Total: total}, nil
}
