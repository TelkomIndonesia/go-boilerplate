package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type outbox struct {
	id          uuid.UUID
	tenantID    uuid.UUID
	contentType string
	content     any
	isEncrypted bool
}

func (p *Postgres) storeOutbox(ctx context.Context, tx *sql.Tx, ob *outbox) (err error) {
	_, span := tracer.Start(ctx, "storeOutbox", trace.WithAttributes(
		attribute.Stringer("tenantID", ob.tenantID),
		attribute.Stringer("id", ob.id),
		attribute.String("contentType", ob.contentType),
	))
	defer span.End()

	content := ob.content
	if ob.isEncrypted {
		content, err = p.argEncJSON(ob.tenantID, ob, ob.id[:])()
		if err != nil {
			return fmt.Errorf("fail to encrypt outbox content: %w", err)
		}
	}
	content, err = json.Marshal(content)
	if err != nil {
		return fmt.Errorf("fail to marshall content as json")
	}

	if len(ob.id) == 0 {
		ob.id, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("fail to automatically assign id to outbox : %w", err)
		}
	}

	outboxQ := `
		INSERT INTO outbox 
		(id, tenant_id, type, content, is_encrypted)
		VALUES
		($1, $2, $3, $4, $5)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.id, ob.tenantID, ob.contentType, content, ob.isEncrypted,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to outbox: %w", err)
	}

	return
}
