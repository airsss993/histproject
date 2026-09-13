package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/health"
)

type fakeDependencies struct {
	closed bool
}

func (*fakeDependencies) Checkers() []health.Checker { return nil }
func (d *fakeDependencies) Close() error {
	d.closed = true
	return nil
}

func TestRunReturnsFailureForInvalidProductionConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), func(key string) string {
		if key == "APP_ENV" {
			return "production"
		}
		return ""
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL") {
		t.Fatalf("stderr = %q, want missing variable", stderr.String())
	}
}

func TestRunStopsCleanlyWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	dependencies := &fakeDependencies{}
	code := runWithOpener(ctx, func(string) string { return "" }, &stdout, &stderr,
		func(context.Context, config.Config) (dependencySet, error) {
			return dependencies, nil
		},
	)

	if code != 0 {
		t.Fatalf("run() = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !dependencies.closed {
		t.Fatal("dependencies were not closed")
	}
}

func TestRunReturnsFailureWhenDependenciesCannotOpen(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	want := errors.New("postgres unavailable")

	code := runWithOpener(context.Background(), func(string) string { return "" }, &stdout, &stderr,
		func(context.Context, config.Config) (dependencySet, error) {
			return nil, want
		},
	)

	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "dependencies") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
