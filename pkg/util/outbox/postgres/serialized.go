package postgres

import (
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox"
	"github.com/tink-crypto/tink-go/v2/tink"
	"github.com/vmihailenco/msgpack/v5"
)

type serialized struct {
	b []byte

	aead tink.AEAD
	ad   []byte
}

// Unmarshal implements SerializedI.
func (m serialized) Unmarshal(pointer any) (err error) {
	if m.aead == nil {
		return msgpack.Unmarshal(m.b, pointer)
	}

	b, err := m.aead.Decrypt(m.b, m.ad)
	if err != nil {
		return fmt.Errorf("fail to decrypt data: %w", err)
	}

	return msgpack.Unmarshal(b, pointer)
}

func (m serialized) Serialized() outbox.Serialized {
	return outbox.Serialized{
		ByteArray:     m.b,
		Unmarshalable: m,
	}
}
