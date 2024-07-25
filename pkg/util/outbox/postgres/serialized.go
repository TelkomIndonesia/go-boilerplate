package postgres

import (
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox"
	"github.com/tink-crypto/tink-go/v2/tink"
	"github.com/vmihailenco/msgpack/v5"
)

var _ outbox.SerializedI = serialized{}

type serialized struct {
	b []byte

	aead tink.AEAD
	ad   []byte
}

// ByteArray implements SerializedI.
func (m serialized) ByteArray() []byte {
	return m.b
}

// Unmarshal implements SerializedI.
func (m serialized) Unmarshal(pointer any) error {
	if m.aead == nil {
		return msgpack.Unmarshal(m.b, pointer)
	}

	var b []byte
	err := msgpack.Unmarshal(m.b, &b)
	if err != nil {
		return fmt.Errorf("fail to unmarshal encrypted data: %w", err)
	}

	b, err = m.aead.Decrypt(b, m.ad)
	if err != nil {
		return fmt.Errorf("fail to decrypt data: %w", err)
	}

	return msgpack.Unmarshal(b, pointer)
}
