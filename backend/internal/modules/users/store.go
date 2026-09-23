package users

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Код лежит хешем, счётчик попыток рядом. Кончились попытки - код стирается, нужен новый.
var checkCode = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return {-1} end
local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
local v = redis.call('HMGET', KEYS[1], 'hash', 'first', 'last')
if v[1] == ARGV[1] then
  redis.call('DEL', KEYS[1])
  return {1, v[2], v[3]}
end
if attempts >= tonumber(ARGV[2]) then
  redis.call('DEL', KEYS[1])
  return {-1}
end
return {0}
`)

var incrementAttempts = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

type RedisStore struct {
	client *redis.Client
}

func NewStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (store *RedisStore) SaveCode(ctx context.Context, email string, pending Pending, ttl time.Duration) error {
	key := codeKey(email)
	_, err := store.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, key)
		pipe.HSet(ctx, key, "hash", pending.CodeHash, "first", pending.FirstName, "last", pending.LastName, "attempts", 0)
		pipe.PExpire(ctx, key, ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("save code: %w", err)
	}
	return nil
}

func (store *RedisStore) CheckCode(ctx context.Context, email, hash string, attempts int) (Pending, error) {
	result, err := checkCode.Run(ctx, store.client, []string{codeKey(email)}, hash, attempts).Slice()
	if err != nil {
		return Pending{}, fmt.Errorf("check code: %w", err)
	}
	switch result[0].(int64) {
	case 1:
		return Pending{CodeHash: hash, FirstName: result[1].(string), LastName: result[2].(string)}, nil
	case 0:
		return Pending{}, ErrInvalidCode
	default:
		return Pending{}, ErrCodeExpired
	}
}

func (store *RedisStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	count, err := incrementAttempts.Run(ctx, store.client, []string{key}, window.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("increment %s: %w", key, err)
	}
	return count <= int64(limit), nil
}

func (store *RedisStore) ResetLimit(ctx context.Context, key string) error {
	if err := store.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("reset %s: %w", key, err)
	}
	return nil
}

func (store *RedisStore) CreateSession(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	if err := store.client.Set(ctx, sessionKey(token), userID, ttl).Err(); err != nil {
		return fmt.Errorf("store user session: %w", err)
	}
	return nil
}

func (store *RedisStore) SessionUser(ctx context.Context, token string) (int64, error) {
	value, err := store.client.Get(ctx, sessionKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrNoSession
	}
	if err != nil {
		return 0, fmt.Errorf("read user session: %w", err)
	}
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, ErrNoSession
	}
	return userID, nil
}

func (store *RedisStore) DeleteSession(ctx context.Context, token string) error {
	if err := store.client.Del(ctx, sessionKey(token)).Err(); err != nil {
		return fmt.Errorf("delete user session: %w", err)
	}
	return nil
}

func codeKey(email string) string    { return "users:code:" + digest(email) }
func sessionKey(token string) string { return "users:session:" + digest(token) }
