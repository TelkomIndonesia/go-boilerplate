package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

func txRollbackDeferer(tx *sql.Tx, err *error) func() {
	return func() {
		if *err != nil {
			tx.Rollback()
		}
	}
}

func (p *Postgres) storeTextHeap(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, contentType string, content string) (err error) {
	nameHeapQ := `
	INSERT INTO text_heap 
	(tenant_id, type, content)
	VALUES
	($1, $2, $3)
	ON CONFLICT (type, content)
	DO NOTHING
`
	_, err = tx.ExecContext(ctx, nameHeapQ,
		tenantID, contentType, content,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to text_heap: %w", err)
	}

	return
}

func (p *Postgres) storeOutbox(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, msgType string, msgFunc argFunc) (err error) {
	outboxQ := `
		INSERT INTO text_heap 
		(id, tenant_id, type, content)
		VALUES
		($1, $2, $3, $4)
	`
	args, err := argList(
		argLiteralE(uuid.NewV7()),
		argLiteral(tenantID),
		argLiteral(msgType),
		msgFunc,
	)
	if err != nil {
		return fmt.Errorf("fail to prepare arguments for insert outbox: %w", err)
	}
	_, err = tx.ExecContext(ctx, outboxQ, args...)
	if err != nil {
		return fmt.Errorf("fail to insert to outbox: %w", err)
	}

	return
}
