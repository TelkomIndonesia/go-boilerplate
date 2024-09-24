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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, nin)
);

CREATE TABLE IF NOT EXISTS text_heap (
    tenant_id UUID NOT NULL,
    type VARCHAR(128) NOT NULL,
    content TEXT NOT NULL, 
    UNIQUE (tenant_id, type, content)
);
