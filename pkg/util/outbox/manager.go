package outbox

import (
	"context"
	"database/sql"

	"github.com/tink-crypto/tink-go/v2/tink"
)

type Sender func(context.Context, []Outbox[Serialized]) error

type Manager interface {
	StoreOutbox(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error)
	StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error)

	WatchOuboxes(ctx context.Context) (err error)
}

type AEADFunc func(ob Outbox[any]) (tink.AEAD, error)

var _ Manager = managerNOP{}

type managerNOP struct{}

// StoreOutbox implements Manager.
func (n managerNOP) StoreOutbox(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error) {
	return
}

func (n managerNOP) StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error) {
	return
}

// WatchOuboxes implements Manager.
func (n managerNOP) WatchOuboxes(ctx context.Context) (err error) {
	<-ctx.Done()
	return ctx.Err()
}

func ManagerNOP() Manager { return managerNOP{} }
