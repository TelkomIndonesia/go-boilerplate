package outboxce

import (
	"fmt"
	"time"

	protobufce "github.com/cloudevents/sdk-go/binding/format/protobuf/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
	"github.com/tink-crypto/tink-go/v2/tink"
	"google.golang.org/protobuf/proto"
)

const (
	ContentTypeProtobufEncrypted = protobufce.ContentTypeProtobuf + "-encrypted"
	ContentTypeProtobuf          = protobufce.ContentTypeProtobuf
	ExtensionTenantID            = "tenantid"
)

type Outbox struct {
	ID        uuid.UUID
	CreatedAt time.Time

	TenantID uuid.UUID
	Source   string
	Type     string
	Content  proto.Message

	ce event.Event
}

func New(tid uuid.UUID, source string, eventType string, content proto.Message) (o Outbox, err error) {
	id, err := uuid.NewV7()
	if err != nil {
		return o, fmt.Errorf("fail to create new id for outbox: %w", err)
	}

	o = Outbox{
		ID:        id,
		CreatedAt: time.Now(),

		TenantID: tid,
		Source:   source,
		Type:     eventType,
		Content:  content,
	}
	err = o.setCloudEvent()
	return
}

func (ob *Outbox) setCloudEvent() (err error) {
	ob.ce = cloudevents.NewEvent()
	ob.ce.SetID(ob.ID.String())
	ob.ce.SetSource(ob.Source)
	ob.ce.SetType(ob.Type)
	ob.ce.SetExtension(ExtensionTenantID, ob.TenantID.String())
	ob.ce.SetTime(ob.CreatedAt)

	data, err := proto.Marshal(ob.Content)
	if err != nil {
		return fmt.Errorf("fail to marshal content: %w", err)
	}
	ob.ce.SetData(protobufce.ContentTypeProtobuf, data)

	err = ob.ce.Validate()
	if err != nil {
		return fmt.Errorf("fail to convert as cloudevent: %w", err)
	}
	return
}

func (ob Outbox) CloudEvent() (ce event.Event) {
	return ob.ce
}

func (ob Outbox) EncryptedCloudEvent(a tink.AEAD) (ce event.Event, err error) {
	b, err := a.Encrypt(ob.ce.Data(), ob.ID[:])
	if err != nil {
		return ce, fmt.Errorf("fail to encrypt cloudevent data: %w", err)
	}
	ce = ob.ce
	ce.SetData(ContentTypeProtobufEncrypted, b)
	return
}
