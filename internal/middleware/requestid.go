package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "request_id"

// RequestID is a Gin middleware that injects a unique identifier
// for each incoming HTTP request.
//
// Behavior:
//   - Generates a new UUID (v4).
//   - Stores it in the Gin context under the key "request_id".
//   - Adds it to the response headers as "X-Request-ID".
//   - Ensures traceability of requests across logs and clients.
//
// Usage:
//
//	router := gin.New()
//	router.Use(middleware.RequestID())
//
// Example log usage:
//
//	rid, _ := c.Get(middleware.RequestIDKey)
//	log.Printf("request_id=%s some log message", rid)
//
// Returns:
//   - gin.HandlerFunc: the middleware function.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate new UUID for each request
		id := uuid.NewString()

		// Store in context for downstream usage
		c.Set(RequestIDKey, id)

		// Expose in response headers for clients
		c.Writer.Header().Set("X-Request-ID", id)

		// Continue with the next handlers
		c.Next()
	}
}
