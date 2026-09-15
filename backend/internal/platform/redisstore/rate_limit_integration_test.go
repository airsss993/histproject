package redisstore

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestAllowCountsHitsInWindow(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("TEST_REDIS_ADDRESS is not set")
	}
	store, err := Open(t.Context(), address, "", 0)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	key := "test:rate-limit:" + t.Name()
	// t.Context is already canceled when cleanup runs.
	t.Cleanup(func() { _ = store.Client().Del(context.Background(), key).Err() })
	if err := store.Client().Del(t.Context(), key).Err(); err != nil {
		t.Fatalf("reset key: %v", err)
	}

	for hit := 1; hit <= 3; hit++ {
		allowed, err := store.Allow(t.Context(), key, 2, time.Minute)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if allowed != (hit <= 2) {
			t.Fatalf("hit %d allowed = %v", hit, allowed)
		}
	}
	if ttl := store.Client().PTTL(t.Context(), key).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("PTTL = %v", ttl)
	}
}
