package auth

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisSessionStoreLifecycle(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("TEST_REDIS_ADDRESS is not set")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || (host != "127.0.0.1" && host != "localhost") {
		t.Fatalf("Redis address %q is not an isolated local test service", address)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: address, DB: 15})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush test Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.FlushDB(context.Background()).Err() })

	store := NewSessionStore(client)
	identity := Identity{AdminID: 42, Login: "root", Role: "super_admin"}
	if err := store.Create(ctx, "raw-session-token", identity, time.Hour); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	keys, err := client.Keys(ctx, sessionKeyPrefix+"*").Result()
	if err != nil {
		t.Fatalf("list session keys: %v", err)
	}
	if len(keys) != 1 || strings.Contains(keys[0], "raw-session-token") {
		t.Fatalf("session keys = %#v", keys)
	}
	ttl, err := client.TTL(ctx, keys[0]).Result()
	if err != nil {
		t.Fatalf("read session TTL: %v", err)
	}
	if ttl <= 0 || ttl > time.Hour {
		t.Fatalf("session TTL = %s", ttl)
	}

	got, err := store.Get(ctx, "raw-session-token")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != identity {
		t.Fatalf("Get() = %#v", got)
	}
	if err := store.Delete(ctx, "raw-session-token"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, "raw-session-token"); err != ErrSessionNotFound {
		t.Fatalf("Get() after delete error = %v, want ErrSessionNotFound", err)
	}
}
