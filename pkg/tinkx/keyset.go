package tinkx

import (
	"fmt"
	"os"

	"github.com/maypok86/otter"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

type DerivableKeysetOptFunc[T Primitive] func(*DerivableKeyset[T]) error

func DerivableKeysetWithCapCache[T Primitive](capacity int) DerivableKeysetOptFunc[T] {
	return func(dk *DerivableKeyset[T]) error {
		{
			o, err := otter.MustBuilder[string, *keyset.Handle](capacity).Build()
			if err != nil {
				return err
			}
			dk.keys = &ottercache[*keyset.Handle]{o: o}
		}
		{
			o, err := otter.MustBuilder[string, T](capacity).Build()
			if err != nil {
				return err
			}
			dk.primitives = &ottercache[T]{o: o}
		}

		return nil
	}
}

type DerivableKeyset[T Primitive] struct {
	master      *keyset.Handle
	constructur NewPrimitive[T]
	keys        DerivationCache[*keyset.Handle]
	primitives  DerivationCache[T]
}

func NewDerivableKeyset[T Primitive](m *keyset.Handle, c NewPrimitive[T], opts ...DerivableKeysetOptFunc[T]) (*DerivableKeyset[T], error) {
	k := &DerivableKeyset[T]{
		master:      m,
		constructur: c,
		keys:        nocache[*keyset.Handle]{},
		primitives:  nocache[T]{},
	}

	for _, opt := range opts {
		if err := opt(k); err != nil {
			return nil, fmt.Errorf("fail to apply options: %w", err)
		}
	}

	if _, err := k.GetPrimitive(nil); err != nil {
		return nil, fmt.Errorf("failed to load primitive: %w", err)
	}
	return k, nil
}

func NewInsecureCleartextDerivableKeyset[T Primitive](path string, c NewPrimitive[T], opts ...DerivableKeysetOptFunc[T]) (*DerivableKeyset[T], error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyset file: %w", err)
	}

	h, err := insecurecleartextkeyset.Read(keyset.NewJSONReader(f))
	if err != nil {
		return nil, fmt.Errorf("failed to load keyset: %w", err)
	}

	return NewDerivableKeyset(h, c, opts...)
}

func (m *DerivableKeyset[T]) GetHandle(deriveKey []byte) (h *keyset.Handle, err error) {
	h, ok := m.keys.Get(deriveKey)
	if !ok {
		deriver, err := keyderivation.New(m.master)
		if err != nil {
			return nil, fmt.Errorf("failed to initiate key derivator: %w", err)
		}
		h, err = deriver.DeriveKeyset(deriveKey[:])
		if err != nil {
			return nil, fmt.Errorf("failed to derrive tennat keyset: %w", err)
		}
		m.keys.Set(deriveKey, h)
	}
	return
}

func (m *DerivableKeyset[T]) GetPrimitive(deriveKey []byte) (p T, err error) {
	p, ok := m.primitives.Get(deriveKey)
	if !ok {
		h, err := m.GetHandle(deriveKey)
		if err != nil {
			return p, err
		}
		p, err = m.constructur(h)
		if err != nil {
			return p, fmt.Errorf("failed to instantiate primitive: %w", err)
		}

		m.primitives.Set(deriveKey, p)
	}
	return
}

func (m *DerivableKeyset[T]) GetPrimitiveAndHandle(deriveKey []byte) (p T, h *keyset.Handle, err error) {
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

func (m *DerivableKeyset[T]) GetPrimitiveAndHandleFunc(deriveKey []byte) func() (T, *keyset.Handle, error) {
	return func() (t T, h *keyset.Handle, err error) {
		return m.GetPrimitiveAndHandle(deriveKey)
	}
}
