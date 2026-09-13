package submissions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	submissionRateLimit  = 5
	submissionRateWindow = time.Hour
)

var incrementSubmissionAttempts = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

type RedisCreateLimiter struct {
	client *redis.Client
}

func NewCreateLimiter(client *redis.Client) *RedisCreateLimiter {
	return &RedisCreateLimiter{client: client}
}

func (limiter *RedisCreateLimiter) Allow(ctx context.Context, remoteAddress string) (bool, error) {
	count, err := incrementSubmissionAttempts.Run(
		ctx,
		limiter.client,
		[]string{submissionRateKey(remoteAddress)},
		submissionRateWindow.Milliseconds(),
	).Int64()
	if err != nil {
		return false, fmt.Errorf("increment submission rate limit: %w", err)
	}
	return count <= submissionRateLimit, nil
}

func submissionRateKey(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	digest := sha256.Sum256([]byte(host))
	return "submissions:create:" + hex.EncodeToString(digest[:])
}
