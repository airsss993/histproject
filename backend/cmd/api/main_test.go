package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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

	code := run(ctx, func(string) string { return "" }, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() = %d, want 0; stderr = %q", code, stderr.String())
	}
}
