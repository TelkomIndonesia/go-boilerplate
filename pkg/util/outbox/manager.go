package outbox

import (
	"context"
	"database/sql"

	"github.com/tink-crypto/tink-go/v2/tink"
)

type Sender func(context.Context, []Outbox) error

type Manager interface {
	StoreOutbox(ctx context.Context, tx *sql.Tx, ob Outbox) (err error)
	StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox) (err error)

	WatchOuboxes(ctx context.Context) (err error)
}

type AEADFunc func(ob Outbox) (tink.AEAD, error)

var _ Manager = noopManager{}

type noopManager struct{}

// StoreOutbox implements Manager.
func (n noopManager) StoreOutbox(ctx context.Context, tx *sql.Tx, ob Outbox) (err error) {
	return nil
}

func (n noopManager) StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox) (err error) {
	return nil
}

// WatchOuboxes implements Manager.
func (n noopManager) WatchOuboxes(ctx context.Context) (err error) {
	<-ctx.Done()
	return ctx.Err()
}
