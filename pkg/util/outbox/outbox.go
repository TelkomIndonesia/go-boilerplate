package outbox

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tink-crypto/tink-go/v2/tink"
)

type Outbox struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	Event       string    `json:"event"`
	Content     any       `json:"content"`
	IsEncrypted bool      `json:"is_encrypted"`

	aead        tink.AEAD
	contentByte []byte
}

func newOutbox(tid uuid.UUID, event string, ctype string, content any) (o Outbox, err error) {
	id, err := uuid.NewV7()
	if err != nil {
		return o, fmt.Errorf("fail to create new id for outbox: %w", err)
	}

	o = Outbox{
		ID:          id,
		TenantID:    tid,
		Event:       event,
		ContentType: ctype,
		CreatedAt:   time.Now(),
		Content:     content,
	}
	return
}

func (ob Outbox) AsEncrypted(aead tink.AEAD) (o Outbox, err error) {
	if ob.IsEncrypted {
		return ob, nil
	}

	ob.aead = aead

	b, err := json.Marshal(ob.Content)
	if err != nil {
		return o, fmt.Errorf("fail to marshal content")
	}

	ob.Content, err = ob.aead.Encrypt(b, ob.ID[:])
	if err != nil {
		return o, fmt.Errorf("fail to encrypt outbox: %w", err)
	}

	ob.IsEncrypted = true
	return ob, nil
}

func (ob Outbox) AsUnEncrypted() (o Outbox, err error) {
	if !ob.IsEncrypted {
		return ob, nil
	}

	if ob.aead == nil {
		return o, fmt.Errorf("can't decrypt due to nil decryptor")
	}

	content, ok := ob.Content.([]byte)
	if !ok {
		return o, fmt.Errorf("not a byte string of chipertext")
	}
	ob.contentByte, err = ob.aead.Decrypt(content, ob.ID[:])
	if err != nil {
		return o, fmt.Errorf("fail to decrypt encrypted outbox: %w", err)
	}
	err = json.Unmarshal(ob.contentByte, &ob.Content)
	if err != nil {
		return o, fmt.Errorf("fail to unmarshal encrypted outbox: %w", err)
	}

	ob.IsEncrypted = false
	return ob, nil
}

func (o Outbox) ContentByte() []byte {
	return o.contentByte
}
