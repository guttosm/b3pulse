package ingestion

// Consolidated unit tests for ingestion.go (migrated from ingestion_process_test.go and ingestion_more_test.go)

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
	"github.com/guttosm/b3pulse/internal/storage"
)

// fakeRepoIngestion implements minimal TradesRepository for ProcessDirectory tests.
type fakeRepoIngestion struct {
	has      map[time.Time]bool
	inserted int
	deleted  map[time.Time]bool
}

func (f *fakeRepoIngestion) InsertTradesBatch(trades []models.Trade) error {
	f.inserted += len(trades)
	return nil
}
func (f *fakeRepoIngestion) GetAggregateByTicker(string, *time.Time, *time.Time) (*models.Aggregate, error) {
	return nil, nil
}
func (f *fakeRepoIngestion) HasIngestionForDate(date time.Time) (bool, error) {
	return f.has[date], nil
}
func (f *fakeRepoIngestion) UpsertIngestionLog(date time.Time, filename string, rowCount int) error {
	if f.has == nil {
		f.has = map[time.Time]bool{}
	}
	f.has[date] = true
	return nil
}
func (f *fakeRepoIngestion) DeleteTradesByDate(date time.Time) error {
	if f.deleted == nil {
		f.deleted = map[time.Time]bool{}
	}
	f.deleted[date] = true
	return nil
}

// dummyDB satisfies *sql.DB usage but is nil internally; we never call db methods directly in tests due to repoCtor override.
func dummyDB() *sql.DB { return (*sql.DB)(nil) }

func writeFile(t *testing.T, dir, name string, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// file for a business day with valid header and 2 rows
func sampleFile() string {
	return "DataReferencia;CodigoInstrumento;AcaoAtualizacao;PrecoNegocio;QuantidadeNegociada;HoraFechamento;CodigoIdentificadorNegocio;TipoSessaoPregao;DataNegocio;CodigoParticipanteComprador;CodigoParticipanteVendedor\n" +
		"2025-09-18;E2E4;I;10,0;50;100000000;X;REG;2025-09-18;B;S\n" +
		"2025-09-18;E2E4;I;11,0;50;100000000;X;REG;2025-09-18;B;S\n"
}

func TestProcessDirectory_SkipIfAlreadyIngested(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	days := LastNBusinessDays(1, today)
	if len(days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(days))
	}
	dayUTC := time.Date(days[0].Year(), days[0].Month(), days[0].Day(), 0, 0, 0, 0, time.UTC)
	fname := days[0].Format(fileDateLayout) + fileSuffix
	writeFile(t, dir, fname, sampleFile())

	fr := &fakeRepoIngestion{has: map[time.Time]bool{dayUTC: true}}
	old := repoCtor
	repoCtor = func(_ *sql.DB) storage.TradesRepository { return fr }
	t.Cleanup(func() { repoCtor = old })

	if err := ProcessDirectory(context.Background(), dir, dummyDB(), 1, runtime.NumCPU(), false); err != nil {
		t.Fatalf("ProcessDirectory err: %v", err)
	}
	if fr.inserted != 0 {
		t.Fatalf("expected no inserts when already ingested, got %d", fr.inserted)
	}
}

func TestProcessDirectory_ForceReprocess(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	days := LastNBusinessDays(1, today)
	dayUTC := time.Date(days[0].Year(), days[0].Month(), days[0].Day(), 0, 0, 0, 0, time.UTC)
	fname := days[0].Format(fileDateLayout) + fileSuffix
	writeFile(t, dir, fname, sampleFile())

	fr := &fakeRepoIngestion{has: map[time.Time]bool{dayUTC: true}}
	old := repoCtor
	repoCtor = func(_ *sql.DB) storage.TradesRepository { return fr }
	t.Cleanup(func() { repoCtor = old })

	if err := ProcessDirectory(context.Background(), dir, dummyDB(), 1, 1, true); err != nil {
		t.Fatalf("ProcessDirectory err: %v", err)
	}
	if !fr.deleted[dayUTC] {
		t.Fatalf("expected delete for %v", dayUTC)
	}
	if fr.inserted != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", fr.inserted)
	}
}

// minimal fake repo to inject specific errors
type errRepo struct {
	hasErr    error
	upsertErr error
}

func (e *errRepo) InsertTradesBatch([]models.Trade) error { return nil }
func (e *errRepo) GetAggregateByTicker(string, *time.Time, *time.Time) (*models.Aggregate, error) {
	return nil, nil
}
func (e *errRepo) HasIngestionForDate(time.Time) (bool, error) {
	if e.hasErr != nil {
		return false, e.hasErr
	}
	return false, nil
}
func (e *errRepo) UpsertIngestionLog(time.Time, string, int) error { return e.upsertErr }
func (e *errRepo) DeleteTradesByDate(time.Time) error              { return nil }

func TestProcessDirectory_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	// no files created => should report missing
	err := ProcessDirectory(context.Background(), dir, (*sql.DB)(nil), 1, runtime.NumCPU(), false)
	if err == nil || !strings.Contains(err.Error(), "missing required files") {
		t.Fatalf("expected missing files error, got %v", err)
	}
}

func TestProcessDirectory_HasIngestionError(t *testing.T) {
	dir := t.TempDir()
	// create expected file for last business day
	d := LastNBusinessDays(1, time.Now())[0]
	fname := d.Format(fileDateLayout) + fileSuffix
	path := filepath.Join(dir, fname)
	// minimal valid content (header only)
	content := "DataReferencia;CodigoInstrumento;AcaoAtualizacao;PrecoNegocio;QuantidadeNegociada;HoraFechamento;CodigoIdentificadorNegocio;TipoSessaoPregao;DataNegocio;CodigoParticipanteComprador;CodigoParticipanteVendedor\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	old := repoCtor
	repoCtor = func(_ *sql.DB) storage.TradesRepository { return &errRepo{hasErr: context.DeadlineExceeded} }
	t.Cleanup(func() { repoCtor = old })

	if err := ProcessDirectory(context.Background(), dir, (*sql.DB)(nil), 1, 1, false); err == nil {
		t.Fatalf("expected error from HasIngestionForDate")
	}
}

func TestProcessDirectory_UpsertLogError(t *testing.T) {
	dir := t.TempDir()
	d := LastNBusinessDays(1, time.Now())[0]
	fname := d.Format(fileDateLayout) + fileSuffix
	path := filepath.Join(dir, fname)
	// valid file with one row
	content := "DataReferencia;CodigoInstrumento;AcaoAtualizacao;PrecoNegocio;QuantidadeNegociada;HoraFechamento;CodigoIdentificadorNegocio;TipoSessaoPregao;DataNegocio;CodigoParticipanteComprador;CodigoParticipanteVendedor\n" +
		";PETR4;I;10,0;1;100000000;X;REG;" + d.Format("2006-01-02") + ";B;S\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	old := repoCtor
	repoCtor = func(_ *sql.DB) storage.TradesRepository { return &errRepo{upsertErr: context.Canceled} }
	t.Cleanup(func() { repoCtor = old })

	if err := ProcessDirectory(context.Background(), dir, (*sql.DB)(nil), 1, 1, false); err == nil {
		t.Fatalf("expected error from UpsertIngestionLog")
	}
}
