package postgres

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
)

func WithKeysets(aeadKey *keyset.Handle, macKey *keyset.Handle) PostgresOptFunc {
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

func WithConnString(connStr string) PostgresOptFunc {
	return func(p *Postgres) (err error) {
		p.db, err = sql.Open("postgres", connStr)
		return
	}
}

type PostgresOptFunc func(*Postgres) error

type Postgres struct {
	db   *sql.DB
	aead multiTenantKeyset[primitiveAEAD]
	mac  multiTenantKeyset[primitiveMAC]
}

func NewPostgres(opts ...PostgresOptFunc) (*Postgres, error) {
	p := &Postgres{}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (p *Postgres) GetBlindIdxKeys(tenantID uuid.UUID, key []byte) (idxs [][]byte, err error) {
	h, err := p.mac.GetHandle(tenantID)
	if err != nil {
		return nil, fmt.Errorf("fail to get keyset handle for tenant %s: %w", tenantID, err)
	}
	h, err = copyHandle(h)
	if err != nil {
		return nil, err
	}

	idxs = make([][]byte, 0, len(h.KeysetInfo().GetKeyInfo()))
	mgr := keyset.NewManagerFromHandle(h)
	for _, i := range h.KeysetInfo().GetKeyInfo() {
		mgr.SetPrimary(i.GetKeyId())
		m, err := mac.New(h)
		if err != nil {
			return nil, fmt.Errorf("fail to instantiate primitive from key id %d: %w", i.GetKeyId(), err)
		}

		b, err := m.ComputeMAC(key)
		if err != nil {
			return nil, fmt.Errorf("fail to compute mac from key id %d: %w", i.GetKeyId(), err)
		}

		idxs = append(idxs, b)
	}
	return nil, nil
}
