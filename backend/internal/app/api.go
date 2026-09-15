package app

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/health"
	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

// apiRateLimit is per client IP; schools behind one NAT share it, so it stays well above a single map session.
const (
	apiRateLimit       = 300
	apiRateLimitWindow = time.Minute
)

func NewAPI(cfg config.Config, logger *slog.Logger, modules Modules, checkers ...health.Checker) http.Handler {
	mux := http.NewServeMux()
	health.NewHandler(2*time.Second, checkers...).Register(mux)
	modules.Register(mux)
	routes := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		writeUnmatchedRoute(w, r, mux)
	})

	middleware := []httpserver.Middleware{
		httpserver.RequestID,
		httpserver.AccessLog(logger),
		httpserver.Recover(logger),
		httpserver.SecurityHeaders,
		httpserver.CORS(cfg.CORSOrigins),
	}
	if modules.rateLimit != nil {
		middleware = append(middleware, httpserver.RateLimit(modules.rateLimit, apiRateLimit, apiRateLimitWindow, logger))
	}
	return httpserver.Chain(routes, middleware...)
}

// writeUnmatchedRoute replaces the plain-text 404 and 405 of ServeMux with the API JSON error format.
func writeUnmatchedRoute(w http.ResponseWriter, r *http.Request, mux *http.ServeMux) {
	var allowed []string
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete} {
		probe := r.Clone(r.Context())
		probe.Method = method
		if _, pattern := mux.Handler(probe); pattern != "" {
			allowed = append(allowed, method)
		}
	}
	if len(allowed) == 0 {
		_ = httpserver.WriteError(w, http.StatusNotFound, "not_found", "Маршрут не найден", nil)
		return
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	_ = httpserver.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Метод не поддерживается", nil)
}
