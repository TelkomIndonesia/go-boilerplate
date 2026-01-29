package tinkx

import (
	"github.com/tink-crypto/tink-go/v2/aead"
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

func (p PrimitiveBIDX) Resize(len int) (BIDX, error) {
	return BIDXWithLen(p.BIDX, len)
}
