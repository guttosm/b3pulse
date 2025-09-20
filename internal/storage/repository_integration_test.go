//go:build integration
// +build integration

package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	goose "github.com/pressly/goose/v3"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPostgres spins up a Postgres container and returns a DSN and terminate func.
func startPostgres(t *testing.T) (dsn string, terminate func()) {
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
		WaitingFor: wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
			return fmt.Sprintf("host=%s port=%s user=postgres password=postgres dbname=b3pulse sslmode=disable", host, port.Port())
		}).WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("container start: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}

	dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", "postgres", "postgres", host, port.Port(), "b3pulse")
	terminate = func() { _ = container.Terminate(context.Background()) }
	return dsn, terminate
}

func openDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

func runMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	// migrations path relative to this test file (internal/storage → ../../db/migrations)
	path := filepath.Join("..", "..", "db", "migrations")
	if err := goose.Up(db, path); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
}

func seedTrades(t *testing.T, db *sql.DB) (dates []time.Time) {
	t.Helper()
	// Insert multiple days for ticker TEST4
	base := time.Date(2025, 9, 11, 0, 0, 0, 0, time.UTC)
	dates = []time.Time{base, base.AddDate(0, 0, 1), base.AddDate(0, 0, 2)} // 11,12,13

	exec := func(price float64, qty int64, d time.Time) {
		_, err := db.Exec(`
            INSERT INTO trades (
                reference_date, instrument_code, update_action, trade_price, trade_quantity,
                closing_time, trade_identifier_code, session_type, trade_date,
                buyer_participant_code, seller_participant_code
            ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        `,
			d, "TEST4", "I", price, qty,
			time.Date(0, 1, 1, 10, 0, 0, 0, time.UTC), "X", "REG", d, "B", "S",
		)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Day 1: two trades, total qty 100, prices up to 11.0
	exec(10.5, 40, dates[0])
	exec(11.0, 60, dates[0])
	// Day 2: volume 200, max price 9.0
	exec(9.0, 200, dates[1])
	// Day 3: volume 150, max price 12.0 (overall max price)
	exec(12.0, 150, dates[2])

	return dates
}

func TestRepository_Integration_TableDriven(t *testing.T) {
	dsn, terminate := startPostgres(t)
	defer terminate()
	db := openDB(t, dsn)
	defer db.Close()
	runMigrations(t, db)
	dates := seedTrades(t, db)

	repo := NewTradesRepository(db)

	// Table-driven cases for GetAggregateByTicker
	cases := []struct {
		name         string
		start        *time.Time
		end          *time.Time
		wantMaxPrice float64
		wantMaxDaily int64
	}{
		{
			name:         "all dates",
			start:        nil,
			end:          nil,
			wantMaxPrice: 12.0, // from day3
			wantMaxDaily: 200,  // from day2 volume
		},
		{
			name:         "last 2 days only",
			start:        &dates[1], // from day2 onward
			end:          nil,
			wantMaxPrice: 12.0, // still day3
			wantMaxDaily: 200,  // day2
		},
		{
			name:         "last day only",
			start:        &dates[2], // only day3
			end:          nil,
			wantMaxPrice: 12.0,
			wantMaxDaily: 150,
		},
		{
			name:         "upper-bound excludes day3",
			start:        &dates[0],
			end:          &dates[1], // up to day2 only
			wantMaxPrice: 11.0,      // day1 max price was 11.0
			wantMaxDaily: 200,       // day2 volume
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agg, err := repo.GetAggregateByTicker("TEST4", tc.start, tc.end)
			if err != nil {
				t.Fatalf("GetAggregateByTicker err: %v", err)
			}
			if agg == nil {
				t.Fatalf("nil aggregate")
			}
			if agg.MaxRangeValue != tc.wantMaxPrice || agg.MaxDailyVolume != tc.wantMaxDaily {
				t.Fatalf("got (price=%.2f, vol=%d), want (price=%.2f, vol=%d)", agg.MaxRangeValue, agg.MaxDailyVolume, tc.wantMaxPrice, tc.wantMaxDaily)
			}
		})
	}

	// Ingestion log upsert + exists
	t.Run("ingestion log upsert+exists", func(t *testing.T) {
		day := dates[0]
		if err := repo.UpsertIngestionLog(day, "file1.txt", 123); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		ok, err := repo.HasIngestionForDate(day)
		if err != nil || !ok {
			t.Fatalf("exists want true, got ok=%v err=%v", ok, err)
		}
	})

	// Delete by date
	t.Run("delete by date", func(t *testing.T) {
		day := dates[1]
		if err := repo.DeleteTradesByDate(day); err != nil {
			t.Fatalf("delete: %v", err)
		}
		var cnt int
		if err := db.QueryRow("SELECT COUNT(*) FROM trades WHERE trade_date=$1", day).Scan(&cnt); err != nil {
			t.Fatalf("count: %v", err)
		}
		if cnt != 0 {
			t.Fatalf("expected 0 rows after delete, got %d", cnt)
		}
	})
}
