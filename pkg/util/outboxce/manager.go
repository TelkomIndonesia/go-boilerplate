//go:generate go run github.com/vektra/mockery/v2
package outboxce

import (
	"context"
	"database/sql"
)

type Manager interface {
	Store(ctx context.Context, tx *sql.Tx, ob OutboxCE) (err error)

	Observe(ctx context.Context, relayFunc RelayFunc) (err error)
}
