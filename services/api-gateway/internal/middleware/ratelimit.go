package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit returns chi middleware enforcing per-IP (ipLimit/min) and per-user (userLimit/min)
// rate limits using Redis INCR + EXPIRE. Keys: "rl:ip:{ip}" and "rl:user:{user_id}".
func RateLimit(rdb *redis.Client, ipLimit, userLimit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ip := realIP(r)
			ipKey := "rl:ip:" + ip

			count, err := rdb.Incr(ctx, ipKey).Result()
			if err == nil && count == 1 {
				rdb.Expire(ctx, ipKey, time.Minute)
			}
			if count > int64(ipLimit) {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			// Per-user check only when authenticated (user_id in context from AuthMiddleware)
			if userID, ok := UserIDFromContext(ctx); ok && userID != "" {
				userKey := "rl:user:" + userID
				ucount, _ := rdb.Incr(ctx, userKey).Result()
				if ucount == 1 {
					rdb.Expire(ctx, userKey, time.Minute)
				}
				if ucount > int64(userLimit) {
					http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// realIP extracts client IP from X-Forwarded-For, X-Real-IP, or RemoteAddr.
// X-Forwarded-For values are bare IPs without port, so strings.Split is used
// to parse the first entry safely.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}
