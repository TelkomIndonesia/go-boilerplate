-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS outboxce (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    cloud_event JSON NOT NULL,
    is_delivered BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS outbox_by_created_at ON outboxce(is_delivered, created_at);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE outboxce

-- +goose StatementEnd