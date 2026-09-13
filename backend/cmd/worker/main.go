package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airsss993/histproject/backend/internal/app"
	"github.com/airsss993/histproject/backend/internal/modules/sites"
	"github.com/airsss993/histproject/backend/internal/modules/submissions"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/logging"
	"github.com/hibiken/asynq"
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
	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	dependencies, err := app.OpenDependencies(startupCtx, cfg)
	cancelStartup()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "dependencies:", err)
		return 1
	}

	workerErr := runWorker(ctx, cfg, logger, dependencies)
	closeErr := dependencies.Close()
	if workerErr != nil {
		logger.Error("worker failed", "error", workerErr)
	}
	if closeErr != nil {
		logger.Error("close dependencies", "error", closeErr)
	}
	if workerErr != nil || closeErr != nil {
		return 1
	}
	return 0
}

func runWorker(ctx context.Context, cfg config.Config, logger *slog.Logger, dependencies *app.Dependencies) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	screenshotter, err := sites.NewChromeScreenshotter(cfg.ChromeURL, cfg.ScreenshotTimeout)
	if err != nil {
		return err
	}
	submissionService := submissions.NewService(
		submissions.NewRepository(dependencies.Database.Pool()), nil, nil, nil, nil, cfg.MaxUploadBytes,
	)
	processor := sites.NewProcessor(
		submissionService,
		func(ctx context.Context, archiveID string) (io.ReaderAt, io.Closer, int64, error) {
			object, size, err := dependencies.Storage.GetArchive(ctx, archiveID)
			if err != nil {
				return nil, nil, 0, err
			}
			return object, object, size, nil
		},
		dependencies.Storage,
		screenshotter,
		sites.ProcessorConfig{
			Limits: sites.Limits{
				MaxArchiveBytes:   cfg.MaxUploadBytes,
				MaxExtractedBytes: cfg.MaxExtractedBytes,
				MaxFiles:          cfg.MaxArchiveFiles,
				MaxFileBytes:      cfg.MaxArchiveFileBytes,
			},
			RenderBaseURL:     cfg.SiteRenderBaseURL,
			PreviewBaseURL:    cfg.SitePreviewBaseURL,
			PublicSiteBaseURL: cfg.SitePublicBaseURL,
		},
	)
	server := asynq.NewServerFromRedisClient(dependencies.Redis.Client(), asynq.Config{
		Concurrency:     1,
		ShutdownTimeout: cfg.ShutdownTimeout,
		BaseContext: func() context.Context {
			return ctx
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(taskCtx context.Context, task *asynq.Task, taskErr error) {
			retry, _ := asynq.GetRetryCount(taskCtx)
			maxRetry, _ := asynq.GetMaxRetry(taskCtx)
			logger.Error("archive task failed", "type", task.Type(), "retry", retry, "maxRetry", maxRetry, "error", taskErr)
		}),
	})
	if err := server.Start(sites.NewTaskMux(processor)); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	logger.Info("worker started", "concurrency", 1)
	<-ctx.Done()
	server.Shutdown()
	if !errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	return nil
}
