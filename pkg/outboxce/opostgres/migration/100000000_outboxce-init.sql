-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS outboxce (
    id UUID PRIMARY KEY,
    attributes JSON NOT NULL,
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    is_delivered BOOLEAN
);
CREATE INDEX IF NOT EXISTS outboxce_by_created_at ON outboxce(created_at) WHERE is_delivered = false;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE outboxce

-- +goose StatementEnd