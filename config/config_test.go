package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestLoadConfig_Defaults verifies that defaults are loaded and DSN is constructed.
func TestLoadConfig_Defaults(t *testing.T) {
	// Clear relevant env vars to ensure defaults are used
	_ = os.Unsetenv("SERVER_PORT")
	_ = os.Unsetenv("POSTGRES_HOST")
	_ = os.Unsetenv("POSTGRES_PORT")
	_ = os.Unsetenv("POSTGRES_USER")
	_ = os.Unsetenv("POSTGRES_PASSWORD")
	_ = os.Unsetenv("POSTGRES_DB")
	_ = os.Unsetenv("POSTGRES_SSLMODE")

	LoadConfig()

	if AppConfig.Server.Port != "8080" {
		t.Fatalf("expected default SERVER_PORT=8080, got %q", AppConfig.Server.Port)
	}
	if AppConfig.Postgres.Host != "localhost" || AppConfig.Postgres.Port != 5432 || AppConfig.Postgres.User != "postgres" || AppConfig.Postgres.Password != "postgres" || AppConfig.Postgres.DBName != "b3pulse" || AppConfig.Postgres.SSLMode != "disable" {
		t.Fatalf("unexpected defaults: %+v", AppConfig.Postgres)
	}
	// DSN must contain expected parts
	dsn := AppConfig.Postgres.URL
	mustHave := []string{"postgres://postgres:postgres@localhost:5432/b3pulse?sslmode=disable"}
	for _, m := range mustHave {
		if !strings.Contains(dsn, m) {
			t.Fatalf("dsn %q does not contain %q", dsn, m)
		}
	}
}

// TestValidateConfig_Fatal uses a subprocess to assert that validateConfig triggers a fatal exit
// when required fields are missing.
func TestValidateConfig_Fatal(t *testing.T) {
	if os.Getenv("RUN_VALIDATE_FATAL") == "1" {
		// In child process: set empty AppConfig and call validateConfig() to trigger log.Fatalf (os.Exit)
		AppConfig = Config{}
		validateConfig()
		t.Fatalf("validateConfig should have exited the process")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "TestValidateConfig_Fatal")
	cmd.Env = append(os.Environ(), "RUN_VALIDATE_FATAL=1")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected process to exit with error, got nil")
	}
}
