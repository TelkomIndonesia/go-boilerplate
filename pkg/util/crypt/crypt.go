package crypt

import (
	"fmt"
	"sync"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/tink"
)

type Primitive interface {
	PrimitiveAEAD | PrimitiveMAC
}
type PrimitiveAEAD struct{ tink.AEAD }
type PrimitiveMAC struct{ tink.MAC }

type NewPrimitive[T Primitive] func(*keyset.Handle) (T, error)

func NewPrimitiveAEAD(h *keyset.Handle) (p PrimitiveAEAD, err error) {
	p.AEAD, err = aead.New(h)
	return
}
func NewPrimitiveMAC(h *keyset.Handle) (p PrimitiveMAC, err error) {
	p.MAC, err = mac.New(h)
	return
}

type DerivableKeyset[T Primitive] struct {
	master      *keyset.Handle
	constructur NewPrimitive[T]
	keys        sync.Map
	primitives  sync.Map
}

func NewDerivableKeySet[T Primitive](m *keyset.Handle, c NewPrimitive[T]) *DerivableKeyset[T] {
	return &DerivableKeyset[T]{
		master:      m,
		constructur: c,
	}
}

func (m *DerivableKeyset[T]) GetHandle(salt []byte) (h *keyset.Handle, err error) {
	v, ok := m.keys.Load(string(salt))
	if ok {
		h, ok = v.(*keyset.Handle)
	}
	if !ok {
		deriver, err := keyderivation.New(m.master)
		if err != nil {
			return nil, fmt.Errorf("fail to initiate key derivator: %w", err)
		}
		h, err = deriver.DeriveKeyset(salt[:])
		if err != nil {
			return nil, fmt.Errorf("fail to derrive tennat keyset: %w", err)
		}
		m.keys.Store(string(salt), h)
	}
	return
}

func (m *DerivableKeyset[T]) GetPrimitive(salt []byte) (p T, err error) {
	var h *keyset.Handle
	v, ok := m.primitives.Load(string(salt))
	if ok {
		p, ok = v.(T)
	}
	if !ok {
		h, err = m.GetHandle(salt)
		if err != nil {
			return p, err
		}
		p, err = m.constructur(h)
		if err != nil {
			return p, fmt.Errorf("fail to instantiate primitive: %w", err)
		}

		m.primitives.Store(string(salt), p)
	}
	return
}
