package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/domain/dto"
	"github.com/guttosm/b3pulse/internal/logger"
)

// RecoveryMiddleware returns a Gin middleware that gracefully recovers from any panics,
// logs the stack trace for debugging, and returns a standardized JSON error response.
//
// Behavior:
//   - Uses defer to catch any panic that occurs during request handling.
//   - Prints the recovered panic value and stack trace to stdout (can be adapted to structured logging).
//   - Returns a 500 Internal Server Error response using dto.NewErrorResponse.
//
// Returns:
//   - gin.HandlerFunc: A middleware function for use in Gin router.
//
// Example:
//
//	router := gin.New()
//	router.Use(middleware.RecoveryMiddleware())
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic and stack trace
				logger.L().Error().
					Str("panic", fmt.Sprintf("%v", r)).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered")

				// Respond with standardized error structure
				errResponse := dto.NewErrorResponse("Internal server error", fmt.Errorf("%v", r))
				c.AbortWithStatusJSON(http.StatusInternalServerError, errResponse)
			}
		}()

		c.Next()
	}
}
