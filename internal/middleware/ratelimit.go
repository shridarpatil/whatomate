package middleware

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
)

// RateLimitOpts configures the rate limit middleware.
type RateLimitOpts struct {
	Redis      *redis.Client
	Log        logf.Logger
	Max        int           // Maximum attempts within the window.
	Window     time.Duration // Fixed window duration.
	KeyPrefix  string        // Redis key prefix (e.g., "login", "register").
	TrustProxy bool          // Trust X-Forwarded-For / X-Real-IP headers.
}

// RateLimit returns a fastglue middleware that enforces a fixed-window
// rate limit per client IP using Redis INCR + EXPIRE.
// It fails open: if Redis is unavailable the request is allowed through.
func RateLimit(opts RateLimitOpts) fastglue.FastMiddleware {
	return func(r *fastglue.Request) *fastglue.Request {
		ip := extractClientIP(r, opts.TrustProxy)
		key := fmt.Sprintf("ratelimit:%s:%s", opts.KeyPrefix, ip)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		count, err := opts.Redis.Incr(ctx, key).Result()
		if err != nil {
			// Fail open — log and allow request.
			opts.Log.Error("Rate limit Redis INCR failed", "error", err, "key", key)
			return r
		}

		// Set expiry on first increment (new window).
		if count == 1 {
			if err := opts.Redis.Expire(ctx, key, opts.Window).Err(); err != nil {
				opts.Log.Error("Rate limit Redis EXPIRE failed", "error", err, "key", key)
			}
		}

		if count > int64(opts.Max) {
			// Look up remaining TTL for Retry-After header.
			ttl, err := opts.Redis.TTL(ctx, key).Result()
			if err != nil || ttl < 0 {
				ttl = opts.Window
			}
			retryAfter := int(ttl.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			r.RequestCtx.Response.Header.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			_ = r.SendErrorEnvelope(fasthttp.StatusTooManyRequests,
				"Too many requests. Please try again later.", nil, "")
			return nil
		}

		return r
	}
}

// extractClientIP returns the client IP address from the request.
// When trustProxy is true, it checks X-Forwarded-For and X-Real-IP headers first.
func extractClientIP(r *fastglue.Request, trustProxy bool) string {
	if trustProxy {
		// X-Forwarded-For may contain a chain: "client, proxy1, proxy2"
		if xff := string(r.RequestCtx.Request.Header.Peek("X-Forwarded-For")); xff != "" {
			parts := strings.SplitN(xff, ",", 2)
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
		if realIP := string(r.RequestCtx.Request.Header.Peek("X-Real-IP")); realIP != "" {
			return strings.TrimSpace(realIP)
		}
	}

	// Fall back to RemoteAddr (strip port).
	addr := r.RequestCtx.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
