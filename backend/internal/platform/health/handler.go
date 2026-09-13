package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Checker interface {
	Name() string
	Ping(context.Context) error
}

type Handler struct {
	timeout  time.Duration
	checkers []Checker
}

func NewHandler(timeout time.Duration, checkers ...Checker) *Handler {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Handler{timeout: timeout, checkers: checkers}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	status := http.StatusOK
	if len(h.checkers) == 0 {
		status = http.StatusServiceUnavailable
	}
	checks := make(map[string]string, len(h.checkers))
	for _, checker := range h.checkers {
		if err := checker.Ping(ctx); err != nil {
			checks[checker.Name()] = "failed"
			status = http.StatusServiceUnavailable
			continue
		}
		checks[checker.Name()] = "ok"
	}

	readiness := "ready"
	if status != http.StatusOK {
		readiness = "not_ready"
	}
	writeJSON(w, status, map[string]any{
		"status": readiness,
		"checks": checks,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
