package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/airsss993/histproject/backend/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultMigrationTimeout = 2 * time.Minute

type migrationRunner func(context.Context, string) error

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr, applyMigrations))
}

func run(parent context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer, migrate migrationRunner) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	apply := flags.Bool("apply", false, "apply pending migrations")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*apply {
		_, _ = fmt.Fprintln(stderr, "migration refused: pass --apply")
		return 2
	}

	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		_, _ = fmt.Fprintln(stderr, "DATABASE_URL is required")
		return 2
	}

	timeout, err := migrationTimeout(getenv("MIGRATION_TIMEOUT"))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	if err := migrate(ctx, databaseURL); err != nil {
		_, _ = fmt.Fprintln(stderr, "migration failed")
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "database migration completed")
	return 0
}

func migrationTimeout(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return defaultMigrationTimeout, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("MIGRATION_TIMEOUT must be a positive duration")
	}
	return timeout, nil
}

func applyMigrations(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database")
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect migration database")
	}
	if err := migrations.Up(ctx, db); err != nil {
		return fmt.Errorf("apply database schema")
	}

	return nil
}
