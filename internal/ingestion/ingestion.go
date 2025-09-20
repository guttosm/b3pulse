package ingestion

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/guttosm/b3pulse/internal/logger"
	"github.com/guttosm/b3pulse/internal/storage"
)

const (
	fileDateLayout   = "02-01-2006" // DD-MM-YYYY
	fileSuffix       = "_NEGOCIOSAVISTA.txt"
	defaultBatchSize = 5000
)

// repoCtor is an indirection for creating the repository; tests can override this.
var repoCtor = func(db *sql.DB) storage.TradesRepository {
	return storage.NewTradesRepository(db)
}

//   - dir: directory containing .txt input files.
//   - db:  open *sql.DB (PostgreSQL).
//
// Behavior:
//   - Expects exactly one file per business day with name "DD-MM-YYYY_NEGOCIOSAVISTA.txt".
//   - Uses a concurrency limit based on CPU count (min(7, NumCPU)).
//   - For each file, parses & inserts trades in batches via repository.
//   - If any file returns error, cancels the rest and returns that error.
//
// Returns:
//   - error: first error encountered (if any).
func ProcessDirectory(ctx context.Context, dir string, db *sql.DB, nDays int, parallel int, force bool) error {
	// use indirection to allow tests to swap repository constructor
	repo := repoCtor(db)

	// Build the list of the last 7 business days (Brazil).
	if nDays < 1 {
		nDays = 1
	}
	if nDays > 7 {
		nDays = 7
	}
	dates := LastNBusinessDays(nDays, time.Now())

	// Build expected filenames & validate presence upfront.
	var files []string
	var missing []string

	for _, d := range dates {
		name := d.Format(fileDateLayout) + fileSuffix
		full := filepath.Join(dir, name)
		files = append(files, full)

		if _, err := os.Stat(full); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, name)
			} else {
				return fmt.Errorf("stat failed for %s: %w", full, err)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required files: %s", strings.Join(missing, ", "))
	}

	logger.L().Info().Int("files", len(files)).Str("dir", dir).Msg("ingestion start")

	// Concurrency: default to min(7, NumCPU), or use provided clamp(1..7)
	maxParallel := 7
	if parallel > 0 {
		if parallel > 7 {
			parallel = 7
		}
		maxParallel = parallel
	} else if c := runtime.NumCPU(); c < maxParallel {
		maxParallel = c
	}

	logger.L().Info().Int("max_parallel", maxParallel).Msg("ingestion configured")

	// errgroup will cancel siblings on first error.
	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxParallel)

	for i, file := range files {
		idx := i
		f := file
		sem <- struct{}{}

		g.Go(func() error {
			defer func() { <-sem }()
			start := time.Now()
			base := filepath.Base(f)
			logger.L().Info().Int("idx", idx+1).Int("total", len(files)).Str("file", base).Msg("file start")

			// Determine the business date from the filename (DD-MM-YYYY_...)
			datePart := strings.TrimSuffix(base, fileSuffix)
			d, err := time.Parse(fileDateLayout, datePart)
			if err != nil {
				logger.L().Error().Str("file", base).Err(err).Msg("invalid date in filename")
				return fmt.Errorf("file %s: parse date from filename: %w", f, err)
			}

			// Idempotency: skip if already ingested, unless force
			exists, err := repo.HasIngestionForDate(d)
			if err != nil {
				logger.L().Error().Str("file", base).Err(err).Msg("check ingestion log failed")
				return fmt.Errorf("file %s: check ingestion log: %w", f, err)
			}
			if exists && !force {
				logger.L().Info().Int("idx", idx+1).Int("total", len(files)).Str("file", base).Bool("skipped", true).Msg("already ingested")
				return nil
			}
			if exists && force {
				// Delete existing data for that date and reprocess
				if err := repo.DeleteTradesByDate(d); err != nil {
					logger.L().Error().Str("file", base).Err(err).Msg("delete existing failed")
					return fmt.Errorf("file %s: delete existing: %w", f, err)
				}
			}

			// Process each file; this function:
			// - validates header/order/columns strictly
			// - parses rows tolerantly (empty cells allowed)
			// - inserts in batches (defaultBatchSize)
			total, err := parseAndPersistFile(gctx, f, repo, defaultBatchSize)
			if err != nil {
				logger.L().Error().Str("file", base).Dur("elapsed", time.Since(start)).Err(err).Msg("file failed")
				return fmt.Errorf("file %s: %w", f, err)
			}
			if err := repo.UpsertIngestionLog(d, base, total); err != nil {
				logger.L().Error().Str("file", base).Err(err).Msg("update ingestion log failed")
				return fmt.Errorf("file %s: upsert ingestion log: %w", f, err)
			}
			logger.L().Info().Int("idx", idx+1).Int("total", len(files)).Str("file", base).Int("rows", total).Dur("elapsed", time.Since(start)).Bool("force", force).Msg("file done")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
