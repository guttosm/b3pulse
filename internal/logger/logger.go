package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var (
	base zerolog.Logger
)

// Init configures the global JSON logger.
//
// Environment variables (optional):
//   - LOG_LEVEL: debug|info|warn|error (default: info)
//   - LOG_PRETTY: true|false (default: false)
func Init() {
	level := parseLevel(getenv("LOG_LEVEL", "info"))
	pretty := strings.EqualFold(getenv("LOG_PRETTY", "false"), "true")

	zerolog.TimeFieldFormat = time.RFC3339Nano
	var w io.Writer = os.Stdout
	if pretty {
		cw := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		w = cw
	}
	l := zerolog.New(w).With().Timestamp().Logger().Level(level)
	base = l
}

// L returns the global logger. Call Init() once on startup.
func L() *zerolog.Logger {
	if base.GetLevel() == zerolog.NoLevel {
		Init()
	}
	return &base
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zerolog.DebugLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error", "err":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
