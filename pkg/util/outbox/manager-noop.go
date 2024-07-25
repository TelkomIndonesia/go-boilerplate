package outbox

import (
	"context"
	"database/sql"
)

var _ Manager = managerNOP{}

type managerNOP struct{}

func (n managerNOP) Store(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error) {
	return
}

func (n managerNOP) StoreAsEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error) {
	return
}

func (n managerNOP) Observe(ctx context.Context, sender Relay) (err error) {
	<-ctx.Done()
	return ctx.Err()
}

func ManagerNOP() Manager { return managerNOP{} }
