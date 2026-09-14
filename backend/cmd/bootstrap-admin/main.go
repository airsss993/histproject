package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/jackc/pgx/v5/pgxpool"
)

const bootstrapTimeout = 30 * time.Second

type bootstrapInput struct {
	databaseURL string
	login       string
	password    string
	email       string
}

type bootstrapRunner func(context.Context, bootstrapInput) error

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr, applyBootstrap))
}

func run(parent context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer, bootstrap bootstrapRunner) int {
	flags := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
	flags.SetOutput(stderr)
	apply := flags.Bool("apply", false, "create the initial super admin")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*apply {
		_, _ = fmt.Fprintln(stderr, "bootstrap refused: pass --apply")
		return 2
	}

	input := bootstrapInput{
		databaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
		login:       strings.TrimSpace(getenv("BOOTSTRAP_ADMIN_LOGIN")),
		password:    getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		email:       strings.TrimSpace(getenv("BOOTSTRAP_ADMIN_EMAIL")),
	}
	for key, value := range map[string]string{
		"DATABASE_URL":             input.databaseURL,
		"BOOTSTRAP_ADMIN_LOGIN":    input.login,
		"BOOTSTRAP_ADMIN_PASSWORD": input.password,
	} {
		if value == "" {
			_, _ = fmt.Fprintf(stderr, "%s is required\n", key)
			return 2
		}
	}

	ctx, cancel := context.WithTimeout(parent, bootstrapTimeout)
	defer cancel()
	if err := bootstrap(ctx, input); err != nil {
		if errors.Is(err, admins.ErrSuperAdminExists) {
			_, _ = fmt.Fprintln(stderr, "super admin already exists")
		} else {
			_, _ = fmt.Fprintln(stderr, "bootstrap failed")
		}
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "initial super admin created")
	return 0
}

func applyBootstrap(ctx context.Context, input bootstrapInput) error {
	pool, err := pgxpool.New(ctx, input.databaseURL)
	if err != nil {
		return errors.New("open bootstrap database")
	}
	defer pool.Close()

	repository := admins.NewRepository(pool)
	service := admins.NewService(repository)
	_, err = service.Bootstrap(ctx, admins.CreateInput{
		Login:    input.login,
		Password: input.password,
		Email:    input.email,
	})
	return err
}
