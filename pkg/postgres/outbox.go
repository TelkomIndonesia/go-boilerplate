package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Outbox struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ContentType    string
	CreatedAt      time.Time
	Content        []byte
	storeEncrypted bool
}

func newOutbox(tid uuid.UUID, ctype string, content any) (o *Outbox, err error) {
	o = &Outbox{TenantID: tid, ContentType: ctype, CreatedAt: time.Now()}
	o.ID, err = uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("fail to create new id for outbox: %w", err)
	}
	o.Content, err = json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal content as json")
	}
	return
}

func newOutboxEncrypted(tid uuid.UUID, ctype string, content any) (o *Outbox, err error) {
	o, err = newOutbox(tid, ctype, content)
	if err != nil {
		return
	}
	o.storeEncrypted = true
	return
}

type OutboxSender func(context.Context, []*Outbox) error

func (p *Postgres) storeOutbox(ctx context.Context, tx *sql.Tx, ob *Outbox) (err error) {
	_, span := p.tracer.Start(ctx, "storeOutbox", trace.WithAttributes(
		attribute.Stringer("tenantID", ob.TenantID),
		attribute.Stringer("id", ob.ID),
		attribute.String("contentType", ob.ContentType),
	))
	defer span.End()

	content := ob.Content
	if ob.storeEncrypted {
		paead, err := p.aead.GetPrimitive(ob.TenantID)
		if err != nil {
			return err
		}
		content, err = paead.Encrypt(content, ob.ID[:])
		if err != nil {
			return fmt.Errorf("fail to encrypt outbox: %w", err)
		}
		content, err = json.Marshal(content)
		if err != nil {
			return fmt.Errorf("fail to marshal encrypted outbox: %w", err)
		}
	}
	fmt.Println(string(content))
	outboxQ := `
		INSERT INTO outbox 
		(id, tenant_id, type, content, is_encrypted, created_at)
		VALUES
		($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.ID, ob.TenantID, ob.ContentType, content, ob.storeEncrypted, ob.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to outbox: %w", err)
	}

	return
}

func (p *Postgres) sendOutbox(ctx context.Context, limit int) (err error) {
	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return fmt.Errorf("fail to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	q := `
		WITH cte AS (
			SELECT id FROM outbox
			WHERE is_delivered = false ORDER BY created_at
			LIMIT $1
		)
		UPDATE outbox o 
		SET is_delivered = true 
		FROM cte
		WHERE o.id = cte.id
		RETURNING o.id, o.tenant_id, o.type, o.content, o.is_encrypted, o.created_at
	`
	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	outboxes := []*Outbox{}
	for rows.Next() {
		o := &Outbox{}

		err = rows.Scan(&o.ID, &o.TenantID, &o.ContentType, &o.Content, &o.storeEncrypted, &o.CreatedAt)
		if err != nil {
			return fmt.Errorf("fail to scan row: %w", err)
		}

		if o.storeEncrypted {
			var content []byte
			err = json.Unmarshal(o.Content, &content)
			if err != nil {
				return fmt.Errorf("fail to unmarshal encrypted outbox: %w", err)
			}
			paead, err := p.aead.GetPrimitive(o.TenantID)
			if err != nil {
				return err
			}
			o.Content, err = paead.Decrypt(content, o.ID[:])
			if err != nil {
				return fmt.Errorf("fail to decrypt encrypted outboxL %w", err)
			}
		}

		outboxes = append(outboxes, o)
	}

	if err = p.obSender(ctx, outboxes); err != nil {
		return fmt.Errorf("fail to send outboxes: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("fail to commit: %w", err)
	}
	return
}
