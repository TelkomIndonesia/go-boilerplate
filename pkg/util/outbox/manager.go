//go:generate go run github.com/vektra/mockery/v2
package outbox

import (
	"context"
	"database/sql"

	"github.com/tink-crypto/tink-go/v2/tink"
)

type AEADFunc func(ob Outbox[any]) (tink.AEAD, error)

type RelayFunc func(ctx context.Context, obs []Outbox[Serialized]) error

type Manager interface {
	Store(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error)
	StoreAsEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox[any]) (err error)

	Observe(ctx context.Context, relayFunc RelayFunc) (err error)
}
