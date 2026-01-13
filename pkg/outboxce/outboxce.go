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

	CEExtensionTenantID = "tenantid"
)

type AEADFunc func(event.Event) (tink.AEAD, error)

type OutboxCE struct {
	TenantID *uuid.UUID
	ID       uuid.UUID
	Time     time.Time

	EventType  string
	Source     string
	Subject    string
	Data       proto.Message
	DataSchema string

	AEADFunc AEADFunc

	err error
}

func New(source string, eventType string, content proto.Message) OutboxCE {
	o := OutboxCE{
		Source:    source,
		EventType: eventType,
		Data:      content,
	}

	var err error
	o.ID, err = uuid.NewV7()
	if err != nil {
		o.err = fmt.Errorf("failed to generate ID: %w", err)
	}
	o.Time = time.Unix(o.ID.Time().UnixTime())
	return o
}

func (o OutboxCE) WithTenantID(tid uuid.UUID) OutboxCE {
	o.TenantID = &tid
	return o
}

func (o OutboxCE) WithSubject(sub string) OutboxCE {
	o.Subject = sub
	return o
}

func (o OutboxCE) WithDataSchema(schema string) OutboxCE {
	o.DataSchema = schema
	return o
}

func (o OutboxCE) WithEncryptor(fn func(event.Event) (tink.AEAD, error)) OutboxCE {
	o.AEADFunc = fn
	return o
}

func (o OutboxCE) WithModifier(fn func(o OutboxCE) OutboxCE) OutboxCE {
	return fn(o)
}

var emptyData = []byte{}

func (o OutboxCE) Build() (ce event.Event, err error) {
	if o.err != nil {
		return ce, fmt.Errorf("failed to build: %w", err)
	}

	ce = cloudevents.NewEvent()
	ce.SetID(o.ID.String())
	ce.SetType(o.EventType)
	ce.SetSource(o.Source)
	ce.SetTime(o.Time)
	if o.Subject != "" {
		ce.SetSubject(o.Subject)
	}
	if o.DataSchema != "" {
		ce.SetDataSchema(o.DataSchema)
	}
	if o.TenantID != nil {
		ce.SetExtension(CEExtensionTenantID, o.TenantID.String())
	}

	dct := ContentTypeProtobuf
	if o.Data == nil {
		ce.SetData(dct, emptyData)
		return
	}

	data, err := proto.Marshal(o.Data)
	if err != nil {
		return ce, fmt.Errorf("failed to marshal content: %w", err)
	}
	if o.AEADFunc != nil {
		dct = ContentTypeProtobufEncrypted
		aead, err := o.AEADFunc(ce)
		if err != nil {
			return ce, fmt.Errorf("failed to obtain aead primitive: %w", err)
		}
		data, err = aead.Encrypt(data, []byte(ce.ID()))
		if err != nil {
			return ce, fmt.Errorf("failed to encrypt data: %w", err)
		}
	}
	ce.SetData(dct, data)
	return
}

func FromEvent(e event.Event, aeadFunc AEADFunc, Unmarshaller func([]byte) (proto.Message, error)) (o OutboxCE, err error) {

	o.ID, err = uuid.Parse(e.ID())
	if err != nil {
		return o, fmt.Errorf("failed to parse id : %w", err)
	}

	if v, ok := e.Extensions()[CEExtensionTenantID]; ok {
		tidstr, ok := v.(string)
		if ok {
			tid, err := uuid.Parse(tidstr)
			if err != nil {
				return o, fmt.Errorf("failed to parse tenant id: %w", err)
			}
			o.TenantID = &tid
		}
	}

	o.Subject = e.Subject()
	o.DataSchema = e.DataSchema()
	o.Source = e.Source()
	o.EventType = e.Type()
	o.Time = e.Time()

	d := e.Data()
	if e.DataContentType() == ContentTypeProtobufEncrypted {
		aead, err := aeadFunc(e)
		if err != nil {
			return o, fmt.Errorf("failed to obtain aead primitive: %w", err)
		}
		d, err = aead.Decrypt(d, []byte(e.ID()))
		if err != nil {
			return o, fmt.Errorf("failed to decrypt: %w", err)
		}
		o.AEADFunc = aeadFunc
	}
	o.Data, err = Unmarshaller(d)
	return
}
