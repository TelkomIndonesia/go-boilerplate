package postgres

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/tink"
)

type primitiveAEAD struct{ tink.AEAD }
type primitiveMAC struct{ tink.MAC }
type newPrimitive[T primitiveAEAD | primitiveMAC] func(*keyset.Handle) (T, error)

func newPrimitiveAEAD(h *keyset.Handle) (p primitiveAEAD, err error) {
	p.AEAD, err = aead.New(h)
	return
}
func newPrimitiveMAC(h *keyset.Handle) (p primitiveMAC, err error) {
	p.MAC, err = mac.New(h)
	return
}

type multiTenantKeyset[T primitiveAEAD | primitiveMAC] struct {
	master      *keyset.Handle
	keys        sync.Map
	primitives  sync.Map
	constructur newPrimitive[T]
}

func (m *multiTenantKeyset[T]) GetHandle(tenantID uuid.UUID) (h *keyset.Handle, err error) {
	v, ok := m.keys.Load(tenantID)
	if ok {
		h, ok = v.(*keyset.Handle)
	}
	if !ok {
		deriver, err := keyderivation.New(m.master)
		if err != nil {
			return nil, fmt.Errorf("fail to initiate key derivator: %w", err)
		}
		h, err = deriver.DeriveKeyset(tenantID[:])
		if err != nil {
			return nil, fmt.Errorf("fail to derrive tennat keyset: %w", err)
		}
		m.keys.Store(tenantID, h)
	}
	return
}

func (m *multiTenantKeyset[T]) GetPrimitive(tenantID uuid.UUID) (p T, err error) {
	var h *keyset.Handle
	v, ok := m.primitives.Load(tenantID)
	if ok {
		p, ok = v.(T)
	}
	if !ok {
		h, err = m.GetHandle(tenantID)
		if err != nil {
			return p, err
		}
		p, err = m.constructur(h)
		if err != nil {
			return p, fmt.Errorf("fail to instantiate primitive: %w", err)
		}

		m.primitives.Store(tenantID, p)
	}
	return
}

func copyHandle(h *keyset.Handle) (hc *keyset.Handle, err error) {
	b := new(bytes.Buffer)
	w := keyset.NewBinaryWriter(b)
	err = insecurecleartextkeyset.Write(h, w)
	if err != nil {
		return nil, fmt.Errorf("fail to copy handle to memory: %w", err)
	}

	r := keyset.NewBinaryReader(b)
	hc, err = insecurecleartextkeyset.Read(r)
	if err != nil {
		return nil, fmt.Errorf("fail to copy handle from memory: %w", err)
	}

	return hc, nil
}
