package main

//
//  @title           b3pulse API
//  @version         1.0
//  @description     B3 trade ingestion & aggregation service.
//  @termsOfService  https://github.com/guttosm/b3pulse
//  @contact.name    API Support
//  @contact.url     https://github.com/guttosm/b3pulse
//  @contact.email   support@example.com
//  @license.name    MIT
//  @license.url     https://opensource.org/licenses/MIT
//  @host            localhost:8080
//  @BasePath        /
//  @schemes         http
//
//  @tag.name        aggregate
//  @tag.description Endpoints for querying ticker aggregates
//
//  @tag.name        health
//  @tag.description Liveness and readiness probes

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/guttosm/b3pulse/config"
	_ "github.com/guttosm/b3pulse/docs" // swagger docs
	"github.com/guttosm/b3pulse/internal/app"
	"github.com/guttosm/b3pulse/internal/ingestion"
	"github.com/guttosm/b3pulse/internal/logger"
)

// startServer initializes and starts the HTTP server in a separate goroutine.
//
// Parameters:
//   - router (http.Handler): The HTTP router (Gin Engine) configured with all routes.
//   - port (string): The port where the server will listen for incoming requests.
//
// Returns:
//   - *http.Server: The initialized HTTP server instance.
func startServer(router http.Handler, port string) *http.Server {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.L().Info().Str("port", port).Msg("server starting")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.L().Fatal().Err(err).Msg("server failed to start")
		}
	}()

	return server
}

// gracefulShutdown gracefully terminates the HTTP server and cleans up resources
// when an OS interrupt signal (SIGINT, SIGTERM) is received.
//
// Parameters:
//   - ctx (context.Context): A context with timeout for graceful shutdown.
//   - server (*http.Server): The HTTP server instance to shut down.
//   - cleanup (func()): Cleanup callback to release resources (e.g., DB connections).
func gracefulShutdown(ctx context.Context, server *http.Server, cleanup func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit
	logger.L().Info().Msg("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.L().Fatal().Err(err).Msg("server forced to shutdown")
	}

	cleanup()
	logger.L().Info().Msg("server exited gracefully")
}

// main is the entry point of the b3pulse application.
//
// Modes (selected via --mode flag):
//   - ingest: Processes the last 7 business days of .txt files from ./data/input/.
//   - api:    Starts the REST API to expose aggregated trade data.
//
// Flags:
//   - --mode: Execution mode ("ingest" or "api"). Default: "ingest".
//   - --dir:  Directory containing .txt input files. Default: "./data/input".
//   - --port: Port for the API server. Defaults to value from config (SERVER_PORT).
func main() {
	ctx := context.Background()

	// Load configuration from environment or .env file
	config.LoadConfig()

	// Initialize JSON logger
	logger.Init()

	// Parse CLI flags (override config defaults if provided)
	mode := flag.String("mode", "ingest", "Mode: ingest or api")
	dir := flag.String("dir", "./data/input", "Directory with .txt files")
	days := flag.Int("days", 7, "Number of last business days to ingest (1-7)")
	parallel := flag.Int("parallel", 0, "How many files to process concurrently (0=auto up to CPU, max 7)")
	force := flag.Bool("force", false, "Reprocess days even if already ingested (deletes existing trades for that day)")
	port := flag.String("port", config.AppConfig.Server.Port, "Port for API mode")
	flag.Parse()

	switch *mode {
	case "ingest":
		// Ingestion mode: process .txt files and persist trades
		logger.L().Info().Msg("running ingestion")
		if *days < 1 {
			*days = 1
		}
		if *days > 7 {
			*days = 7
		}

		// Direct DB connection for ingestion
		db, err := app.InitPostgres(config.AppConfig)
		if err != nil {
			logger.L().Fatal().Err(err).Msg("db connect error")
		}
		defer func() { _ = db.Close() }()

		if err := ingestion.ProcessDirectory(ctx, *dir, db, *days, *parallel, *force); err != nil {
			logger.L().Fatal().Err(err).Msg("ingestion failed")
		}
		logger.L().Info().Msg("ingestion completed successfully")

	case "api":
		// API mode: start the HTTP server
		logger.L().Info().Msg("starting API server")

		router, cleanup, err := app.InitializeApp()
		if err != nil {
			logger.L().Fatal().Err(err).Msg("app init error")
		}

		server := startServer(router, *port)
		gracefulShutdown(ctx, server, cleanup)

	default:
		logger.L().Fatal().Str("mode", *mode).Msg("unknown mode")
	}
}
