-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ingestion_log (
    file_date   DATE PRIMARY KEY,
    filename    TEXT NOT NULL,
    row_count   BIGINT NOT NULL DEFAULT 0,
    ingested_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ingestion_log_file_date
    ON ingestion_log (file_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ingestion_log;
-- +goose StatementEnd
