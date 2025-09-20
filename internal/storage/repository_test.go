package storage

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guttosm/b3pulse/internal/domain/models"
)

type dummyErr struct{}

func (dummyErr) Error() string { return "dummy" }

func newMockRepo(t *testing.T) (*tradesRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	repo := &tradesRepository{db: db}
	cleanup := func() { _ = db.Close() }
	return repo, mock, cleanup
}

func TestGetAggregateByTicker_SQLMock(t *testing.T) {
	repo, mock, done := newMockRepo(t)
	defer done()

	// Common regex to avoid brittle query matching; focus on the final SELECT shape
	selectRegex := regexp.MustCompile(`SELECT\s+\(SELECT MAX\(trade_price\) FROM trades WHERE .*\) AS max_price,\s*\(SELECT MAX\(daily_volume\) FROM daily\) AS max_volume`)

	day := time.Date(2025, 9, 12, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2025, 9, 13, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		start     *time.Time
		end       *time.Time
		argsCount int
		maxPrice  interface{}
		maxVolume interface{}
	}{
		{name: "no dates", start: nil, end: nil, argsCount: 1, maxPrice: 12.3, maxVolume: int64(200)},
		{name: "with start", start: &day, end: nil, argsCount: 2, maxPrice: 9.1, maxVolume: int64(100)},
		{name: "with range", start: &day, end: &day2, argsCount: 3, maxPrice: 10.0, maxVolume: int64(150)},
		{name: "no data (NULLs)", start: &day, end: &day2, argsCount: 3, maxPrice: nil, maxVolume: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Build result row; nil,nil means database NULLs
			price := tc.maxPrice
			volume := tc.maxVolume
			rows := sqlmock.NewRows([]string{"max_price", "max_volume"}).AddRow(price, volume)

			switch tc.argsCount {
			case 1:
				mock.ExpectQuery(selectRegex.String()).
					WithArgs("TEST4").
					WillReturnRows(rows)
			case 2:
				mock.ExpectQuery(selectRegex.String()).
					WithArgs("TEST4", day).
					WillReturnRows(rows)
			case 3:
				mock.ExpectQuery(selectRegex.String()).
					WithArgs("TEST4", day, day2).
					WillReturnRows(rows)
			}

			out, err := repo.GetAggregateByTicker("TEST4", tc.start, tc.end)
			if tc.maxPrice == nil && tc.maxVolume == nil {
				if err != nil || out != nil {
					t.Fatalf("want nil,nil got out=%+v err=%v", out, err)
				}
			} else {
				if err != nil || out == nil {
					t.Fatalf("unexpected out=%+v err=%v", out, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestIngestionLog_SQLMock(t *testing.T) {
	repo, mock, done := newMockRepo(t)
	defer done()

	d := time.Date(2025, 9, 11, 0, 0, 0, 0, time.UTC)

	// HasIngestionForDate
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM ingestion_log WHERE file_date = $1)")).
		WithArgs(d).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	ok, err := repo.HasIngestionForDate(d)
	if err != nil || !ok {
		t.Fatalf("HasIngestionForDate: ok=%v err=%v", ok, err)
	}

	// UpsertIngestionLog
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ingestion_log (file_date, filename, row_count) VALUES ($1, $2, $3) ON CONFLICT (file_date) DO UPDATE SET filename = EXCLUDED.filename,\n\t\t\t\t\t\t\t\t\t\t\t\trow_count = EXCLUDED.row_count,\n\t\t\t\t\t\t\t\t\t\t\t\tingested_at = NOW()")).
		WithArgs(d, "file.txt", 10).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpsertIngestionLog(d, "file.txt", 10); err != nil {
		t.Fatalf("UpsertIngestionLog: %v", err)
	}

	// DeleteTradesByDate
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM trades WHERE trade_date = $1")).
		WithArgs(d).WillReturnResult(sqlmock.NewResult(0, 3))
	if err := repo.DeleteTradesByDate(d); err != nil {
		t.Fatalf("DeleteTradesByDate: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNewTradesRepository_Construct(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func() { _ = db.Close() }()
	r := NewTradesRepository(db)
	if r == nil {
		t.Fatalf("expected non-nil repository")
	}
}

func TestInsertTradesBatch_SQLMock(t *testing.T) {
	repo, mock, done := newMockRepo(t)
	defer done()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect setting local synchronous_commit off
	mock.ExpectExec(regexp.QuoteMeta("SET LOCAL synchronous_commit = OFF")).WillReturnResult(sqlmock.NewResult(0, 0))
	// We cannot intercept pq.CopyIn precisely. Use ExpectPrepare to allow any statement name,
	// then ExpectExec without args twice (for the row and final Exec()). Close/Commit happens normally.
	prep := mock.ExpectPrepare(".*")
	prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 1))     // row exec
	mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0)) // final Exec()
	mock.ExpectCommit()

	trades := []models.Trade{
		{
			ReferenceDate:         time.Date(2025, 9, 11, 0, 0, 0, 0, time.UTC),
			InstrumentCode:        "TEST4",
			UpdateAction:          "I",
			TradePrice:            10.5,
			TradeQuantity:         100,
			ClosingTime:           time.Date(0, 1, 1, 10, 0, 0, 0, time.UTC),
			TradeIdentifierCode:   "X",
			SessionType:           "REG",
			TradeDate:             time.Date(2025, 9, 11, 0, 0, 0, 0, time.UTC),
			BuyerParticipantCode:  "B",
			SellerParticipantCode: "S",
		},
	}

	// Since pq.CopyIn uses the driver-specific CopyIn, sqlmock doesn't support it natively.
	// We validate that the function performs BEGIN, SET, PREPARE/EXEC sequences and COMMIT without error.
	// Note: This is a shallow test to mark coverage; full path is validated by integration tests.
	if err := repo.InsertTradesBatch(trades); err != nil {
		t.Fatalf("InsertTradesBatch: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestInsertTradesBatch_ErrorOnBegin(t *testing.T) {
	repo, mock, done := newMockRepo(t)
	defer done()

	// Force Begin() error
	mock.ExpectBegin().WillReturnError(dummyErr{})
	trades := []models.Trade{{}}
	if err := repo.InsertTradesBatch(trades); err == nil {
		t.Fatalf("expected error on begin")
	}
}

func TestInsertTradesBatch_ErrorOnRowExec(t *testing.T) {
	repo, mock, done := newMockRepo(t)
	defer done()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SET LOCAL synchronous_commit = OFF")).WillReturnResult(sqlmock.NewResult(0, 0))
	prep := mock.ExpectPrepare(".*")
	// First row exec fails
	prep.ExpectExec().WillReturnError(dummyErr{})
	mock.ExpectRollback()

	if err := repo.InsertTradesBatch([]models.Trade{{InstrumentCode: "X"}}); err == nil {
		t.Fatalf("expected error on row exec")
	}
}

func TestInsertTradesBatch_ErrorOnFinalExec(t *testing.T) {
	repo, mock, done := newMockRepo(t)
	defer done()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SET LOCAL synchronous_commit = OFF")).WillReturnResult(sqlmock.NewResult(0, 0))
	prep := mock.ExpectPrepare(".*")
	// Row exec ok
	prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 1))
	// Final Exec() after rows fails
	mock.ExpectExec(".*").WillReturnError(dummyErr{})
	mock.ExpectRollback()

	if err := repo.InsertTradesBatch([]models.Trade{{InstrumentCode: "X"}}); err == nil {
		t.Fatalf("expected error on final exec")
	}
}

// Note: We intentionally skip simulating stmt.Close() error path because sqlmock cannot intercept Close().
