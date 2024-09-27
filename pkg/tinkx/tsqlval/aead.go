package tsqlval

import (
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/tink-crypto/tink-go/v2/tink"
	"github.com/vmihailenco/msgpack/v5"
)

var _ interface {
	sql.Scanner
	driver.Value
} = &AEAD[any, tink.AEAD]{}
var _ AEADFunc[tinkx.PrimitiveAEAD] = (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD])(nil).GetPrimitiveFunc(nil)

type AEADFunc[A tink.AEAD] func() (A, error)

type AEAD[T any, A tink.AEAD] struct {
	aeadFunc AEADFunc[A]
	vtob     func([]byte) (T, error)
	btov     func(T) ([]byte, error)

	v     T
	b     []byte
	ad    []byte
	isNil bool
}

func (s AEAD[T, A]) Value() (driver.Value, error) {
	aead, err := s.aeadFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain AEAD primitive: %w", err)
	}
	s.b, err = s.btov(s.v)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to byte: %w", err)
	}
	return aead.Encrypt(s.b, s.ad)
}

func (s *AEAD[T, A]) Scan(src any) (err error) {
	if src == nil {
		s.isNil = true
		return
	}

	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("not an encrypted byte :%v", src)
	}

	aead, err := s.aeadFunc()
	if err != nil {
		return fmt.Errorf("failed to obtain AEAD primitive: %w", err)
	}

	s.b, err = aead.Decrypt(b, s.ad)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %w", err)
	}
	s.v, err = s.vtob(s.b)
	return
}

func (s *AEAD[T, A]) Plain() T {
	return s.v
}

func (s *AEAD[T, A]) PlainP() *T {
	if s.isNil {
		return nil
	}
	return &s.v
}

func AEADByteArray[A tink.AEAD](aead AEADFunc[A], b []byte, ad []byte) AEAD[[]byte, A] {
	return AEAD[[]byte, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob:     func(b []byte) ([]byte, error) { return b, nil },
		btov:     func(b []byte) ([]byte, error) { return b, nil },
		v:        b,
	}
}

func AEADString[A tink.AEAD](aead AEADFunc[A], s string, ad []byte) AEAD[string, A] {
	return AEAD[string, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob: func(b []byte) (string, error) {
			return string(b), nil
		},
		btov: func(s string) ([]byte, error) {
			return []byte(s), nil
		},
		v: s,
	}
}

func AEADTime[A tink.AEAD](aead AEADFunc[A], t time.Time, ad []byte) AEAD[time.Time, A] {
	return AEAD[time.Time, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob: func(b []byte) (t time.Time, err error) {
			err = t.UnmarshalBinary(b)
			return
		},
		btov: func(s time.Time) ([]byte, error) {
			return s.MarshalBinary()
		},
		v: t,
	}
}

func AEADBool[A tink.AEAD](aead AEADFunc[A], t bool, ad []byte) AEAD[bool, A] {
	return AEAD[bool, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob: func(b []byte) (bool, error) {
			if len(b) != 1 || b[0] > 1 {
				return false, fmt.Errorf("invalid byte %v", b)
			}

			if b[0] == 0 {
				return false, nil
			}
			return true, nil
		},
		btov: func(b bool) ([]byte, error) {
			if !b {
				return []byte{0}, nil
			}
			return []byte{1}, nil
		},
		v: t,
	}
}

func AEADInt64[A tink.AEAD](aead AEADFunc[A], t int64, ad []byte) AEAD[int64, A] {
	return AEAD[int64, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob: func(b []byte) (int64, error) {
			return int64(binary.LittleEndian.Uint64(b)), nil
		},
		btov: func(i int64) ([]byte, error) {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(i))
			return b, nil
		},
		v: t,
	}
}

func AEADFloat64[A tink.AEAD](aead AEADFunc[A], t float64, ad []byte) AEAD[float64, A] {
	return AEAD[float64, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob: func(b []byte) (float64, error) {
			return math.Float64frombits(binary.LittleEndian.Uint64(b)), nil
		},
		btov: func(i float64) ([]byte, error) {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, math.Float64bits(i))
			return b, nil
		},
		v: t,
	}
}

func AEADAny[A tink.AEAD, T any](aead AEADFunc[A], t T, ad []byte) AEAD[T, A] {
	return AEAD[T, A]{
		aeadFunc: aead,
		ad:       ad,
		vtob: func(b []byte) (T, error) {
			pt := new(T)
			err := msgpack.Unmarshal(b, pt)
			return *pt, err
		},
		btov: func(i T) ([]byte, error) {
			return msgpack.Marshal(i)
		},
		v: t,
	}
}
