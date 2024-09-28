package tinkx

import (
	"github.com/maypok86/otter"
)

type DerivationCache[T any] interface {
	Get(key []byte) (t T, ok bool)
	Set(key []byte, t T) (ok bool)
}

var _ DerivationCache[struct{}] = nocache[struct{}]{}

type nocache[T any] struct{}

func (n nocache[T]) Get(key []byte) (t T, ok bool) {
	return
}

func (n nocache[T]) Set(key []byte, t T) (ok bool) {
	return true
}

var _ DerivationCache[struct{}] = &ottercache[struct{}]{}

type ottercache[T any] struct {
	o otter.Cache[string, T]
}

func (o *ottercache[T]) Get(key []byte) (t T, ok bool) {
	return o.o.Get(string(key))
}
func (o *ottercache[T]) Set(key []byte, t T) (ok bool) {
	return o.o.Set(string(key), t)
}
