package outbox

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Unmarshalable interface {
	Unmarshal(pointer any) error
}

type Serialized struct {
	Value []byte
	Unmarshalable
}

type Outbox[T any | Serialized] struct {
	ID          uuid.UUID `json:"id" msgpack:"id"`
	TenantID    uuid.UUID `json:"tenant_id" msgpack:"tenant_id"`
	ContentType string    `json:"content_type" msgpack:"content_type"`
	CreatedAt   time.Time `json:"created_at" msgpack:"created_at"`
	EventName   string    `json:"event_name" msgpack:"event_name"`
	Content     T         `json:"content" msgpack:"content"`
	IsEncrypted bool      `json:"is_encrypted" msgpack:"is_encrypted"`
}

func New(tid uuid.UUID, eventName string, contentType string, content any) (o Outbox[any], err error) {
	id, err := uuid.NewV7()
	if err != nil {
		return o, fmt.Errorf("fail to create new id for outbox: %w", err)
	}

	o = Outbox[any]{
		ID:        id,
		CreatedAt: time.Now(),

		TenantID:    tid,
		EventName:   eventName,
		ContentType: contentType,
		Content:     content,
	}
	return
}
