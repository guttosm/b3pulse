package api

import "github.com/gin-gonic/gin"

// HealthHandler provides liveness and readiness endpoints for the service.
//
// Responsibilities:
//   - /healthz: Basic liveness probe (always returns 200 OK).
//   - /readyz: Readiness probe (depends on database connectivity).
type HealthHandler struct {
	dbPing func() error // Function to check database connectivity
}

// NewHealthHandler constructs a HealthHandler with the provided dbPing function.
//
// Parameters:
//   - dbPing (func() error): A function used to check if the database is reachable.
//     Typically, this is db.Ping from *sql.DB.
//
// Returns:
//   - *HealthHandler: A new handler instance.
func NewHealthHandler(dbPing func() error) *HealthHandler {
	return &HealthHandler{dbPing: dbPing}
}

// Register mounts the health and readiness endpoints into the provided Gin router.
//
// Routes:
//   - GET /healthz: Always returns 200 OK.
//   - GET /readyz: Returns 200 OK if dbPing succeeds, 503 if database is not reachable.
//
// Parameters:
//   - r (*gin.Engine): The Gin router to register routes on.
func (h *HealthHandler) Register(r *gin.Engine) {
	// Liveness probe (just checks if the service is up)
	// @Summary      Liveness probe
	// @Description  Always returns OK if the service is running
	// @Tags         health
	// @Produce      json
	// @Success      200  {object}  map[string]string
	// @Router       /healthz [get]
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Readiness probe (checks DB connection)
	// @Summary      Readiness probe
	// @Description  Returns ready if the service dependencies (DB) are reachable
	// @Tags         health
	// @Produce      json
	// @Success      200  {object}  map[string]string
	// @Failure      503  {object}  map[string]string
	// @Router       /readyz [get]
	r.GET("/readyz", func(c *gin.Context) {
		if h.dbPing != nil && h.dbPing() != nil {
			c.JSON(503, gin.H{"status": "degraded"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})
}
