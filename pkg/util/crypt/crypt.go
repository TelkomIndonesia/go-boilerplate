package crypt

import (
	"fmt"
	"os"
	"sync"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
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

func NewDerivableKeyset[T Primitive](m *keyset.Handle, c NewPrimitive[T]) (*DerivableKeyset[T], error) {
	k := &DerivableKeyset[T]{
		master:      m,
		constructur: c,
	}

	if _, err := k.GetPrimitive(nil); err != nil {
		return nil, fmt.Errorf("fail to load primitive: %w", err)
	}
	return k, nil
}

func NewInsecureCleartextDerivableKeyset[T Primitive](path string, c NewPrimitive[T]) (*DerivableKeyset[T], error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("fail to open keyset file: %w", err)
	}

	h, err := insecurecleartextkeyset.Read(keyset.NewJSONReader(f))
	if err != nil {
		return nil, fmt.Errorf("fail to load keyset: %w", err)
	}

	return NewDerivableKeyset(h, c)
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
