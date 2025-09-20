package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// NewRouter creates a Gin engine with routes configured.
// It receives a Handler instance with all business logic already injected.
//
// Responsibilities:
//   - Registers global middlewares (RequestID, Logger, Recovery, RateLimiter).
//   - Adds request timeout handling (10 seconds).
//   - Mounts Swagger docs (/swagger/*any).
//   - Configures API v1 routes (/api/v1).
//
// Note:
//   - Health and readiness endpoints (/healthz, /readyz) are registered in app.InitializeApp().
//
// Parameters:
//   - handler (*Handler): The HTTP handler with business logic.
//
// Returns:
//   - *gin.Engine: Configured Gin router.
func NewRouter(handler *Handler) *gin.Engine {
	router := gin.New()

	// ─── Middlewares ───────────────────────────────
	router.Use(
		middleware.RequestID(),
		middleware.RequestLogger(),
		middleware.RecoveryMiddleware(),
		middleware.ErrorHandler,
		middleware.RateLimiter(),
	)

	// ─── Timeout ──────────────────────────────────
	router.Use(func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	// ─── Swagger ──────────────────────────────────
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// ─── API v1 ───────────────────────────────────
	v1 := router.Group("/api/v1")
	{
		v1.GET("/aggregate", handler.GetAggregate)
	}

	return router
}
