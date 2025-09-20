package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
)

type fakeRepo struct {
	batches [][]models.Trade
	err     error
}

func (f *fakeRepo) InsertTradesBatch(trades []models.Trade) error {
	f.batches = append(f.batches, append([]models.Trade(nil), trades...))
	return f.err
}
func (f *fakeRepo) GetAggregateByTicker(string, *time.Time, *time.Time) (*models.Aggregate, error) {
	return nil, nil
}
func (f *fakeRepo) HasIngestionForDate(time.Time) (bool, error)     { return false, nil }
func (f *fakeRepo) UpsertIngestionLog(time.Time, string, int) error { return nil }
func (f *fakeRepo) DeleteTradesByDate(time.Time) error              { return nil }

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return p
}

func TestParseAndPersistFile_TableDriven(t *testing.T) {
	dir := t.TempDir()
	validHeader := "DataReferencia;CodigoInstrumento;AcaoAtualizacao;PrecoNegocio;QuantidadeNegociada;HoraFechamento;CodigoIdentificadorNegocio;TipoSessaoPregao;DataNegocio;CodigoParticipanteComprador;CodigoParticipanteVendedor\n"
	validRow := ";PETR4;I;10,50;100;101530000;ABC;REGULAR;2025-09-11;B;S\n"

	cases := []struct {
		name        string
		content     string
		wantErr     bool
		wantBatches int
		wantRows    int
	}{
		{name: "ok single row", content: validHeader + validRow, wantErr: false, wantBatches: 1, wantRows: 1},
		{name: "bad header order", content: "X;Y;Z\n", wantErr: true},
		{name: "bad col count", content: validHeader + "a;b\n", wantErr: true},
		{name: "empty numeric tolerated", content: validHeader + ";PETR4;I;; ;;;;;;\n", wantErr: false, wantBatches: 1, wantRows: 1},
		{name: "invalid price", content: validHeader + ";PETR4;I;abc;100;;;;;;;\n", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempFile(t, dir, "file.txt", tc.content)
			repo := &fakeRepo{}
			n, err := parseAndPersistFile(context.Background(), path, repo, 5)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if n != tc.wantRows {
				t.Fatalf("rows: want %d got %d", tc.wantRows, n)
			}
			if len(repo.batches) != tc.wantBatches {
				t.Fatalf("batches: want %d got %d", tc.wantBatches, len(repo.batches))
			}
		})
	}
}

func TestParseAndPersistFile_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	validHeader := "DataReferencia;CodigoInstrumento;AcaoAtualizacao;PrecoNegocio;QuantidadeNegociada;HoraFechamento;CodigoIdentificadorNegocio;TipoSessaoPregao;DataNegocio;CodigoParticipanteComprador;CodigoParticipanteVendedor\n"
	// many rows to ensure loop would run if not canceled
	rows := ""
	for i := 0; i < 1000; i++ {
		rows += ";PETR4;I;10,50;100;101530000;ABC;REGULAR;2025-09-11;B;S\n"
	}
	path := writeTempFile(t, dir, "big.csv", validHeader+rows)

	repo := &fakeRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately canceled
	if _, err := parseAndPersistFile(ctx, path, repo, 100); err == nil {
		t.Fatalf("expected context canceled error")
	}
}
