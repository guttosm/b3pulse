-- +goose Up
-- +goose StatementBegin
-- Additional indexes to improve aggregate queries performance
-- Speeds up lookups by ticker within a date range and grouping by date
CREATE INDEX IF NOT EXISTS idx_trades_instr_date
    ON trades (instrument_code, trade_date);

-- Optional: helps queries that compute MAX(price) per ticker
-- Note: PostgreSQL may not always use this for MAX(), but it can help in some plans
CREATE INDEX IF NOT EXISTS idx_trades_instr_price
    ON trades (instrument_code, trade_price);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_trades_instr_price;
DROP INDEX IF EXISTS idx_trades_instr_date;
-- +goose StatementEnd
