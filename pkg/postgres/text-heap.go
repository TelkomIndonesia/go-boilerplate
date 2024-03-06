package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type textHeap struct {
	tenantID    uuid.UUID
	contentType string
	content     string
}

func (p *Postgres) storeTextHeap(ctx context.Context, tx *sql.Tx, th textHeap) (err error) {
	_, span := tracer.Start(ctx, "storeTextHeap", trace.WithAttributes(
		attribute.Stringer("tenantID", th.tenantID),
		attribute.String("contentType", th.contentType),
	))
	defer span.End()

	nameHeapQ := `
	INSERT INTO text_heap 
	(tenant_id, type, content)
	VALUES
	($1, $2, $3)
	ON CONFLICT (tenant_id, type, content)
	DO NOTHING
`
	_, err = tx.ExecContext(ctx, nameHeapQ,
		th.tenantID, th.contentType, th.content,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to text_heap: %w", err)
	}

	return
}

func (p *Postgres) findTextHeap(ctx context.Context, tenantID uuid.UUID, ctype string, qname string) (text []string, err error) {
	_, span := tracer.Start(ctx, "storeTextHeap", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
		attribute.String("contentType", ctype),
	))
	defer span.End()

	q := `SELECT content FROM text_heap WHERE tenand_id = $1 AND type = $2 AND content like %$3%`
	rows, err := p.db.QueryContext(ctx, q, tenantID, ctype, qname)
	if err != nil {
		return nil, fmt.Errorf("fail to query text_heap: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, fmt.Errorf("fail to scan row: %w", err)
		}
		text = append(text, s)
	}
	return
}
