-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS outboxce (
    id UUID PRIMARY KEY,
    tenant_id UUID,
    cloud_event JSON NOT NULL,
    is_delivered BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS outboxce_by_created_at ON outboxce(created_at) WHERE is_delivered = false;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE outboxce

-- +goose StatementEnd