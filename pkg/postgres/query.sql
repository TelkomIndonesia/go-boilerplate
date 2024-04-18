
-- name: StoreProfile :exec
INSERT INTO profile
    (id, tenant_id, nin, nin_bidx, name, name_bidx, phone, phone_bidx, email, email_bidx, dob)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (id) 
    DO UPDATE SET updated_at = NOW();

-- name: FetchProfile :one
SELECT 
    nin, name, phone, email, dob 
FROM 
    profile 
WHERE 
    id = $1 AND tenant_id = $2;

-- name: FindProfilesByName :many
SELECT 
    id, nin, name, phone, email, dob 
FROM 
    profile 
WHERE 
    tenant_id = $1 and name_bidx = ANY($2);

-- name: FindTextHeap :many
SELECT 
    content 
FROM 
    text_heap 
WHERE 
    tenant_id = $1 AND type = $2 
    AND content LIKE sqlc.arg(content) || '%'; -- https://docs.sqlc.dev/en/latest/howto/named_parameters.html#naming-parameters


-- name: StoreTextHeap :exec
INSERT INTO text_heap 
	(tenant_id, type, content)
VALUES
	($1, $2, $3)
ON CONFLICT (tenant_id, type, content) 
    DO NOTHING;
