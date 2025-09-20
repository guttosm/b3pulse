package app

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/config"
	"github.com/guttosm/b3pulse/internal/api"
	"github.com/guttosm/b3pulse/internal/service"
	"github.com/guttosm/b3pulse/internal/storage"
)

// InitializeApp sets up all application dependencies and returns
// a fully configured Gin router, a cleanup function for graceful shutdown,
// and any error encountered during initialization.
//
// Responsibilities:
//   - Connects to PostgreSQL using InitPostgres().
//   - Initializes the repository layer (TradesRepository).
//   - Creates the HTTP handler layer to handle requests.
//   - Configures the Gin router with all API routes.
//   - Registers health and readiness probes.
//   - Provides a cleanup function to close resources (e.g., DB connection).
//
// Returns:
//   - *gin.Engine: the configured Gin HTTP router.
//   - func(): cleanup function to be executed on shutdown.
//   - error: any initialization error that occurred.
func InitializeApp() (*gin.Engine, func(), error) {
	// Load global configuration
	cfg := config.AppConfig

	// Connect to PostgreSQL
	// indirection for unit testing
	db, err := postgresOpener(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize postgres: %w", err)
	}

	// Initialize repository layer (responsible for DB access)
	repo := storage.NewTradesRepository(db)

	// Initialize service layer (business logic)
	svc := service.NewAggregateService(repo)

	// Initialize HTTP handler layer (business logic to HTTP mapping)
	handler := api.NewHandler(svc)

	// Setup Gin router with routes
	router := api.NewRouter(handler)

	// Register health and readiness probes
	healthHandler := api.NewHealthHandler(db.Ping)
	healthHandler.Register(router)

	// Cleanup resources on shutdown
	cleanup := func() {
		_ = db.Close()
	}

	return router, cleanup, nil
}
