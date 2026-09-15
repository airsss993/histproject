package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airsss993/histproject/backend/internal/app"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/health"
	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
	"github.com/airsss993/histproject/backend/internal/platform/logging"
)

type dependencySet interface {
	Checkers() []health.Checker
	Modules(config.Config, *slog.Logger) app.Modules
	Close() error
}

type dependencyOpener func(context.Context, config.Config) (dependencySet, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Getenv, os.Stdout, os.Stderr))
}

func run(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) int {
	return runWithOpener(ctx, getenv, stdout, stderr, func(ctx context.Context, cfg config.Config) (dependencySet, error) {
		return app.OpenDependencies(ctx, cfg)
	})
}

func runWithOpener(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer, open dependencyOpener) int {
	cfg, err := config.LoadFrom(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "config:", err)
		return 1
	}

	logger := logging.New(cfg.Environment, stdout)
	slog.SetDefault(logger)
	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	dependencies, err := open(startupCtx, cfg)
	cancelStartup()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "dependencies:", err)
		return 1
	}

	handler := app.NewAPI(cfg, logger, dependencies.Modules(cfg, logger), dependencies.Checkers()...)
	server := httpserver.NewServer(httpserver.ServerConfig{
		Address:         cfg.ServerAddress,
		ShutdownTimeout: cfg.ShutdownTimeout,
	}, handler, logger)
	serverErr := server.Run(ctx)
	closeErr := dependencies.Close()
	if serverErr != nil {
		logger.Error("HTTP server failed", "error", serverErr)
	}
	if closeErr != nil {
		logger.Error("close dependencies", "error", closeErr)
	}
	if serverErr != nil || closeErr != nil {
		return 1
	}
	return 0
}
