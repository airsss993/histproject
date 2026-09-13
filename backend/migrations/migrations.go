package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var embedMigrations embed.FS

func Run(db *sql.DB) error {
	return up(context.Background(), db, embedMigrations)
}

func up(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	if db == nil {
		return errors.New("migration database is nil")
	}

	provider, err := newProvider(db, migrations)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

func newProvider(db *sql.DB, migrations fs.FS) (*goose.Provider, error) {
	return goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations,
		goose.WithDisableGlobalRegistry(true),
	)
}
