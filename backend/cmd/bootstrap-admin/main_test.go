package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
)

func TestRunRefusesWithoutApplyFlag(t *testing.T) {
	called := false
	var stderr bytes.Buffer

	code := run(t.Context(), nil, env(map[string]string{
		"DATABASE_URL":             "postgres://database",
		"BOOTSTRAP_ADMIN_LOGIN":    "root",
		"BOOTSTRAP_ADMIN_PASSWORD": "long-enough-password",
		"BOOTSTRAP_ADMIN_EMAIL":    "root@example.test",
	}), &bytes.Buffer{}, &stderr, func(context.Context, bootstrapInput) error {
		called = true
		return nil
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if called {
		t.Fatal("bootstrap runner was called without --apply")
	}
	if !strings.Contains(stderr.String(), "--apply") {
		t.Fatalf("stderr = %q, want --apply hint", stderr.String())
	}
}

func TestRunRequiresBootstrapEnvironment(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{name: "database URL", values: map[string]string{"BOOTSTRAP_ADMIN_LOGIN": "root", "BOOTSTRAP_ADMIN_PASSWORD": "long-enough-password", "BOOTSTRAP_ADMIN_EMAIL": "root@example.test"}, want: "DATABASE_URL"},
		{name: "login", values: map[string]string{"DATABASE_URL": "postgres://database", "BOOTSTRAP_ADMIN_PASSWORD": "long-enough-password", "BOOTSTRAP_ADMIN_EMAIL": "root@example.test"}, want: "BOOTSTRAP_ADMIN_LOGIN"},
		{name: "password", values: map[string]string{"DATABASE_URL": "postgres://database", "BOOTSTRAP_ADMIN_LOGIN": "root", "BOOTSTRAP_ADMIN_EMAIL": "root@example.test"}, want: "BOOTSTRAP_ADMIN_PASSWORD"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := run(t.Context(), []string{"--apply"}, env(test.values), &bytes.Buffer{}, &stderr, func(context.Context, bootstrapInput) error {
				t.Fatal("bootstrap runner must not be called")
				return nil
			})

			if code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %s", stderr.String(), test.want)
			}
		})
	}
}

func TestRunPassesNormalizedInput(t *testing.T) {
	var stdout bytes.Buffer

	code := run(t.Context(), []string{"--apply"}, env(map[string]string{
		"DATABASE_URL":             " postgres://database ",
		"BOOTSTRAP_ADMIN_LOGIN":    " root ",
		"BOOTSTRAP_ADMIN_PASSWORD": "long-enough-password",
		"BOOTSTRAP_ADMIN_EMAIL":    " root@example.test ",
	}), &stdout, &bytes.Buffer{}, func(_ context.Context, input bootstrapInput) error {
		if input.databaseURL != "postgres://database" || input.login != "root" || input.password != "long-enough-password" || input.email != "root@example.test" {
			t.Fatalf("bootstrap input = %#v", input)
		}
		return nil
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "created") {
		t.Fatalf("stdout = %q, want completion message", stdout.String())
	}
}

func TestRunReportsExistingSuperAdminWithoutSecrets(t *testing.T) {
	var stderr bytes.Buffer
	password := "secret-password-value"
	databaseURL := "postgres://admin:database-secret@database/histproject"

	code := run(t.Context(), []string{"--apply"}, env(map[string]string{
		"DATABASE_URL":             databaseURL,
		"BOOTSTRAP_ADMIN_LOGIN":    "root",
		"BOOTSTRAP_ADMIN_PASSWORD": password,
		"BOOTSTRAP_ADMIN_EMAIL":    "root@example.test",
	}), &bytes.Buffer{}, &stderr, func(context.Context, bootstrapInput) error {
		return admins.ErrSuperAdminExists
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), password) || strings.Contains(stderr.String(), databaseURL) || strings.Contains(stderr.String(), "database-secret") {
		t.Fatalf("stderr exposes bootstrap secrets: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr = %q, want existing admin message", stderr.String())
	}
}

func TestRunDoesNotPrintBootstrapErrorDetails(t *testing.T) {
	var stderr bytes.Buffer

	code := run(t.Context(), []string{"--apply"}, env(map[string]string{
		"DATABASE_URL":             "postgres://admin:database-secret@database/histproject",
		"BOOTSTRAP_ADMIN_LOGIN":    "root",
		"BOOTSTRAP_ADMIN_PASSWORD": "secret-password-value",
		"BOOTSTRAP_ADMIN_EMAIL":    "root@example.test",
	}), &bytes.Buffer{}, &stderr, func(context.Context, bootstrapInput) error {
		return errors.New("connect postgres://admin:database-secret@database/histproject with secret-password-value")
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), "secret") || strings.Contains(stderr.String(), "postgres://") {
		t.Fatalf("stderr exposes bootstrap details: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "bootstrap failed") {
		t.Fatalf("stderr = %q, want safe failure", stderr.String())
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
