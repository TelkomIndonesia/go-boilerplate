package outboxce

import (
	"context"
	"database/sql"
)

type Manager interface {
	Store(ctx context.Context, tx *sql.Tx, ob OutboxCE) (err error)

	RelayLoop(ctx context.Context, relayFunc RelayFunc) (err error)
}
