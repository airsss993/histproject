package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ClientRateLimiter counts one hit for key and reports whether it fits into limit per window.
type ClientRateLimiter func(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)

// RateLimit caps /api/ requests per client IP. A limiter error lets the request through:
// a Redis outage must not take the public map down.
// ponytail: one counter per IP, IPv6 clients rotating addresses inside a /64 get separate counters.
func RateLimit(allow ClientRateLimiter, limit int64, window time.Duration, logger *slog.Logger) Middleware {
	retryAfter := strconv.FormatInt(int64(window/time.Second), 10)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Caddy calls the session endpoint for every preview asset; without a valid session it answers from Redis alone.
			if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/v1/auth/session" {
				next.ServeHTTP(w, r)
				return
			}
			digest := sha256.Sum256([]byte(ClientIP(r)))
			allowed, err := allow(r.Context(), "api:client:"+hex.EncodeToString(digest[:]), limit, window)
			if err != nil {
				logger.WarnContext(r.Context(), "rate limit unavailable", "request_id", RequestIDFromContext(r.Context()), "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", retryAfter)
				_ = WriteError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
