package app

import (
	"database/sql"
	"fmt"

	"github.com/guttosm/b3pulse/config"

	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
)

// InitPostgres initializes a PostgreSQL connection using the provided configuration.
//
// Parameters:
//   - cfg (config.Config): The application configuration object containing Postgres settings.
//
// Behavior:
//   - Constructs a DSN (Data Source Name) using values from cfg.Postgres.
//   - Opens a database handle with sql.Open.
//   - Immediately pings the database to validate connectivity.
//   - Returns the live connection if successful.
//
// Returns:
//   - *sql.DB: an open database connection pool (safe for concurrent use).
//   - error: if opening or pinging the database fails.
//
// Example usage:
//
//	db, err := app.InitPostgres(config.AppConfig)
//	if err != nil {
//	    log.Fatalf("❌ failed to connect: %v", err)
//	}
//	defer db.Close()
//
// sqlOpener is an indirection for unit testing; defaults to sql.Open
var sqlOpener = sql.Open

func InitPostgres(cfg config.Config) (*sql.DB, error) {
	// Construct PostgreSQL DSN from configuration
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
	)

	// Initialize database handle (does not establish a real connection yet)
	db, err := sqlOpener("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}

	// Verify connectivity by pinging the database
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return db, nil
}

// postgresOpener is an indirection used by InitializeApp; overridden in tests to avoid real connections.
var postgresOpener = InitPostgres
