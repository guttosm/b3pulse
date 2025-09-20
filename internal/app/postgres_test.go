package app

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guttosm/b3pulse/config"
)

func TestInitPostgres_OpenError(t *testing.T) {
	old := sqlOpener
	sqlOpener = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, errors.New("open failed")
	}
	t.Cleanup(func() { sqlOpener = old })

	_, err := InitPostgres(config.Config{Postgres: config.PostgresConfig{User: "u", Password: "p", Host: "h", Port: 5432, DBName: "d", SSLMode: "disable"}})
	if err == nil {
		t.Fatalf("expected error from InitPostgres when open fails")
	}
}

func TestInitPostgres_PingError(t *testing.T) {
	old := sqlOpener
	sqlOpener = func(driverName, dataSourceName string) (*sql.DB, error) {
		// Use sqlmock to return a *sql.DB whose Ping fails (enable ping monitoring)
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatalf("sqlmock new: %v", err)
		}
		// Cause Ping to fail
		mock.ExpectPing().WillReturnError(errors.New("ping failed"))
		return db, nil
	}
	t.Cleanup(func() { sqlOpener = old })

	_, err := InitPostgres(config.Config{Postgres: config.PostgresConfig{User: "u", Password: "p", Host: "h", Port: 5432, DBName: "d", SSLMode: "disable"}})
	if err == nil {
		t.Fatalf("expected ping error from InitPostgres")
	}
}
