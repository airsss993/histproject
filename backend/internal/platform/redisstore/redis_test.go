package redisstore

import (
	"context"
	"strings"
	"testing"
)

func TestStoreName(t *testing.T) {
	store := &Store{}
	if store.Name() != "redis" {
		t.Fatalf("Name() = %q, want redis", store.Name())
	}
}

func TestOpenHonorsCanceledContextWithoutLeakingPassword(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store, err := Open(ctx, "127.0.0.1:1", "secret-value", 4)

	if err == nil {
		_ = store.Close()
		t.Fatal("Open() error = nil")
	}
	if store != nil {
		t.Fatal("Open() store is not nil")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Open() leaks Redis password: %v", err)
	}
}
