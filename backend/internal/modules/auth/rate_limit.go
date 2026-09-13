package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	loginRateLimit  = 5
	loginRateWindow = time.Minute
)

var incrementLoginAttempts = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

type RedisLoginLimiter struct {
	client *redis.Client
}

func NewLoginLimiter(client *redis.Client) *RedisLoginLimiter {
	return &RedisLoginLimiter{client: client}
}

func (limiter *RedisLoginLimiter) Allow(ctx context.Context, login, remoteAddress string) (bool, error) {
	count, err := incrementLoginAttempts.Run(ctx, limiter.client, []string{loginRateKey(login, remoteAddress)}, loginRateWindow.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("increment login rate limit: %w", err)
	}
	return count <= loginRateLimit, nil
}

func loginRateKey(login, remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(login)) + "\x00" + host))
	return "auth:login:" + hex.EncodeToString(digest[:])
}
