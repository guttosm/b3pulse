package logger

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"ERR", zerolog.ErrorLevel},
		{"", zerolog.InfoLevel},
		{"something", zerolog.InfoLevel},
	}
	for _, c := range cases {
		if got := parseLevel(c.in); got != c.want {
			t.Fatalf("parseLevel(%q)=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestGetenv(t *testing.T) {
	t.Setenv("X", "val")
	if v := getenv("X", "def"); v != "val" {
		t.Fatalf("getenv returned %q, want 'val'", v)
	}
	if v := getenv("Y", "def"); v != "def" {
		t.Fatalf("getenv returned %q, want 'def'", v)
	}
}

func TestInitAndL(t *testing.T) {
	// Info by default
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("LOG_PRETTY")
	Init()
	if L() == nil {
		t.Fatalf("L() returned nil")
	}

	// Set debug level and pretty
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_PRETTY", "true")
	Init()
	if L().GetLevel() != zerolog.DebugLevel {
		t.Fatalf("expected debug level, got %v", L().GetLevel())
	}
}

// Ensure L() never returns nil and initializes level if not set
func TestLoggerAccessor_NotNil(t *testing.T) {
	// Reset base to zero value to force Init path
	base = zerolog.Logger{}
	lg := L()
	if lg == nil {
		t.Fatalf("logger is nil")
	}
	// After calling L(), the level should not be NoLevel
	if lg.GetLevel() == zerolog.NoLevel {
		t.Fatalf("logger level not initialized")
	}
}
