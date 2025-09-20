package middleware

import (
	"net/http"
	"time"

	"sync"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/logger"
)

// RequestLogger is a Gin middleware that logs method, path, status code,
// request latency, and request ID (if available).
//
// Behavior:
//   - Captures start time before request handling.
//   - After request is processed, calculates latency.
//   - Logs method, path, status, latency in ms, and request_id (if injected by RequestID()).
//
// Usage:
//
//	router := gin.New()
//	router.Use(middleware.RequestID(), middleware.RequestLogger())
//
// Example log output:
//
//	request_id=123e4567-e89b-12d3-a456-426614174000 method=GET path=/api/v1/aggregate status=200 latency_ms=15
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		method := c.Request.Method
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Compute latency and get status
		latency := time.Since(start)
		status := c.Writer.Status()

		// Get request_id if available
		rid, _ := c.Get(RequestIDKey)

		// Structured JSON log
		logger.L().Info().
			Str("request_id", toString(rid)).
			Str("method", method).
			Str("path", path).
			Int("status", status).
			Int64("latency_ms", latency.Milliseconds()).
			Str("client_ip", c.ClientIP()).
			Msg("http_request")
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// client represents a rate-limited client with request count and last seen timestamp.
type client struct {
	lastSeen time.Time
	count    int
}

// Global in-memory store for rate limiting.
// NOTE: In production, consider Redis or another distributed store for multi-instance deployments.
var (
	clients         = make(map[string]*client)
	window          = time.Minute
	limit           = 60
	rateLimiterLock sync.Mutex
)

// RateLimiter is a simple in-memory middleware that limits the number of requests per client IP.
//
// Behavior:
//   - Allows up to `limit` requests per `window` (default: 60 requests per 1 minute).
//   - Identifies clients by their IP address.
//   - If limit exceeded, returns HTTP 429 Too Many Requests.
//
// Usage:
//
//	router := gin.New()
//	router.Use(middleware.RateLimiter())
//
// Response when limit exceeded:
//
//	HTTP/1.1 429 Too Many Requests
//	{
//	    "error": "rate limit exceeded"
//	}
func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		// simple mutex using gin context to avoid global data race without introducing external deps
		// NOTE: for high concurrency or multi-instance, use Redis or sync.Map with a proper mutex.
		// Here we'll protect the map via a channel-like critical section using a package-level mutex.
		rateLimiterLock.Lock()
		cl, ok := clients[ip]
		if !ok || now.Sub(cl.lastSeen) > window {
			cl = &client{lastSeen: now, count: 1}
			clients[ip] = cl
		} else {
			cl.count++
			cl.lastSeen = now
		}
		exceeded := cl.count > limit
		rateLimiterLock.Unlock()

		if exceeded {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		c.Next()
	}
}
