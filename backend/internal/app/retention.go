package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Сроки хранения из политики обработки персональных данных (/privacy, раздел 6).
const (
	rejectedRetention = "90 days"
	auditRetention    = "3 years"
	viewsRetention    = "12 months"
	actionsRetention  = "12 months"
	retentionBatch    = 100
)

// SiteRemover удаляет архив заявки и файлы её сайта. Реализует storage.MinIO.
type SiteRemover interface {
	DeleteArchive(ctx context.Context, archiveID string) error
	DeleteSite(ctx context.Context, slug string) error
}

// Retention удаляет данные, у которых вышел срок хранения.
type Retention struct {
	pool   *pgxpool.Pool
	files  SiteRemover
	logger *slog.Logger
}

func NewRetention(pool *pgxpool.Pool, files SiteRemover, logger *slog.Logger) *Retention {
	return &Retention{pool: pool, files: files, logger: logger}
}

// Loop чистит сразу и затем раз в сутки, пока жив ctx.
func (retention *Retention) Loop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		if err := retention.Run(ctx); err != nil && ctx.Err() == nil {
			retention.logger.ErrorContext(ctx, "retention cleanup failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (retention *Retention) Run(ctx context.Context) error {
	removed, err := retention.rejected(ctx)
	if err != nil {
		return err
	}
	var audit, views, actions int64
	err = retention.inRetention(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM submissions.audit_log WHERE created_at < NOW() - $1::interval`, auditRetention)
		if err != nil {
			return fmt.Errorf("delete old audit log: %w", err)
		}
		audit = tag.RowsAffected()
		tag, err = tx.Exec(ctx, `DELETE FROM events.views WHERE started_at < NOW() - $1::interval`, viewsRetention)
		if err != nil {
			return fmt.Errorf("delete old views: %w", err)
		}
		views = tag.RowsAffected()
		tag, err = tx.Exec(ctx, `DELETE FROM metrics.actions WHERE created_at < NOW() - $1::interval`, actionsRetention)
		if err != nil {
			return fmt.Errorf("delete old actions: %w", err)
		}
		actions = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if removed+audit+views+actions > 0 {
		retention.logger.InfoContext(ctx, "retention cleanup", "rejected_submissions", removed, "audit_rows", audit, "views", views, "actions", actions)
	}
	return nil
}

// rejected удаляет отклонённые заявки старше срока: сначала файлы, потом строку. Упал на файлах -
// строка остаётся, и заявка попадёт в следующий проход.
func (retention *Retention) rejected(ctx context.Context) (int64, error) {
	var removed int64
	for {
		rows, err := retention.pool.Query(ctx, `
			SELECT id, archive_id, slug FROM submissions.submissions
			WHERE status = 'rejected' AND updated_at < NOW() - $1::interval
			ORDER BY id LIMIT $2`, rejectedRetention, retentionBatch)
		if err != nil {
			return removed, fmt.Errorf("select expired submissions: %w", err)
		}
		type expired struct {
			id              int64
			archiveID, slug string
		}
		batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (expired, error) {
			var item expired
			return item, row.Scan(&item.id, &item.archiveID, &item.slug)
		})
		if err != nil {
			return removed, fmt.Errorf("scan expired submissions: %w", err)
		}
		var failed error
		for _, item := range batch {
			if err := errors.Join(retention.files.DeleteArchive(ctx, item.archiveID), retention.files.DeleteSite(ctx, item.slug)); err != nil {
				failed = errors.Join(failed, fmt.Errorf("submission %d files: %w", item.id, err))
				continue
			}
			err := retention.inRetention(ctx, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `DELETE FROM submissions.submissions WHERE id = $1 AND status = 'rejected'`, item.id)
				return err
			})
			if err != nil {
				return removed, fmt.Errorf("delete submission %d: %w", item.id, err)
			}
			removed++
		}
		if failed != nil || len(batch) < retentionBatch {
			return removed, failed
		}
	}
}

// inRetention открывает транзакцию, в которой триггер журнала модерации пропускает удаление.
func (retention *Retention) inRetention(ctx context.Context, work func(pgx.Tx) error) error {
	tx, err := retention.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin retention: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL histproject.retention = 'on'`); err != nil {
		return fmt.Errorf("enable retention: %w", err)
	}
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
