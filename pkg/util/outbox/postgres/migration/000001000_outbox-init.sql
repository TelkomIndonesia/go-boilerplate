-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    type VARCHAR(128) NOT NULL,
    content JSONB NOT NULL,
    event VARCHAR(128) NOT NULL,
    is_encrypted BOOLEAN DEFAULT FALSE,
    is_delivered BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS outbox_by_created_at ON outbox(is_delivered, created_at);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE outbox

-- +goose StatementEnd