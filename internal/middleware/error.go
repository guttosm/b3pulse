package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/domain/dto"
)

// ErrorHandler is a Gin middleware that captures any errors registered during
// the request lifecycle and returns a consistent, structured JSON error response.
//
// Usage:
//
//	router.Use(ErrorHandler)
//
// Behavior:
//   - After the request is processed by downstream handlers, it checks for errors via `c.Errors`.
//   - If any errors are present, it takes the first one and builds an ErrorResponse using dto.NewErrorResponse.
//   - It responds with HTTP 500 and JSON body unless the error was already handled (use cautiously with AbortWithError).
func ErrorHandler(c *gin.Context) {
	c.Next()

	if len(c.Errors) > 0 {
		firstErr := c.Errors[0].Err
		errResponse := dto.NewErrorResponse("An unexpected error occurred", firstErr)

		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, errResponse)
		}
	}
}

// AbortWithError aborts the current request and returns a structured JSON error response.
//
// Parameters:
//   - c (*gin.Context): The Gin context.
//   - status (int): The HTTP status code to return (e.g., 400, 401, 500).
//   - msg (string): A user-friendly message describing the error.
//   - err (error): The technical error (optional, can be nil).
//
// Behavior:
//   - Constructs an ErrorResponse with the provided message and error.
//   - Aborts the request immediately and writes the response.
func AbortWithError(c *gin.Context, status int, msg string, err error) {
	errResp := dto.NewErrorResponse(msg, err)
	c.AbortWithStatusJSON(status, errResp)
}
