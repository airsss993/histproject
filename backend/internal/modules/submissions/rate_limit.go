package submissions

import (
	"context"
	"fmt"
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

// Allow считает заявки по clientKey - HMAC от IP из httpserver.ClientKey.
func (limiter *RedisCreateLimiter) Allow(ctx context.Context, clientKey string) (bool, error) {
	count, err := incrementSubmissionAttempts.Run(
		ctx,
		limiter.client,
		[]string{"submissions:create:" + clientKey},
		submissionRateWindow.Milliseconds(),
	).Int64()
	if err != nil {
		return false, fmt.Errorf("increment submission rate limit: %w", err)
	}
	return count <= submissionRateLimit, nil
}
