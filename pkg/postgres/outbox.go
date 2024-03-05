package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type outbox struct {
	id          uuid.UUID
	tenantID    uuid.UUID
	contentType string
	content     any
	isEncrypted bool
}

func (p *Postgres) storeOutbox(ctx context.Context, tx *sql.Tx, ob *outbox) (err error) {
	var content any
	switch ob.isEncrypted {
	case true:
		content, err = argAsB64(p.argEncJSON(ob.tenantID, ob, ob.id[:]))()
		if err != nil {
			return fmt.Errorf("fail to encrypt outbox content: %w", err)
		}

	case false:
		content, err = json.Marshal(ob.content)
		if err != nil {
			return fmt.Errorf("fail to marshall content as json")
		}
	}

	if len(ob.id) == 0 {
		ob.id, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("fail to automatically assign id to outbox : %w", err)
		}
	}

	outboxQ := `
		INSERT INTO text_heap 
		(id, tenant_id, type, content)
		VALUES
		($1, $2, $3, $4)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.id, ob.tenantID, ob.contentType, content,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to outbox: %w", err)
	}

	return
}
