//go:build integration
// +build integration

package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	goose "github.com/pressly/goose/v3"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/guttosm/b3pulse/config"
	"github.com/guttosm/b3pulse/internal/app"
)

func startPG(t *testing.T) (dsn string, host string, port nat.Port, terminate func()) {
	t.Helper()
	ctx := context.Background()
	req := tc.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "b3pulse",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
		},
		WaitingFor: wait.ForSQL("5432/tcp", "postgres", func(h string, p nat.Port) string {
			return fmt.Sprintf("host=%s port=%s user=postgres password=postgres dbname=b3pulse sslmode=disable", h, p.Port())
		}).WithStartupTimeout(60 * time.Second),
	}
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("container: %v", err)
	}
	h, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	mp, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", "postgres", "postgres", h, mp.Port(), "b3pulse")
	terminate = func() { _ = c.Terminate(context.Background()) }
	return dsn, h, mp, terminate
}

func openAndMigrate(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	path := filepath.Join("..", "..", "db", "migrations")
	if err := goose.Up(db, path); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedForE2E(t *testing.T, db *sql.DB, d time.Time) {
	t.Helper()
	// minimal two rows with same day for volume and price checks
	_, err := db.Exec(`INSERT INTO trades (reference_date, instrument_code, update_action, trade_price, trade_quantity, closing_time, trade_identifier_code, session_type, trade_date, buyer_participant_code, seller_participant_code)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		d, "E2E4", "I", 10.5, 40, time.Date(0, 1, 1, 10, 0, 0, 0, time.UTC), "X", "REG", d, "B", "S")
	if err != nil {
		t.Fatalf("seed1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO trades (reference_date, instrument_code, update_action, trade_price, trade_quantity, closing_time, trade_identifier_code, session_type, trade_date, buyer_participant_code, seller_participant_code)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		d, "E2E4", "I", 12.0, 60, time.Date(0, 1, 1, 10, 0, 0, 0, time.UTC), "Y", "REG", d, "B", "S")
	if err != nil {
		t.Fatalf("seed2: %v", err)
	}
}

func TestAPI_E2E_Aggregate_WithStartDate(t *testing.T) {
	dsn, host, port, term := startPG(t)
	defer term()
	db := openAndMigrate(t, dsn)
	defer db.Close()

	// Seed data on a fixed day
	day := time.Now().UTC().AddDate(0, 0, -2) // two days ago
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	seedForE2E(t, db, day)

	// Point application config to containerized DB
	config.AppConfig.Postgres.Host = host
	p, _ := nat.ParsePort(port.Port())
	config.AppConfig.Postgres.Port = int(p)
	config.AppConfig.Postgres.User = "postgres"
	config.AppConfig.Postgres.Password = "postgres"
	config.AppConfig.Postgres.DBName = "b3pulse"
	config.AppConfig.Postgres.SSLMode = "disable"

	router, cleanup, err := app.InitializeApp()
	if err != nil {
		t.Fatalf("init app: %v", err)
	}
	defer cleanup()

	// Build request with data_inicio = day
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/aggregate?ticker=E2E4&data_inicio="+day.Format("2006-01-02"), nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Ticker         string  `json:"ticker"`
		MaxRangeValue  float64 `json:"max_range_value"`
		MaxDailyVolume int64   `json:"max_daily_volume"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body.Ticker != "E2E4" || body.MaxRangeValue != 12.0 || body.MaxDailyVolume != 100 {
		t.Fatalf("unexpected body: %+v", body)
	}
}
