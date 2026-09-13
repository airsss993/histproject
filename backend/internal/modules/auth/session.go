package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "auth:session:"

var ErrSessionNotFound = errors.New("session not found")

type Identity struct {
	AdminID int64  `json:"admin_id"`
	Login   string `json:"login"`
	Role    string `json:"role"`
}

type redisCommands interface {
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Get(context.Context, string) *redis.StringCmd
	Del(context.Context, ...string) *redis.IntCmd
}

type RedisSessionStore struct {
	client redisCommands
}

func NewSessionStore(client *redis.Client) *RedisSessionStore {
	return newSessionStore(client)
}

func newSessionStore(client redisCommands) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

func (store *RedisSessionStore) Create(ctx context.Context, token string, identity Identity, ttl time.Duration) error {
	payload, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	if err := store.client.Set(ctx, sessionKey(token), string(payload), ttl).Err(); err != nil {
		return fmt.Errorf("store session: %w", err)
	}
	return nil
}

func (store *RedisSessionStore) Get(ctx context.Context, token string) (Identity, error) {
	payload, err := store.client.Get(ctx, sessionKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return Identity{}, ErrSessionNotFound
	}
	if err != nil {
		return Identity{}, fmt.Errorf("read session: %w", err)
	}

	var identity Identity
	if err := json.Unmarshal([]byte(payload), &identity); err != nil {
		return Identity{}, fmt.Errorf("decode session: %w", err)
	}
	return identity, nil
}

func (store *RedisSessionStore) Delete(ctx context.Context, token string) error {
	if err := store.client.Del(ctx, sessionKey(token)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func sessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return sessionKeyPrefix + hex.EncodeToString(digest[:])
}
