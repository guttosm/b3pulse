package app

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guttosm/b3pulse/config"
)

// TestInitPostgres_InvalidHost expects ping failure.
func TestInitPostgres_InvalidHost(t *testing.T) {
	cfg := config.Config{Postgres: config.PostgresConfig{
		Host:     "127.0.0.1",
		Port:     54329, // unlikely mapped
		User:     "x",
		Password: "y",
		DBName:   "z",
		SSLMode:  "disable",
	}}
	db, err := InitPostgres(cfg)
	if err == nil {
		_ = db.Close()
		t.Fatalf("expected error connecting to invalid DB")
	}
}

// TestInitializeApp_DBFailure ensures InitializeApp returns error when DB cannot connect.
func TestInitializeApp_DBFailure(t *testing.T) {
	// Backup and override global config
	old := config.AppConfig
	t.Cleanup(func() { config.AppConfig = old })
	config.AppConfig = config.Config{Postgres: config.PostgresConfig{
		Host:     "127.0.0.1",
		Port:     54329,
		User:     "x",
		Password: "y",
		DBName:   "z",
		SSLMode:  "disable",
	}}

	r, cleanup, err := InitializeApp()
	if err == nil || r != nil || cleanup != nil {
		if cleanup != nil {
			cleanup()
		}
		if r != nil {
			// close engine-related resources if any
			_ = (&sql.DB{})
		}
		t.Fatalf("expected error from InitializeApp with invalid DB config")
	}
}

func TestInitializeApp_HappyPath(t *testing.T) {
	// Override opener to return a sqlmock DB that pings successfully
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	// Expect a ping during InitializeApp's health handler (db.Ping used elsewhere as well)
	mock.ExpectPing()

	old := postgresOpener
	postgresOpener = func(cfg config.Config) (*sql.DB, error) { return db, nil }
	t.Cleanup(func() {
		postgresOpener = old
		_ = db.Close()
	})

	router, cleanup, err := InitializeApp()
	if err != nil || router == nil || cleanup == nil {
		t.Fatalf("InitializeApp failed: err set or nil components")
	}

	// Hit health endpoints
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz status=%d", w.Code)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("readyz status=%d", w2.Code)
	}

	// Call cleanup and ensure it doesn't panic
	cleanup()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
