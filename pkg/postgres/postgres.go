package postgres

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/tink-crypto/tink-go/v2/keyset"
)

func WithKeysets(aeadKey *keyset.Handle, macKey *keyset.Handle) OptFunc {
	return func(p *Postgres) (err error) {
		p.aead = multiTenantKeyset[primitiveAEAD]{master: aeadKey, constructur: newPrimitiveAEAD}
		if _, err = p.aead.GetPrimitive(uuid.UUID{}); err != nil {
			return fmt.Errorf("fail to verify aead derivable-keyset: %w", err)
		}

		p.mac = multiTenantKeyset[primitiveMAC]{master: macKey, constructur: newPrimitiveMAC}
		if _, err = p.mac.GetPrimitive(uuid.UUID{}); err != nil {
			return fmt.Errorf("fail to create mac derivable-keyset: %w", err)
		}
		return nil
	}
}

func WithConnString(connStr string) OptFunc {
	return func(p *Postgres) (err error) {
		p.db, err = sql.Open("postgres", connStr)
		return
	}
}

type OptFunc func(*Postgres) error

type Postgres struct {
	db   *sql.DB
	aead multiTenantKeyset[primitiveAEAD]
	mac  multiTenantKeyset[primitiveMAC]
}

func New(opts ...OptFunc) (p *Postgres, err error) {
	p = &Postgres{}
	for _, opt := range opts {
		if err = opt(p); err != nil {
			return p, err
		}
	}
	return p, nil
}
