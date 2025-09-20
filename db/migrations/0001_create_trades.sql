-- +goose Up
-- +goose StatementBegin
-- Install UUID and crypto extensions for PostgreSQL
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create a custom UUID v7 function
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS UUID AS $$
DECLARE
unix_ts_ms BIGINT;
    uuid_bytes BYTEA;
BEGIN
    -- Get current timestamp in milliseconds since Unix epoch
    unix_ts_ms := EXTRACT(EPOCH FROM NOW()) * 1000;

    -- Generate UUID v7: timestamp (48 bits) + version (4 bits) + random (12 bits) + variant (2 bits) + random (62 bits)
    uuid_bytes :=
        substring(int8send(unix_ts_ms), 3, 6) ||
        substring(gen_random_bytes(2), 1, 2) ||
        substring(gen_random_bytes(8), 1, 8);

    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112);
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128);

RETURN encode(uuid_bytes, 'hex')::UUID;
END;
$$ LANGUAGE plpgsql VOLATILE;

-- Create trades table with all columns in English
CREATE TABLE IF NOT EXISTS trades (
    id UUID                 PRIMARY KEY DEFAULT uuid_generate_v7(),
    reference_date          DATE,
    instrument_code         VARCHAR(50) NOT NULL,
    update_action           VARCHAR(10),
    trade_price             NUMERIC(18,6),
    trade_quantity          BIGINT,
    closing_time            TIME,
    trade_identifier_code   VARCHAR(50),
    session_type            VARCHAR(10),
    trade_date              DATE,
    buyer_participant_code  VARCHAR(50),
    seller_participant_code VARCHAR(50),

    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
    );

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_trades_instrument_code
    ON trades (instrument_code);

CREATE INDEX IF NOT EXISTS idx_trades_trade_date
    ON trades (trade_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trades;
DROP FUNCTION IF EXISTS uuid_generate_v7();
DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
-- +goose StatementEnd
