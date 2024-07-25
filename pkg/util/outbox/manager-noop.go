package outbox

import (
	"context"
	"database/sql"
)

var _ Manager = managerNOP{}

type managerNOP struct{}

func (n managerNOP) StoreOutbox(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error) {
	return
}

func (n managerNOP) StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error) {
	return
}

// ObserveOutboxes implements Manager.
func (n managerNOP) ObserveOutboxes(ctx context.Context, sender Relay) (err error) {
	<-ctx.Done()
	return ctx.Err()
}

func ManagerNOP() Manager { return managerNOP{} }
