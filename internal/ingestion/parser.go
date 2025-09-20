package ingestion

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
	"github.com/guttosm/b3pulse/internal/storage"
)

// expectedHeaders enforces strict column ordering for B3 "Negócios à Vista" files.
// If the header doesn't match EXACTLY (order + count), ingestion must fail.
var expectedHeaders = []string{
	"DataReferencia",
	"CodigoInstrumento",
	"AcaoAtualizacao",
	"PrecoNegocio",
	"QuantidadeNegociada",
	"HoraFechamento",
	"CodigoIdentificadorNegocio",
	"TipoSessaoPregao",
	"DataNegocio",
	"CodigoParticipanteComprador",
	"CodigoParticipanteVendedor",
}

// parseAndPersistFile opens, validates, parses, and persists one file in batches.
// It fails on:
//   - header not matching expected order/length
//   - unrecoverable I/O errors
//
// It tolerates:
//   - empty cells (they become zero values)
//
// Parameters:
//   - ctx:    context for cancellation/timeouts.
//   - path:   file path.
//   - repo:   repository for DB insertion.
//   - batch:  batch size for inserts (e.g., 5000).
func parseAndPersistFile(ctx context.Context, path string, repo storage.TradesRepository, batch int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // allow variable but we’ll check explicitly

	// Validate headers strictly.
	header, err := r.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}
	if len(header) != len(expectedHeaders) {
		return 0, fmt.Errorf("invalid header length: expected %d, got %d", len(expectedHeaders), len(header))
	}
	for i, h := range header {
		if strings.TrimSpace(h) != expectedHeaders[i] {
			return 0, fmt.Errorf("invalid header at col %d: expected %q, got %q", i+1, expectedHeaders[i], h)
		}
	}

	// Parse rows streaming; flush batches to DB.
	buf := make([]models.Trade, 0, batch)
	lineNumber := 1 // header already read

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		if err := repo.InsertTradesBatch(buf); err != nil {
			return err
		}
		buf = buf[:0]
		return nil
	}

	total := 0

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		rec, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, fmt.Errorf("read line after %d: %w", lineNumber, err)
		}
		lineNumber++

		// Enforce structure: exactly 11 columns. If not, fail entire ingestion.
		if len(rec) != len(expectedHeaders) {
			return 0, fmt.Errorf("invalid column count on line %d: expected %d got %d", lineNumber, len(expectedHeaders), len(rec))
		}

		tr, err := recordToTrade(rec)
		if err != nil {
			// Structural/format error → fail the whole pipeline (explicit requirement).
			return 0, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		buf = append(buf, tr)
		total++
		if len(buf) >= batch {
			if err := flush(); err != nil {
				return 0, fmt.Errorf("flush batch ending line %d: %w", lineNumber, err)
			}
		}
	}

	// Final flush
	if err := flush(); err != nil {
		return 0, fmt.Errorf("final flush: %w", err)
	}

	return total, nil
}

// recordToTrade converts a single CSV record (already validated length==11)
// into a models.Trade. It is STRICT about types/format but TOLERATES empty cells,
// mapping them to zero-values.
//
// Column order (Portuguese header → English model fields):
//
//	 0 DataReferencia               → ReferenceDate (DATE, "2006-01-02")
//	 1 CodigoInstrumento            → InstrumentCode (string)
//	 2 AcaoAtualizacao              → UpdateAction (string, keep as-is)
//	 3 PrecoNegocio                 → TradePrice (float, comma→dot, empty→0)
//	 4 QuantidadeNegociada          → TradeQuantity (int64, empty→0)
//	 5 HoraFechamento               → ClosingTime (TIME; HHMMSSmmm → HH:MM:SS; empty→zero)
//	 6 CodigoIdentificadorNegocio   → TradeIdentifierCode (string)
//	 7 TipoSessaoPregao             → SessionType (string, keep as-is)
//	 8 DataNegocio                  → TradeDate (DATE, "2006-01-02")
//	 9 CodigoParticipanteComprador  → BuyerParticipantCode (string)
//	10 CodigoParticipanteVendedor   → SellerParticipantCode (string)
func recordToTrade(rec []string) (models.Trade, error) {
	var t models.Trade

	// ReferenceDate (0) — may be empty
	if s := strings.TrimSpace(rec[0]); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return t, fmt.Errorf("invalid ReferenceDate: %v", err)
		}
		t.ReferenceDate = d
	}

	// InstrumentCode (1)
	t.InstrumentCode = strings.TrimSpace(rec[1])

	// UpdateAction (2) — keep as string to match DB schema
	t.UpdateAction = strings.TrimSpace(rec[2])

	// TradePrice (3) — may be empty, uses comma as decimal separator
	if s := strings.TrimSpace(rec[3]); s != "" {
		s = strings.ReplaceAll(s, ",", ".")
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return t, fmt.Errorf("invalid TradePrice: %v", err)
		}
		t.TradePrice = v
	}

	// TradeQuantity (4) — may be empty
	if s := strings.TrimSpace(rec[4]); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return t, fmt.Errorf("invalid TradeQuantity: %v", err)
		}
		t.TradeQuantity = v
	}

	// ClosingTime (5) — may be empty, often "HHMMSSmmm" → we use only "HHMMSS"
	if s := strings.TrimSpace(rec[5]); s != "" {
		if len(s) < 6 {
			return t, fmt.Errorf("invalid ClosingTime length (need at least HHMMSS): %q", s)
		}
		// Only first 6 digits → HHMMSS
		hhmmss := s[:6]
		h, err := time.Parse("150405", hhmmss)
		if err != nil {
			return t, fmt.Errorf("invalid ClosingTime: %v", err)
		}
		// Keep only the clock part.
		t.ClosingTime = time.Date(0, 1, 1, h.Hour(), h.Minute(), h.Second(), 0, time.UTC)
	}

	// TradeIdentifierCode (6)
	t.TradeIdentifierCode = strings.TrimSpace(rec[6])

	// SessionType (7) — keep as string to match DB schema
	t.SessionType = strings.TrimSpace(rec[7])

	// TradeDate (8) — may be empty
	if s := strings.TrimSpace(rec[8]); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return t, fmt.Errorf("invalid TradeDate: %v", err)
		}
		t.TradeDate = d
	}

	// BuyerParticipantCode (9)
	t.BuyerParticipantCode = strings.TrimSpace(rec[9])

	// SellerParticipantCode (10)
	t.SellerParticipantCode = strings.TrimSpace(rec[10])

	return t, nil
}
