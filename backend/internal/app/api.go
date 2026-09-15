package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/health"
	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

func NewAPI(cfg config.Config, logger *slog.Logger, modules Modules, checkers ...health.Checker) http.Handler {
	mux := http.NewServeMux()
	health.NewHandler(2*time.Second, checkers...).Register(mux)
	modules.Register(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_ = httpserver.WriteError(w, http.StatusNotFound, "not_found", "Маршрут не найден", nil)
	})

	return httpserver.Chain(
		mux,
		httpserver.RequestID,
		httpserver.AccessLog(logger),
		httpserver.Recover(logger),
		httpserver.SecurityHeaders,
		httpserver.CORS(cfg.CORSOrigins),
	)
}
