package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunRefusesWithoutApplyFlag(t *testing.T) {
	called := false
	var stderr bytes.Buffer

	code := run(t.Context(), nil, env(map[string]string{
		"DATABASE_URL": "postgres://database",
	}), &bytes.Buffer{}, &stderr, func(context.Context, string) error {
		called = true
		return nil
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if called {
		t.Fatal("migration runner was called without --apply")
	}
	if !strings.Contains(stderr.String(), "--apply") {
		t.Fatalf("stderr = %q, want --apply hint", stderr.String())
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--apply"}, env(nil), &bytes.Buffer{}, &stderr, func(context.Context, string) error {
		t.Fatal("migration runner must not be called")
		return nil
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL") {
		t.Fatalf("stderr = %q, want DATABASE_URL", stderr.String())
	}
}

func TestRunRejectsInvalidTimeout(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--apply"}, env(map[string]string{
		"DATABASE_URL":      "postgres://database",
		"MIGRATION_TIMEOUT": "later",
	}), &bytes.Buffer{}, &stderr, func(context.Context, string) error {
		t.Fatal("migration runner must not be called")
		return nil
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "MIGRATION_TIMEOUT") {
		t.Fatalf("stderr = %q, want MIGRATION_TIMEOUT", stderr.String())
	}
}

func TestRunAppliesMigrationsWithTimeout(t *testing.T) {
	var stdout bytes.Buffer
	called := false

	code := run(t.Context(), []string{"--apply"}, env(map[string]string{
		"DATABASE_URL":      "postgres://database",
		"MIGRATION_TIMEOUT": "15s",
	}), &stdout, &bytes.Buffer{}, func(ctx context.Context, databaseURL string) error {
		called = true
		if databaseURL != "postgres://database" {
			t.Fatalf("database URL = %q", databaseURL)
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("migration context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 15*time.Second {
			t.Fatalf("migration deadline remaining = %s", remaining)
		}
		return nil
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !called {
		t.Fatal("migration runner was not called")
	}
	if !strings.Contains(stdout.String(), "completed") {
		t.Fatalf("stdout = %q, want completion message", stdout.String())
	}
}

func TestRunDoesNotPrintMigrationErrorDetails(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--apply"}, env(map[string]string{
		"DATABASE_URL": "postgres://admin:secret-value@database/histproject",
	}), &bytes.Buffer{}, &stderr, func(context.Context, string) error {
		return errors.New("connect postgres://admin:secret-value@database/histproject")
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), "secret-value") || strings.Contains(stderr.String(), "postgres://") {
		t.Fatalf("stderr exposes migration details: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "migration failed") {
		t.Fatalf("stderr = %q, want safe failure", stderr.String())
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
