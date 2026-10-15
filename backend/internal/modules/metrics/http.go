// Package metrics собирает действия посетителей для метрик и отдаёт отчёт панели модерации.
package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

// ReadThroughPercent - с какой прокрутки сайт считается дочитанным.
const ReadThroughPercent = 90

// MaxDays - самый длинный период отчёта: просмотры и действия хранятся 12 месяцев.
const MaxDays = 365

var visitorIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Action struct {
	Name      string
	EventID   int64
	Step      *int
	VisitorID string
}

type Report struct {
	Days             int          `json:"days"`
	Submissions      int64        `json:"submissions"`
	Approved         int64        `json:"approved"`
	Rejected         int64        `json:"rejected"`
	Published        int64        `json:"published"`
	EventsTotal      int64        `json:"eventsTotal"`
	Views            int64        `json:"views"`
	Visitors         int64        `json:"visitors"`
	AvgActiveSeconds float64      `json:"avgActiveSeconds"`
	Stars            int64        `json:"stars"`
	StarsTotal       int64        `json:"starsTotal"`
	CardOpens        int64        `json:"cardOpens"`
	CardClicks       int64        `json:"cardClicks"`
	AvgScroll        float64      `json:"avgScroll"`
	ReadThrough      int64        `json:"readThrough"`
	ScrolledViews    int64        `json:"scrolledViews"`
	Funnel           []FunnelStep `json:"funnel"`
	Daily            []Day        `json:"daily"`
	Events           []EventRow   `json:"events"`
}

type FunnelStep struct {
	Step     int   `json:"step"`
	Visitors int64 `json:"visitors"`
}

type Day struct {
	Date     string `json:"date"`
	Views    int64  `json:"views"`
	Visitors int64  `json:"visitors"`
}

type EventRow struct {
	ID               int64   `json:"id"`
	Title            string  `json:"title"`
	Views            int64   `json:"views"`
	Visitors         int64   `json:"visitors"`
	AvgActiveSeconds float64 `json:"avgActiveSeconds"`
	AvgScroll        float64 `json:"avgScroll"`
	ReadThrough      int64   `json:"readThrough"`
	ScrolledViews    int64   `json:"scrolledViews"`
	Stars            int64   `json:"stars"`
	CardOpens        int64   `json:"cardOpens"`
	CardClicks       int64   `json:"cardClicks"`
}

type Store interface {
	Record(context.Context, Action) error
	Report(ctx context.Context, days int) (Report, error)
}

type Handler struct {
	store  Store
	logger *slog.Logger
}

func NewHandler(store Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

func (handler *Handler) Register(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	// POST, а не PUT: переход на сайт уходит через sendBeacon, он умеет только POST.
	mux.HandleFunc("POST /api/v1/metrics/actions", handler.record)
	mux.Handle("GET /api/v1/admin/metrics", protect(http.HandlerFunc(handler.report)))
}

func (handler *Handler) record(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Action    string `json:"action"`
		EventID   int64  `json:"eventId"`
		Step      *int   `json:"step"`
		VisitorID string `json:"visitorId"`
	}
	if err := httpserver.DecodeJSON(w, r, &request); err != nil {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON", nil)
		return
	}
	action := Action{Name: request.Action, EventID: request.EventID, Step: request.Step, VisitorID: request.VisitorID}
	if fields := validate(action); len(fields) > 0 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", fields)
		return
	}
	if err := handler.store.Record(r.Context(), action); err != nil {
		handler.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validate(action Action) map[string]string {
	fields := make(map[string]string)
	switch action.Name {
	case "card_open", "card_click":
		if action.EventID <= 0 {
			fields["eventId"] = "Ожидается положительный ID"
		}
		if action.Step != nil {
			fields["step"] = "Только для form_step"
		}
	case "form_step":
		if action.Step == nil || *action.Step < 0 || *action.Step > 20 {
			fields["step"] = "От 0 до 20"
		}
		if action.EventID != 0 {
			fields["eventId"] = "Только для card_open и card_click"
		}
	default:
		fields["action"] = "card_open, card_click или form_step"
	}
	if !visitorIDPattern.MatchString(action.VisitorID) {
		fields["visitorId"] = "Ожидается UUID"
	}
	return fields
}

func (handler *Handler) report(w http.ResponseWriter, r *http.Request) {
	days := 30
	if value := r.URL.Query().Get("days"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > MaxDays {
			_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Проверьте поля", map[string]string{"days": "От 1 до 365"})
			return
		}
		days = parsed
	}
	report, err := handler.store.Report(r.Context(), days)
	if err != nil {
		handler.internal(w, r, err)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, report)
}

func (handler *Handler) internal(w http.ResponseWriter, r *http.Request, err error) {
	handler.logger.ErrorContext(r.Context(), "metrics request failed", "request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
	_ = httpserver.WriteError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка", nil)
}
