package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	"github.com/airsss993/histproject/backend/internal/platform/health"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
	"github.com/airsss993/histproject/backend/internal/platform/storage"
)

type Dependencies struct {
	Database  *database.Database
	Redis     *redisstore.Store
	Storage   *storage.MinIO
	resources []dependencyResource
}

type dependencyResource struct {
	checker health.Checker
	close   func() error
}

type dependencyOpener func(context.Context) (dependencyResource, error)

func OpenDependencies(ctx context.Context, cfg config.Config) (*Dependencies, error) {
	dependencies := &Dependencies{}
	resources, err := openAll(ctx, []dependencyOpener{
		func(ctx context.Context) (dependencyResource, error) {
			databaseClient, err := database.Open(ctx, cfg.DatabaseURL)
			if err != nil {
				return dependencyResource{}, fmt.Errorf("open PostgreSQL: %w", err)
			}
			dependencies.Database = databaseClient
			return dependencyResource{
				checker: databaseClient,
				close: func() error {
					databaseClient.Close()
					return nil
				},
			}, nil
		},
		func(ctx context.Context) (dependencyResource, error) {
			redisClient, err := redisstore.Open(ctx, cfg.RedisAddress, cfg.RedisPassword, cfg.RedisDB)
			if err != nil {
				return dependencyResource{}, fmt.Errorf("open Redis: %w", err)
			}
			dependencies.Redis = redisClient
			return dependencyResource{checker: redisClient, close: redisClient.Close}, nil
		},
		func(ctx context.Context) (dependencyResource, error) {
			storageClient, err := storage.Open(ctx, storage.Config{
				Endpoint:      cfg.MinIOEndpoint,
				AccessKey:     cfg.MinIOAccessKey,
				SecretKey:     cfg.MinIOSecretKey,
				ArchiveBucket: cfg.MinIOArchiveBucket,
				SiteBucket:    cfg.MinIOSiteBucket,
				Secure:        cfg.MinIOUseSSL,
			})
			if err != nil {
				return dependencyResource{}, fmt.Errorf("open MinIO: %w", err)
			}
			dependencies.Storage = storageClient
			return dependencyResource{checker: storageClient}, nil
		},
	})
	if err != nil {
		return nil, err
	}
	dependencies.resources = resources
	return dependencies, nil
}

func (d *Dependencies) Checkers() []health.Checker {
	checkers := make([]health.Checker, 0, len(d.resources))
	for _, resource := range d.resources {
		checkers = append(checkers, resource.checker)
	}
	return checkers
}

func (d *Dependencies) Modules(cfg config.Config, logger *slog.Logger) Modules {
	return NewModules(cfg, logger, d.Database, d.Redis, d.Storage)
}

func (d *Dependencies) Close() error {
	if d == nil {
		return nil
	}
	return closeResources(d.resources)
}

func openAll(ctx context.Context, openers []dependencyOpener) ([]dependencyResource, error) {
	resources := make([]dependencyResource, 0, len(openers))
	for _, open := range openers {
		resource, err := open(ctx)
		if err != nil {
			return nil, errors.Join(err, closeResources(resources))
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func closeResources(resources []dependencyResource) error {
	var closeErrors []error
	for index := len(resources) - 1; index >= 0; index-- {
		if resources[index].close == nil {
			continue
		}
		if err := resources[index].close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}
