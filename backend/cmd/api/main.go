package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/airsss993/histproject/backend/internal/app"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
	"github.com/airsss993/histproject/backend/internal/platform/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Getenv, os.Stdout, os.Stderr))
}

func run(ctx context.Context, getenv func(string) string, stdout, stderr io.Writer) int {
	cfg, err := config.LoadFrom(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "config:", err)
		return 1
	}

	logger := logging.New(cfg.Environment, stdout)
	handler := app.NewAPI(cfg, logger)
	server := httpserver.NewServer(httpserver.ServerConfig{
		Address:         cfg.ServerAddress,
		ShutdownTimeout: cfg.ShutdownTimeout,
	}, handler, logger)
	if err := server.Run(ctx); err != nil {
		logger.Error("HTTP server failed", "error", err)
		return 1
	}
	return 0
}
