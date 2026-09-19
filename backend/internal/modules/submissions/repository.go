package submissions

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

func (repository *Repository) Create(ctx context.Context, input CreateRecord) (Submission, error) {
	submission, err := scanSubmission(repository.pool.QueryRow(ctx, `
		INSERT INTO submissions.submissions (
			title, description, event_date, event_type_code,
			email, telegram_username, latitude, longitude, archive_id, slug, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'processing')
		RETURNING `+submissionColumns+`
	`, input.Title, input.Description, input.EventDate, input.EventType,
		input.Email, input.TelegramUsername, input.Latitude, input.Longitude, input.ArchiveID, input.Slug))
	if err != nil {
		return Submission{}, fmt.Errorf("create submission: %w", err)
	}
	return submission, nil
}

func (repository *Repository) Get(ctx context.Context, id int64) (Submission, error) {
	submission, err := scanSubmission(repository.pool.QueryRow(ctx, submissionSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrSubmissionNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("get submission: %w", err)
	}
	return submission, nil
}

func (repository *Repository) List(ctx context.Context, filter ListFilter) (SubmissionPage, error) {
	where, args := submissionWhere(filter.Status, filter.Search)
	var total int64
	if err := repository.pool.QueryRow(ctx, `SELECT COUNT(*) FROM submissions.submissions`+where, args...).Scan(&total); err != nil {
		return SubmissionPage{}, fmt.Errorf("count submissions: %w", err)
	}
	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := repository.pool.Query(ctx, submissionSelect+where+fmt.Sprintf(`
		ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, limitPosition, offsetPosition), args...)
	if err != nil {
		return SubmissionPage{}, fmt.Errorf("list submissions: %w", err)
	}
	defer rows.Close()
	items := make([]Submission, 0)
	for rows.Next() {
		submission, err := scanSubmission(rows)
		if err != nil {
			return SubmissionPage{}, fmt.Errorf("scan submission: %w", err)
		}
		items = append(items, submission)
	}
	if err := rows.Err(); err != nil {
		return SubmissionPage{}, fmt.Errorf("iterate submissions: %w", err)
	}
	return SubmissionPage{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (repository *Repository) Transition(ctx context.Context, transition Transition) (Submission, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Submission{}, fmt.Errorf("begin submission transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanSubmission(tx.QueryRow(ctx, submissionSelect+` WHERE id = $1 FOR UPDATE`, transition.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrSubmissionNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("lock submission: %w", err)
	}
	if current.Status != transition.From {
		return Submission{}, ErrInvalidTransition
	}

	updated, err := scanSubmission(tx.QueryRow(ctx, `
		UPDATE submissions.submissions
		SET status = $1,
			admin_comment = CASE WHEN $2 = '' THEN admin_comment ELSE $2 END,
			site_url = CASE WHEN $3 = '' THEN site_url ELSE $3 END,
			screenshot_url = CASE WHEN $4 = '' THEN screenshot_url ELSE $4 END,
			event_type_code = CASE WHEN $6 = '' THEN event_type_code ELSE $6 END,
			updated_at = NOW()
		WHERE id = $5
		RETURNING `+submissionColumns+`
	`, transition.To, transition.Comment, transition.SiteURL, transition.ScreenshotURL, transition.ID, transition.EventType))
	if err != nil {
		return Submission{}, fmt.Errorf("update submission status: %w", err)
	}
	if transition.Audit.Action != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO submissions.audit_log (
				submission_id, admin_login, admin_role, action, comment
			) VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		`, transition.ID, transition.Audit.AdminLogin, transition.Audit.AdminRole, transition.Audit.Action, transition.Audit.Comment)
		if err != nil {
			return Submission{}, fmt.Errorf("insert submission audit: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, fmt.Errorf("commit submission transition: %w", err)
	}
	return updated, nil
}

func (repository *Repository) Delete(ctx context.Context, id int64) error {
	if _, err := repository.pool.Exec(ctx, `DELETE FROM submissions.submissions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete submission: %w", err)
	}
	return nil
}

func (repository *Repository) AuditLog(ctx context.Context, filter AuditFilter) (AuditPage, error) {
	where, args := auditWhere(filter.Search)
	var total int64
	if err := repository.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM submissions.audit_log AS audit
		JOIN submissions.submissions AS submission ON submission.id = audit.submission_id
	`+where, args...).Scan(&total); err != nil {
		return AuditPage{}, fmt.Errorf("count submission audit: %w", err)
	}
	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := repository.pool.Query(ctx, `
		SELECT audit.id, audit.submission_id, submission.title, audit.admin_login,
			audit.admin_role, audit.action, COALESCE(audit.comment, ''), audit.created_at
		FROM submissions.audit_log AS audit
		JOIN submissions.submissions AS submission ON submission.id = audit.submission_id
	`+where+fmt.Sprintf(` ORDER BY audit.created_at DESC, audit.id DESC LIMIT $%d OFFSET $%d`, limitPosition, offsetPosition), args...)
	if err != nil {
		return AuditPage{}, fmt.Errorf("list submission audit: %w", err)
	}
	defer rows.Close()
	items := make([]AuditEntry, 0)
	for rows.Next() {
		var item AuditEntry
		if err := rows.Scan(&item.ID, &item.SubmissionID, &item.SubmissionTitle, &item.AdminLogin, &item.AdminRole, &item.Action, &item.Comment, &item.CreatedAt); err != nil {
			return AuditPage{}, fmt.Errorf("scan submission audit: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AuditPage{}, fmt.Errorf("iterate submission audit: %w", err)
	}
	return AuditPage{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

// submissionColumns follows the Scan order in scanSubmission.
const submissionColumns = `id, title, description, event_date, event_type_code, email,
	telegram_username, latitude, longitude, archive_id, COALESCE(site_url, ''),
	COALESCE(screenshot_url, ''), COALESCE(slug, ''), status::TEXT,
	COALESCE(admin_comment, ''), created_at, updated_at`

const submissionSelect = `SELECT ` + submissionColumns + ` FROM submissions.submissions`

type rowScanner interface {
	Scan(...any) error
}

func scanSubmission(row rowScanner) (Submission, error) {
	var submission Submission
	var status string
	err := row.Scan(
		&submission.ID, &submission.Title, &submission.Description, &submission.EventDate,
		&submission.EventType, &submission.Email, &submission.TelegramUsername,
		&submission.Latitude, &submission.Longitude, &submission.ArchiveID, &submission.SiteURL, &submission.ScreenshotURL,
		&submission.Slug, &status, &submission.AdminComment,
		&submission.CreatedAt, &submission.UpdatedAt,
	)
	submission.Status = Status(status)
	return submission, err
}

func submissionWhere(status Status, search string) (string, []any) {
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR id::TEXT ILIKE $%d)", len(args), len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func auditWhere(search string) (string, []any) {
	search = strings.TrimSpace(search)
	if search == "" {
		return "", nil
	}
	return `WHERE submission.title ILIKE $1 OR audit.action ILIKE $1 OR audit.admin_login ILIKE $1 OR audit.submission_id::TEXT ILIKE $1`, []any{"%" + search + "%"}
}
