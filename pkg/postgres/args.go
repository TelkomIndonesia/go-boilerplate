package postgres

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type argFunc func() (any, error)

func argList(fns ...argFunc) ([]any, error) {
	args := make([]any, 0, len(fns))
	for i, fn := range fns {
		v, err := fn()
		if err != nil {
			return nil, fmt.Errorf("fail creating argument at index %d: %w", i, err)
		}
		args = append(args, v)
	}
	return args, nil
}

func argLiteral(a any) argFunc {
	return func() (any, error) {
		return a, nil
	}
}

func argLiteralE(a any, e error) argFunc {
	return func() (any, error) {
		return a, e
	}
}

func argAsB64(fn argFunc) argFunc {
	return func() (any, error) {
		a, err := fn()
		if err != nil {
			return nil, err
		}

		b, ok := a.([]byte)
		if !ok {
			return nil, fmt.Errorf("cannot convert a non byte arg to base64")
		}

		buf := make([]byte, 0, base64.StdEncoding.EncodedLen(len(b)))
		base64.StdEncoding.Encode(b, buf)
		return buf, nil
	}
}

func (p *Postgres) argEncTime(tenantID uuid.UUID, t time.Time, aad []byte) argFunc {
	return func() (any, error) {
		b, err := t.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("fail to marshal time to binary: %w", err)
		}
		paead, err := p.aead.GetPrimitive(tenantID)
		if err != nil {
			return nil, err
		}
		enc, err := paead.Encrypt(b, aad)
		return enc, err
	}
}

func (p *Postgres) argEncStr(tenantID uuid.UUID, str string, aad []byte) argFunc {
	return func() (any, error) {
		paead, err := p.aead.GetPrimitive(tenantID)
		if err != nil {
			return nil, err
		}
		enc, err := paead.Encrypt([]byte(str), aad)
		return enc, err
	}
}

func (p *Postgres) argEncJSON(tenantID uuid.UUID, v any, aad []byte) argFunc {
	return func() (any, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal: %w", err)
		}

		paead, err := p.aead.GetPrimitive(tenantID)
		if err != nil {
			return nil, err
		}
		return paead.Encrypt(b, aad)
	}
}

func (p *Postgres) argBlindIdx(tenantID uuid.UUID, str string) argFunc {
	return func() (any, error) {
		pmac, err := p.mac.GetPrimitive(tenantID)
		if err != nil {
			return nil, err
		}
		enc, err := pmac.ComputeMAC([]byte(str))
		return enc[:min(len(enc), p.bidxLen)], err
	}
}
