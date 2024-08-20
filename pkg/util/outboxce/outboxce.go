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
)

type AEADFunc func(event.Event) (tink.AEAD, error)

type OutboxCE struct {
	ID   uuid.UUID
	Time time.Time

	TenantID  uuid.UUID
	Source    string
	EventType string
	Content   proto.Message

	AEADFunc AEADFunc

	errID error
}

func New(source string, eventType string, tenantID uuid.UUID, content proto.Message) OutboxCE {
	o := OutboxCE{
		TenantID:  tenantID,
		Source:    source,
		EventType: eventType,
		Content:   content,
		Time:      time.Now(),
	}
	o.ID, o.errID = uuid.NewV7()
	return o
}

func (o OutboxCE) WithModifier(fn func(o OutboxCE) OutboxCE) OutboxCE {
	return fn(o)
}

func (o OutboxCE) WithEncryptor(fn func(event.Event) (tink.AEAD, error)) OutboxCE {
	o.AEADFunc = fn
	return o
}

func (o OutboxCE) Build() (ce event.Event, err error) {
	if o.errID != nil {
		return ce, fmt.Errorf("fail to generate id: %w", err)
	}

	ce = cloudevents.NewEvent()
	ce.SetID(o.ID.String())
	ce.SetSource(o.Source)
	ce.SetSubject(o.TenantID.String())
	ce.SetType(o.EventType)
	ce.SetTime(o.Time)

	dct := ContentTypeProtobuf
	data, err := proto.Marshal(o.Content)
	if err != nil {
		return ce, fmt.Errorf("fail to marshal content: %w", err)
	}
	if o.AEADFunc != nil {
		dct = ContentTypeProtobufEncrypted
		aead, err := o.AEADFunc(ce)
		if err != nil {
			return ce, fmt.Errorf("faill to obtain aead primitive: %w", err)
		}
		data, err = aead.Encrypt(data, []byte(ce.ID()))
		if err != nil {
			return ce, fmt.Errorf("faill to encrypt data: %w", err)
		}
	}
	ce.SetData(dct, data)
	return
}

func FromEvent(e event.Event, aeadFunc AEADFunc, Unmarshaller func([]byte) (proto.Message, error)) (o OutboxCE, err error) {
	o.ID, err = uuid.Parse(e.ID())
	if err != nil {
		return o, fmt.Errorf("fail to parse id : %w", err)
	}
	o.TenantID, err = uuid.Parse(e.Subject())
	if err != nil {
		return o, fmt.Errorf("fail to parse tenant id : %w", err)
	}
	o.Source = e.Source()
	o.EventType = e.Type()
	o.Time = e.Time()

	d := e.Data()
	if e.DataContentType() == ContentTypeProtobufEncrypted {
		aead, err := aeadFunc(e)
		if err != nil {
			return o, fmt.Errorf("fail to obtain aead primitive: %w", err)
		}
		d, err = aead.Decrypt(d, []byte(e.ID()))
		if err != nil {
			return o, fmt.Errorf("fail to decrypt: %w", err)
		}
		o.AEADFunc = aeadFunc
	}
	o.Content, err = Unmarshaller(d)
	return
}
