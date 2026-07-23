package middleware

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/pkg/response"
	"golang.org/x/time/rate"
)

// limiterEntry wraps a rate.Limiter with a last-used timestamp so the
// periodic cleanup can evict only stale entries instead of wiping all of
// them (which would reset every active user's burst quota at once).
type limiterEntry struct {
	limiter  *rate.Limiter
	lastUsed atomic.Int64 // unix nanos
}

var (
	ipLimiters sync.Map   // map[string]*limiterEntry
	limitRate  rate.Limit // Token rate per second
	limitBurst = 100      // Max burst allowed
)

// limiterIdleTTL is how long an entry can be unused before it's eligible
// for eviction. Tuned to be longer than the cleanup interval so we never
// evict an entry that was touched in the most recent sweep.
const limiterIdleTTL = 30 * time.Minute

// Init initializes rate limiter configuration from global config
func Init(cfg config.Config) {
	// Convert per-minute limit to per-second rate
	limitRate = rate.Limit(cfg.RateLimit.PerMinute) / 60
}

// getLimiter returns a rate limiter for the given key, creating one if
// it does not exist. The entry's lastUsed is bumped on each access so the
// cleanup goroutine can distinguish active from idle limiters.
func getLimiter(key string) *rate.Limiter {
	now := time.Now().UnixNano()
	v, _ := ipLimiters.LoadOrStore(key, &limiterEntry{
		limiter:  rate.NewLimiter(limitRate, limitBurst),
		lastUsed: atomic.Int64{},
	})
	entry := v.(*limiterEntry)
	entry.lastUsed.Store(now)
	return entry.limiter
}

// RateLimitMiddleware provides per-token or per-IP rate limiting
// Uses token bucket algorithm for smooth traffic control
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get identity for rate limiting: prioritize API token, fall back to client IP
		token := c.GetHeader("X-API-TOKEN")
		ip := c.ClientIP()
		path := c.Request.URL.Path
		method := c.Request.Method

		limitKey := ip
		if token != "" {
			limitKey = token
		}

		logger.Debug("rate_limit_check",
			logger.String("ip", ip),
			logger.String("path", path),
			logger.String("method", method),
			logger.String("limit_key", limitKey),
		)

		// Get or create limiter
		limiter := getLimiter(limitKey)

		// Reject request if rate limit exceeded
		if !limiter.Allow() {
			logger.Warn("rate_limit_exceeded",
				logger.String("ip", ip),
				logger.String("path", path),
				logger.String("method", method),
				logger.String("limit_key", limitKey),
			)
			response.Fail(c, http.StatusTooManyRequests, "rate limit exceeded, please try later")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthMiddleware validates the X-API-TOKEN header against the whitelist in config
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-API-TOKEN")
		ip := c.ClientIP()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Reject if token is missing
		if token == "" {
			logger.Warn("auth_missing_token",
				logger.String("ip", ip),
				logger.String("path", path),
				logger.String("method", method),
			)
			response.Fail(c, http.StatusBadRequest, "missing X-API-TOKEN")
			c.Abort()
			return
		}

		// Validate token against whitelist
		valid := false
		for _, t := range config.Cfg.Auth.Tokens {
			if t == token {
				valid = true
				break
			}
		}

		if !valid {
			logger.Warn("auth_invalid_token",
				logger.String("ip", ip),
				logger.String("path", path),
				logger.String("method", method),
			)
			response.Fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Next()
	}
}

// TimeoutMiddleware sets request timeout to prevent hanging requests.
// Uses context cancellation to signal handlers; does NOT run c.Next() in a
// separate goroutine, because gin's Context is not safe for concurrent use
// and writing to an aborted ResponseWriter causes data races.
func TimeoutMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		timeout := 10 * time.Second

		// Use longer timeout for marked endpoints
		if long, ok := c.Get("long_timeout"); ok && long.(bool) {
			timeout = 30 * time.Second
		}

		// Create timeout context derived from the request context.
		// Handlers must respect ctx.Done(); long-running operations should
		// select on the request context.
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// If the context was cancelled (timeout), ensure a proper response is sent
		// only if the handler has not already written a response.
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			logger.Errorf("request_timeout",
				logger.String("ip", c.ClientIP()),
				logger.String("path", c.Request.URL.Path),
				logger.Error(ctx.Err()),
			)
			response.Fail(c, http.StatusRequestTimeout, "request timeout")
			c.Abort()
		}
	}
}

// SetLongTimeout marks an API endpoint to use a longer timeout (30s)
func SetLongTimeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("long_timeout", true)
		c.Next()
	}
}

// StartLimiterCleanup periodically evicts only idle limiters to bound memory.
//
// The previous implementation deleted ALL entries on every tick, which reset
// every active user's burst quota at once — causing synchronized traffic
// spikes. We now keep entries that have been used within limiterIdleTTL and
// only evict truly stale ones.
func StartLimiterCleanup() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("limiter_cleanup_panic", logger.Any("error", r))
			}
		}()

		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			evicted := 0
			now := time.Now().UnixNano()
			cutoff := now - int64(limiterIdleTTL)
			ipLimiters.Range(func(key, value any) bool {
				entry := value.(*limiterEntry)
				if entry.lastUsed.Load() < cutoff {
					ipLimiters.Delete(key)
					evicted++
				}
				return true
			})
			logger.Info("limiter_cleanup_done",
				logger.Int("evicted", evicted),
				logger.Duration("idle_ttl", limiterIdleTTL),
			)
		}
	}()
}

// CorsMiddleware handles cross-origin resource sharing.
// Reflects the request Origin (when present) instead of using "*", because
// "Access-Control-Allow-Origin: *" combined with "Allow-Credentials: true"
// is rejected by browsers for credentialed requests.
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-API-TOKEN, Authorization, Origin")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS request
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
