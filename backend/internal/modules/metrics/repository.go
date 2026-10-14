package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Record(ctx context.Context, action Action) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO metrics.actions (action, event_id, step, visitor_id)
		VALUES ($1, NULLIF($2, 0::bigint), $3, $4)
	`, action.Name, action.EventID, action.Step, action.VisitorID)
	if err != nil {
		return fmt.Errorf("record action: %w", err)
	}
	return nil
}

// Report читает таблицы submissions, events и metrics: отчёт только читает, поэтому границу модулей
// не нарушает. Уникальный посетитель - аккаунт, а у гостя - браузер, как в счётчике просмотров.
// Сайты, опубликованные до скрипта прокрутки, присылают 0, поэтому прокрутка считается по scroll_percent > 0.
func (repository *Repository) Report(ctx context.Context, days int) (Report, error) {
	report := Report{Days: days, Funnel: []FunnelStep{}, Daily: []Day{}, Events: []EventRow{}}
	err := repository.pool.QueryRow(ctx, `
		WITH period AS (SELECT NOW() - make_interval(days => $1) AS since),
		submitted AS (SELECT status FROM submissions.submissions, period WHERE created_at >= period.since),
		viewed AS (SELECT * FROM events.views, period WHERE started_at >= period.since),
		acted AS (SELECT * FROM metrics.actions, period WHERE created_at >= period.since)
		SELECT
			(SELECT COUNT(*) FROM submitted),
			(SELECT COUNT(*) FROM submitted WHERE status = 'published'),
			(SELECT COUNT(*) FROM submitted WHERE status = 'rejected'),
			(SELECT COUNT(*) FROM events.events, period WHERE created_at >= period.since),
			(SELECT COUNT(*) FROM events.events),
			(SELECT COUNT(*) FROM viewed),
			(SELECT COUNT(DISTINCT COALESCE(user_id::text, visitor_id::text)) FROM viewed),
			(SELECT COALESCE(AVG(active_seconds), 0) FROM viewed),
			(SELECT COUNT(*) FROM events.stars, period WHERE created_at >= period.since),
			(SELECT COUNT(*) FROM events.stars),
			(SELECT COUNT(DISTINCT (visitor_id, event_id)) FROM acted WHERE action = 'card_open'),
			(SELECT COUNT(DISTINCT (visitor_id, event_id)) FROM acted WHERE action = 'card_click'),
			(SELECT COALESCE(AVG(scroll_percent), 0) FROM viewed WHERE scroll_percent > 0),
			(SELECT COUNT(*) FILTER (WHERE scroll_percent >= $2) FROM viewed),
			(SELECT COUNT(*) FROM viewed WHERE scroll_percent > 0)
	`, days, ReadThroughPercent).Scan(
		&report.Submissions, &report.Approved, &report.Rejected, &report.Published, &report.EventsTotal,
		&report.Views, &report.Visitors, &report.AvgActiveSeconds, &report.Stars, &report.StarsTotal,
		&report.CardOpens, &report.CardClicks, &report.AvgScroll, &report.ReadThrough, &report.ScrolledViews,
	)
	if err != nil {
		return Report{}, fmt.Errorf("metrics totals: %w", err)
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT step, COUNT(DISTINCT visitor_id)
		FROM metrics.actions
		WHERE action = 'form_step' AND created_at >= NOW() - make_interval(days => $1)
		GROUP BY step ORDER BY step
	`, days)
	if err != nil {
		return Report{}, fmt.Errorf("metrics funnel: %w", err)
	}
	for rows.Next() {
		var step FunnelStep
		if err := rows.Scan(&step.Step, &step.Visitors); err != nil {
			rows.Close()
			return Report{}, fmt.Errorf("scan funnel step: %w", err)
		}
		report.Funnel = append(report.Funnel, step)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("iterate funnel: %w", err)
	}

	// День по Москве: аудитория и модераторы живут в ней, а сервер в UTC.
	rows, err = repository.pool.Query(ctx, `
		SELECT (started_at AT TIME ZONE 'Europe/Moscow')::date AS day, COUNT(*),
			COUNT(DISTINCT COALESCE(user_id::text, visitor_id::text))
		FROM events.views
		WHERE started_at >= NOW() - make_interval(days => $1)
		GROUP BY day ORDER BY day
	`, days)
	if err != nil {
		return Report{}, fmt.Errorf("metrics daily: %w", err)
	}
	for rows.Next() {
		var day Day
		var date time.Time
		if err := rows.Scan(&date, &day.Views, &day.Visitors); err != nil {
			rows.Close()
			return Report{}, fmt.Errorf("scan day: %w", err)
		}
		day.Date = date.Format(time.DateOnly)
		report.Daily = append(report.Daily, day)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("iterate daily: %w", err)
	}

	rows, err = repository.pool.Query(ctx, `
		WITH since AS (SELECT NOW() - make_interval(days => $1) AS at)
		SELECT event.id, event.title,
			COALESCE(view.views, 0), COALESCE(view.visitors, 0), COALESCE(view.avg_active, 0),
			COALESCE(view.avg_scroll, 0), COALESCE(view.read_through, 0), COALESCE(view.scrolled, 0),
			(SELECT COUNT(*) FROM events.stars star WHERE star.event_id = event.id),
			COALESCE(card.opens, 0), COALESCE(card.clicks, 0)
		FROM events.events event
		LEFT JOIN (
			SELECT event_id, COUNT(*) AS views, COUNT(DISTINCT COALESCE(user_id::text, visitor_id::text)) AS visitors,
				AVG(active_seconds) AS avg_active,
				AVG(scroll_percent) FILTER (WHERE scroll_percent > 0) AS avg_scroll,
				COUNT(*) FILTER (WHERE scroll_percent >= $2) AS read_through,
				COUNT(*) FILTER (WHERE scroll_percent > 0) AS scrolled
			FROM events.views, since WHERE started_at >= since.at GROUP BY event_id
		) view ON view.event_id = event.id
		LEFT JOIN (
			SELECT event_id,
				COUNT(DISTINCT visitor_id) FILTER (WHERE action = 'card_open') AS opens,
				COUNT(DISTINCT visitor_id) FILTER (WHERE action = 'card_click') AS clicks
			FROM metrics.actions, since WHERE created_at >= since.at AND event_id IS NOT NULL GROUP BY event_id
		) card ON card.event_id = event.id
		ORDER BY COALESCE(view.visitors, 0) DESC, event.id DESC
	`, days, ReadThroughPercent)
	if err != nil {
		return Report{}, fmt.Errorf("metrics by event: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row EventRow
		if err := rows.Scan(&row.ID, &row.Title, &row.Views, &row.Visitors, &row.AvgActiveSeconds,
			&row.AvgScroll, &row.ReadThrough, &row.ScrolledViews, &row.Stars, &row.CardOpens, &row.CardClicks); err != nil {
			return Report{}, fmt.Errorf("scan event row: %w", err)
		}
		report.Events = append(report.Events, row)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("iterate event rows: %w", err)
	}
	return report, nil
}
