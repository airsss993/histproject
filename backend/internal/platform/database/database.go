package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Database, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	database := &Database{pool: pool}
	if err := database.Ping(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return database, nil
}

func (d *Database) Name() string {
	return "postgres"
}

func (d *Database) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

func (d *Database) Close() {
	if d != nil && d.pool != nil {
		d.pool.Close()
	}
}

func (d *Database) Pool() *pgxpool.Pool {
	return d.pool
}
