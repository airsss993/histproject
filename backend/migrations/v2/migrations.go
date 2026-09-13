package v2

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
var embeddedMigrations embed.FS

func Run(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("migration database is nil")
	}

	provider, err := newProvider(db, embeddedMigrations)
	if err != nil {
		return fmt.Errorf("create V2 migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("goose V2 up: %w", err)
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
