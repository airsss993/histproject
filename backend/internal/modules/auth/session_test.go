package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestSessionStoreCreatesDigestKeyWithTTL(t *testing.T) {
	client := &redisStub{}
	store := newSessionStore(client)
	identity := Identity{AdminID: 42, Login: "root", Role: "super_admin"}
	rawToken := "raw-secret-session-token"

	if err := store.Create(t.Context(), rawToken, identity, 12*time.Hour); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if client.setExpiration != 12*time.Hour {
		t.Fatalf("expiration = %s, want 12h", client.setExpiration)
	}
	if strings.Contains(client.setKey, rawToken) || len(strings.TrimPrefix(client.setKey, sessionKeyPrefix)) != 64 {
		t.Fatalf("session key = %q, want SHA-256 digest", client.setKey)
	}
	if strings.Contains(client.setValue, rawToken) {
		t.Fatal("Redis value contains raw token")
	}
}

func TestSessionStoreGetsIdentity(t *testing.T) {
	client := &redisStub{
		getValue: `{"admin_id":42,"login":"root","role":"super_admin"}`,
	}
	store := newSessionStore(client)

	identity, err := store.Get(t.Context(), "session-token")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if identity != (Identity{AdminID: 42, Login: "root", Role: "super_admin"}) {
		t.Fatalf("identity = %#v", identity)
	}
	if client.getKey != sessionKey("session-token") {
		t.Fatalf("Redis key = %q", client.getKey)
	}
}

func TestSessionStoreMapsMissingSession(t *testing.T) {
	store := newSessionStore(&redisStub{getError: redis.Nil})

	_, err := store.Get(t.Context(), "missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Get() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionStoreDeletesDigestKey(t *testing.T) {
	client := &redisStub{}
	store := newSessionStore(client)

	if err := store.Delete(t.Context(), "session-token"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if client.deleteKey != sessionKey("session-token") {
		t.Fatalf("deleted key = %q", client.deleteKey)
	}
}

type redisStub struct {
	setKey        string
	setValue      string
	setExpiration time.Duration
	setError      error
	getKey        string
	getValue      string
	getError      error
	deleteKey     string
	deleteError   error
}

func (stub *redisStub) Set(_ context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	stub.setKey = key
	stub.setValue, _ = value.(string)
	stub.setExpiration = expiration
	return redis.NewStatusResult("OK", stub.setError)
}

func (stub *redisStub) Get(_ context.Context, key string) *redis.StringCmd {
	stub.getKey = key
	return redis.NewStringResult(stub.getValue, stub.getError)
}

func (stub *redisStub) Del(_ context.Context, keys ...string) *redis.IntCmd {
	if len(keys) > 0 {
		stub.deleteKey = keys[0]
	}
	return redis.NewIntResult(1, stub.deleteError)
}
