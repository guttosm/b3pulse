package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/guttosm/b3pulse/internal/domain/models"
	pq "github.com/lib/pq"
)

// TradesRepository defines contract for DB operations.
type TradesRepository interface {
	InsertTradesBatch(trades []models.Trade) error
	GetAggregateByTicker(ticker string, startDate *time.Time, endDate *time.Time) (*models.Aggregate, error)
	HasIngestionForDate(date time.Time) (bool, error)
	UpsertIngestionLog(date time.Time, filename string, rowCount int) error
	DeleteTradesByDate(date time.Time) error
}

type tradesRepository struct {
	db *sql.DB
}

func NewTradesRepository(db *sql.DB) TradesRepository {
	return &tradesRepository{db: db}
}

// InsertTradesBatch inserts multiple trades into DB in a single transaction.
func (r *tradesRepository) InsertTradesBatch(trades []models.Trade) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	// Small optimization for bulk load
	if _, err := tx.Exec(`SET LOCAL synchronous_commit = OFF`); err != nil {
		_ = tx.Rollback()
		return err
	}

	stmt, err := tx.Prepare(pq.CopyIn(
		"trades",
		"reference_date",
		"instrument_code",
		"update_action",
		"trade_price",
		"trade_quantity",
		"closing_time",
		"trade_identifier_code",
		"session_type",
		"trade_date",
		"buyer_participant_code",
		"seller_participant_code",
	))
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// helper to map zero-value times/dates to NULL (nil)
	toNullDate := func(d time.Time) interface{} {
		if d.IsZero() {
			return nil
		}
		return d
	}
	toNullTime := func(t time.Time) interface{} {
		if t.IsZero() {
			return nil
		}
		return t
	}

	for _, rec := range trades {
		if _, err := stmt.Exec(
			toNullDate(rec.ReferenceDate),
			rec.InstrumentCode,
			rec.UpdateAction,
			rec.TradePrice,
			rec.TradeQuantity,
			toNullTime(rec.ClosingTime),
			rec.TradeIdentifierCode,
			rec.SessionType,
			toNullDate(rec.TradeDate),
			rec.BuyerParticipantCode,
			rec.SellerParticipantCode,
		); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := stmt.Exec(); err != nil {
		_ = stmt.Close()
		_ = tx.Rollback()
		return err
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// HasIngestionForDate checks if an ingestion was already recorded for a given business day.
func (r *tradesRepository) HasIngestionForDate(date time.Time) (bool, error) {
	var exists bool
	// ingestion_log.file_date is the canonical per-file day
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM ingestion_log WHERE file_date = $1)`, date).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// UpsertIngestionLog records (or updates) an ingestion entry for a given day.
func (r *tradesRepository) UpsertIngestionLog(date time.Time, filename string, rowCount int) error {
	_, err := r.db.Exec(`
		INSERT INTO ingestion_log (file_date, filename, row_count)
		VALUES ($1, $2, $3)
		ON CONFLICT (file_date)
		DO UPDATE SET filename = EXCLUDED.filename,
					  row_count = EXCLUDED.row_count,
					  ingested_at = NOW()
	`, date, filename, rowCount)
	return err
}

// DeleteTradesByDate removes all trades for a given trade_date.
func (r *tradesRepository) DeleteTradesByDate(date time.Time) error {
	_, err := r.db.Exec(`DELETE FROM trades WHERE trade_date = $1`, date)
	return err
}

// GetAggregateByTicker returns max price and max daily volume for a ticker.
func (r *tradesRepository) GetAggregateByTicker(ticker string, startDate *time.Time, endDate *time.Time) (*models.Aggregate, error) {
	var agg models.Aggregate
	agg.Ticker = ticker

	// Build dynamic conditions for date range filters.
	// $1 is always ticker. Subsequent placeholders depend on provided dates.
	conditions := "instrument_code = $1"
	var args []interface{}
	args = append(args, ticker)
	if startDate != nil {
		placeholder := len(args) + 1 // next positional param index
		conditions += fmt.Sprintf(" AND trade_date >= $%d", placeholder)
		args = append(args, *startDate)
	}
	if endDate != nil {
		placeholder := len(args) + 1
		conditions += fmt.Sprintf(" AND trade_date <= $%d", placeholder)
		args = append(args, *endDate)
	}

	query := fmt.Sprintf(`
		WITH daily AS (
			SELECT trade_date, SUM(trade_quantity) AS daily_volume
			FROM trades
			WHERE %s
			GROUP BY trade_date
		)
		SELECT 
			(SELECT MAX(trade_price) FROM trades WHERE %s) AS max_price,
			(SELECT MAX(daily_volume) FROM daily) AS max_volume
	`, conditions, conditions)

	var maxPrice sql.NullFloat64
	var maxVolume sql.NullInt64

	err := r.db.QueryRow(query, args...).Scan(&maxPrice, &maxVolume)
	if err != nil {
		return nil, err
	}

	// If both are NULL, there is no data for this ticker/date range.
	if !maxPrice.Valid && !maxVolume.Valid {
		return nil, nil
	}

	if maxPrice.Valid {
		agg.MaxRangeValue = maxPrice.Float64
	}
	if maxVolume.Valid {
		agg.MaxDailyVolume = maxVolume.Int64
	}

	return &agg, nil
}
