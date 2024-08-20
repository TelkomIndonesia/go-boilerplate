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
	PrimitiveAEAD | PrimitiveMAC | PrimitiveBIDX
}
type NewPrimitive[T Primitive] func(*keyset.Handle) (T, error)

type PrimitiveAEAD struct{ tink.AEAD }

func NewPrimitiveAEAD(h *keyset.Handle) (p PrimitiveAEAD, err error) {
	p.AEAD, err = aead.New(h)
	return
}

type PrimitiveMAC struct{ tink.MAC }

func NewPrimitiveMAC(h *keyset.Handle) (p PrimitiveMAC, err error) {
	p.MAC, err = mac.New(h)
	return
}

type PrimitiveBIDX struct{ BIDX }

func NewPrimitiveBIDX(h *keyset.Handle) (p PrimitiveBIDX, err error) {
	p.BIDX, err = NewBIDX(h, 0)
	return
}
func NewPrimitiveBIDXWithLen(len int) func(h *keyset.Handle) (p PrimitiveBIDX, err error) {
	return func(h *keyset.Handle) (p PrimitiveBIDX, err error) {
		p.BIDX, err = NewBIDX(h, len)
		return
	}
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
		return nil, fmt.Errorf("failed to load primitive: %w", err)
	}
	return k, nil
}

func NewInsecureCleartextDerivableKeyset[T Primitive](path string, c NewPrimitive[T]) (*DerivableKeyset[T], error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyset file: %w", err)
	}

	h, err := insecurecleartextkeyset.Read(keyset.NewJSONReader(f))
	if err != nil {
		return nil, fmt.Errorf("failed to load keyset: %w", err)
	}

	return NewDerivableKeyset(h, c)
}

func (m *DerivableKeyset[T]) GetHandle(deriveKey []byte) (h *keyset.Handle, err error) {
	v, ok := m.keys.Load(string(deriveKey))
	if ok {
		h, ok = v.(*keyset.Handle)
	}
	if !ok {
		deriver, err := keyderivation.New(m.master)
		if err != nil {
			return nil, fmt.Errorf("failed to initiate key derivator: %w", err)
		}
		h, err = deriver.DeriveKeyset(deriveKey[:])
		if err != nil {
			return nil, fmt.Errorf("failed to derrive tennat keyset: %w", err)
		}
		m.keys.Store(string(deriveKey), h)
	}
	return
}

func (m *DerivableKeyset[T]) GetPrimitive(deriveKey []byte) (p T, err error) {
	var h *keyset.Handle
	v, ok := m.primitives.Load(string(deriveKey))
	if ok {
		p, ok = v.(T)
	}
	if !ok {
		h, err = m.GetHandle(deriveKey)
		if err != nil {
			return p, err
		}
		p, err = m.constructur(h)
		if err != nil {
			return p, fmt.Errorf("failed to instantiate primitive: %w", err)
		}

		m.primitives.Store(string(deriveKey), p)
	}
	return
}

func (m *DerivableKeyset[T]) GetPrimitiveNHandle(deriveKey []byte) (p T, h *keyset.Handle, err error) {
	h, err = m.GetHandle(deriveKey)
	if err != nil {
		return
	}
	p, err = m.GetPrimitive(deriveKey)
	return
}

func (m *DerivableKeyset[T]) GetPrimitiveFunc(deriveKey []byte) func() (T, error) {
	return func() (T, error) {
		return m.GetPrimitive(deriveKey)
	}
}

func (m *DerivableKeyset[T]) GetHandleFunc(deriveKey []byte) func() (*keyset.Handle, error) {
	return func() (*keyset.Handle, error) {
		return m.GetHandle(deriveKey)
	}
}

func (m *DerivableKeyset[T]) GetPrimitiveNHandleFunc(deriveKey []byte) func() (T, *keyset.Handle, error) {
	return func() (t T, h *keyset.Handle, err error) {
		return m.GetPrimitiveNHandle(deriveKey)
	}
}
