//go:build integration
// +build integration

package ingestion

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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
	// migrations path relative to this test file (internal/ingestion → ../../db/migrations)
	path := filepath.Join("..", "..", "db", "migrations")
	if err := goose.Up(db, path); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
}

func writeInputFile(t *testing.T, dir string, day time.Time, rows int) (string, int) {
	t.Helper()
	name := day.Format(fileDateLayout) + fileSuffix
	full := filepath.Join(dir, name)

	f, err := os.Create(full)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()

	// header exactly as expected by parser
	header := "DataReferencia;CodigoInstrumento;AcaoAtualizacao;PrecoNegocio;QuantidadeNegociada;HoraFechamento;CodigoIdentificadorNegocio;TipoSessaoPregao;DataNegocio;CodigoParticipanteComprador;CodigoParticipanteVendedor\n"
	if _, err := f.WriteString(header); err != nil {
		t.Fatalf("write header: %v", err)
	}

	// write N rows with varying price/qty for a single instrument
	// Use comma decimal and HHMMSSmmm time format for HoraFechamento
	// Example line fields:
	// 0: DataReferencia (YYYY-MM-DD)
	// 1: CodigoInstrumento (string)
	// 2: AcaoAtualizacao (string)
	// 3: PrecoNegocio (float with comma)
	// 4: QuantidadeNegociada (int)
	// 5: HoraFechamento (HHMMSSmmm)
	// 6: CodigoIdentificadorNegocio (string)
	// 7: TipoSessaoPregao (string)
	// 8: DataNegocio (YYYY-MM-DD)
	// 9: CodigoParticipanteComprador (string)
	// 10: CodigoParticipanteVendedor (string)
	for i := 0; i < rows; i++ {
		price := 10.0 + float64(i)
		qty := 100 + i
		line := fmt.Sprintf("%s;TEST4;I;%.2f;%d;100000000;X;REG;%s;B;S\n",
			day.Format("2006-01-02"),
			price, // will be converted to using dot; parser replaces comma->dot, but dot also parses
			qty,
			day.Format("2006-01-02"),
		)
		// Replace dot with comma to simulate B3 format
		line = replaceDotWithComma(line)
		if _, err := f.WriteString(line); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}

	return full, rows
}

func replaceDotWithComma(s string) string {
	b := []rune(s)
	for i, r := range b {
		if r == '.' {
			b[i] = ','
		}
	}
	return string(b)
}

func TestIngestion_EndToEnd_ProcessDirectory(t *testing.T) {
	dsn, terminate := startPostgres(t)
	defer terminate()
	db := openDB(t, dsn)
	defer db.Close()
	runMigrations(t, db)

	// Prepare input directory with exactly one required business day file
	tdir := t.TempDir()
	// Compute the specific business day that ProcessDirectory(nDays=1) will expect
	day := LastNBusinessDays(1, time.Now())[0]
	_, wrote := writeInputFile(t, tdir, day, 3)

	// nDays=1 to only look for the single file we wrote
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ProcessDirectory(ctx, tdir, db, 1, 2, false); err != nil {
		t.Fatalf("ProcessDirectory: %v", err)
	}

	// Assert data inserted
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM trades WHERE instrument_code='TEST4' AND trade_date=$1", day).Scan(&cnt); err != nil {
		t.Fatalf("count trades: %v", err)
	}
	if cnt != wrote {
		t.Fatalf("expected %d trades, got %d", wrote, cnt)
	}

	// Assert ingestion log upserted
	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM ingestion_log WHERE file_date=$1)", day).Scan(&exists); err != nil {
		t.Fatalf("check ingestion_log: %v", err)
	}
	if !exists {
		t.Fatalf("expected ingestion_log entry for %s", day.Format("2006-01-02"))
	}
}
