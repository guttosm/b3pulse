package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config holds the full application configuration loaded from environment variables or .env file.
//
// It is composed of smaller structs that represent different concerns of the system,
// such as server settings and Postgres database connection details.
//
// Example YAML/ENV equivalent:
//
//	SERVER_PORT=8080
//	POSTGRES_HOST=localhost
//	POSTGRES_PORT=5432
//	POSTGRES_USER=admin
//	POSTGRES_PASSWORD=secret
//	POSTGRES_DB=b3pulse
//	POSTGRES_SSLMODE=disable
type Config struct {
	Server   ServerConfig   // HTTP server configuration
	Postgres PostgresConfig // PostgreSQL connection settings
}

// ServerConfig holds HTTP server settings such as the port to listen on.
type ServerConfig struct {
	Port string // The TCP port the HTTP server will listen on (e.g., "8080")
}

// PostgresConfig defines connection details for PostgreSQL.
//
// Fields:
//   - Host: hostname of the database server.
//   - Port: port number of the database server (default 5432).
//   - User: username for authentication.
//   - Password: password for authentication.
//   - DBName: target database name.
//   - SSLMode: SSL mode (e.g., "disable", "require").
//   - URL: computed DSN used by database/sql to connect.
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	URL      string
}

// AppConfig is the globally accessible configuration instance.
//
// It is populated once via LoadConfig() and used throughout the application.
// All services should import this package and read from AppConfig instead of
// reloading environment variables directly.
var AppConfig Config

// LoadConfig initializes the global AppConfig by reading from .env file
// or directly from environment variables.
//
// Precedence (from lowest to highest):
//  1. Defaults set in this function.
//  2. Values from .env file (if present).
//  3. Environment variables.
//
// Behavior:
//   - Sets defaults for all required fields.
//   - Reads environment variables automatically with viper.AutomaticEnv().
//   - Constructs the PostgreSQL connection string (DSN).
//   - Calls validateConfig() to ensure required fields are present.
//
// Fatal exit:
//   - If required variables are missing, validateConfig() will terminate the app
//     with a descriptive log message.
func LoadConfig() {
	// Default values
	viper.SetDefault("SERVER_PORT", "8080")

	viper.SetDefault("POSTGRES_HOST", "localhost")
	viper.SetDefault("POSTGRES_PORT", 5432)
	viper.SetDefault("POSTGRES_USER", "postgres")
	viper.SetDefault("POSTGRES_PASSWORD", "postgres")
	viper.SetDefault("POSTGRES_DB", "b3pulse")
	viper.SetDefault("POSTGRES_SSLMODE", "disable")

	// Optionally read from .env if present (common in local dev)
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig() // ignore error if no .env

	// Read environment variables automatically
	viper.AutomaticEnv()

	// Populate global config instance
	AppConfig = Config{
		Server: ServerConfig{
			Port: viper.GetString("SERVER_PORT"),
		},
		Postgres: PostgresConfig{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetInt("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			DBName:   viper.GetString("POSTGRES_DB"),
			SSLMode:  viper.GetString("POSTGRES_SSLMODE"),
		},
	}

	// Construct Postgres DSN (used by database/sql)
	AppConfig.Postgres.URL = fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		AppConfig.Postgres.User,
		AppConfig.Postgres.Password,
		AppConfig.Postgres.Host,
		AppConfig.Postgres.Port,
		AppConfig.Postgres.DBName,
		AppConfig.Postgres.SSLMode,
	)

	// Validate critical fields
	validateConfig()
}

// validateConfig ensures required variables are present and terminates
// the application if they are missing.
//
// This avoids unexpected runtime failures due to incomplete configuration.
//
// Behavior:
//   - Checks each critical field of AppConfig.
//   - Collects missing ones in a slice.
//   - If any are missing, logs them and terminates the app with log.Fatalf().
func validateConfig() {
	var missing []string

	if AppConfig.Server.Port == "" {
		missing = append(missing, "SERVER_PORT")
	}
	if AppConfig.Postgres.Host == "" {
		missing = append(missing, "POSTGRES_HOST")
	}
	if AppConfig.Postgres.Port == 0 {
		missing = append(missing, "POSTGRES_PORT")
	}
	if AppConfig.Postgres.User == "" {
		missing = append(missing, "POSTGRES_USER")
	}
	if AppConfig.Postgres.Password == "" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}
	if AppConfig.Postgres.DBName == "" {
		missing = append(missing, "POSTGRES_DB")
	}

	if len(missing) > 0 {
		log.Fatalf("❌ Missing required environment variables: %v\n", missing)
	}
}
