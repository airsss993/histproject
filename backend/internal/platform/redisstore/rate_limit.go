package redisstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var incrementWindow = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// Allow counts one hit for key in a fixed window and reports whether the count is within limit.
func (s *Store) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	count, err := incrementWindow.Run(ctx, s.client, []string{key}, window.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("increment rate limit: %w", err)
	}
	return count <= limit, nil
}
