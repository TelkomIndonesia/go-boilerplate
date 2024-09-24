package sqlval

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/crypto"
)

var _ driver.Value = BIDX[any, crypto.BIDX]{}
var _ BIDXFunc[crypto.PrimitiveBIDX] = (*crypto.DerivableKeyset[crypto.PrimitiveBIDX])(nil).GetPrimitiveFunc(nil)

type BIDXFunc[B crypto.BIDX] func() (B, error)
type BIDXReadWrapper func([][]byte) driver.Valuer
type BIDX[T any, B crypto.BIDX] struct {
	bidxFunc  BIDXFunc[B]
	converter func(T) ([]byte, error)
	wrapper   BIDXReadWrapper
	isWrite   bool
	t         T
}

func (s BIDX[T, M]) Value() (v driver.Value, err error) {
	m, err := s.bidxFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain BlindIndex primitive: %w", err)
	}
	b, err := s.converter(s.t)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to byte: %w", err)
	}

	switch s.isWrite {
	case true:
		b, err = m.ComputePrimary(b)
		v = b
	case false:
		var bs [][]byte
		bs, err = m.ComputeAll(b)
		if err == nil {
			v, err = s.wrapper(bs).Value()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to compute blind index(s): %w", err)
	}
	return v, err
}

func (s BIDX[T, M]) ForWrite() BIDX[T, M] {
	s.isWrite = true
	return s
}

func (s BIDX[T, M]) ForRead(w BIDXReadWrapper) BIDX[T, M] {
	s.isWrite = false
	s.wrapper = w
	return s
}

func BIDXByteArray[B crypto.BIDX](f BIDXFunc[B], s []byte) BIDX[[]byte, B] {
	return BIDX[[]byte, B]{
		bidxFunc: f,
		converter: func(s []byte) ([]byte, error) {
			return s, nil
		},
		isWrite: true,
		t:       s,
	}
}
func BIDXString[B crypto.BIDX](f BIDXFunc[B], s string) BIDX[string, B] {
	return BIDX[string, B]{
		bidxFunc: f,
		converter: func(s string) ([]byte, error) {
			return []byte(s), nil
		},
		isWrite: true,
		t:       s,
	}
}

func BIDXTime[B crypto.BIDX](f BIDXFunc[B], t time.Time) BIDX[time.Time, B] {
	return BIDX[time.Time, B]{
		bidxFunc: f,
		converter: func(t time.Time) ([]byte, error) {
			return t.MarshalBinary()
		},
		isWrite: true,
		t:       t,
	}
}

func BIDXBool[B crypto.BIDX](f BIDXFunc[B], t bool) BIDX[bool, B] {
	return BIDX[bool, B]{
		bidxFunc: f,
		converter: func(t bool) ([]byte, error) {
			if !t {
				return []byte{0}, nil
			}
			return []byte{1}, nil
		},
		isWrite: true,
		t:       t,
	}
}

func BIDXInt64[B crypto.BIDX](f BIDXFunc[B], t int64) BIDX[int64, B] {
	return BIDX[int64, B]{
		bidxFunc: f,
		converter: func(t int64) ([]byte, error) {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(t))
			return b, nil
		},
		isWrite: true,
		t:       t,
	}
}
func BIDXFloat64[B crypto.BIDX](f BIDXFunc[B], t float64) BIDX[float64, B] {
	return BIDX[float64, B]{
		bidxFunc: f,
		converter: func(t float64) ([]byte, error) {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, math.Float64bits(t))
			return b, nil
		},
		isWrite: true,
		t:       t,
	}
}
