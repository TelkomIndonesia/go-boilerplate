CREATE TABLE IF NOT EXISTS profile (
    id UUID PRIMARY KEY,
    tenant_ID UUID NOT NULL,
    nin BYTEA,
    nin_bidx BYTEA,
    name BYTEA,
    name_bidx BYTEA,
    phone BYTEA,
    phone_bidx BYTEA,
    email BYTEA,
    email_bidx BYTEA,
    dob BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS text_heap (
    tenant_id UUID NOT NULL,
    type VARCHAR(128) NOT NULL,
    content TEXT NOT NULL, 
    UNIQUE (tenant_id, type, content)
)
