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
	constructur newPrimitive[T]
	keys        sync.Map
	primitives  sync.Map
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

func (p *Postgres) GetBlindIdxKeys(tenantID uuid.UUID, key []byte) (idxs [][]byte, err error) {
	h, err := p.mac.GetHandle(tenantID)
	if err != nil {
		return nil, fmt.Errorf("fail to get keyset handle for tenant %s: %w", tenantID, err)
	}
	return getBlindIdxs(h, key)
}

func getBlindIdxs(h *keyset.Handle, key []byte) (idxs [][]byte, err error) {
	h, err = cloneHandle(h)
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

func cloneHandle(h *keyset.Handle) (hc *keyset.Handle, err error) {
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
