package kafka

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce"
)

var _ cloudevents.Client = tclientSendFailed{}

type tclientSendFailed struct {
	t      *testing.T
	failed map[string]*event.Event
}

func (m tclientSendFailed) Send(ctx context.Context, event event.Event) protocol.Result {
	e, ok := m.failed[event.ID()]
	if !ok {
		return protocol.ResultACK
	}
	if e == nil {
		m.t.Errorf("event should not be sent due to error on previous event: %v", event)
	}

	failures := []protocol.Result{
		protocol.ResultNACK,
		&protocol.ErrTransportMessageConversion{},
	}
	return failures[rand.Int()%len(failures)]
}

func (m tclientSendFailed) Request(ctx context.Context, event event.Event) (*event.Event, protocol.Result) {
	m.t.Fatal("should not be called")
	return nil, nil
}

func (m tclientSendFailed) StartReceiver(ctx context.Context, fn interface{}) error {
	m.t.Fatal("should not be called")
	return nil
}

func TestOutboxcePartialErrors(t *testing.T) {
	client := tclientSendFailed{
		t:      t,
		failed: map[string]*event.Event{},
	}

	scenarios := []struct {
		name  string
		total int
		sent  int
	}{
		{
			name:  "NoError",
			total: 10,
			sent:  10,
		},
		{
			name:  "PartialError",
			total: 10,
			sent:  3,
		},
		{
			name:  "AllError",
			total: 10,
			sent:  0,
		},
	}
	for _, d := range scenarios {
		t.Run(d.name, func(t *testing.T) {
			events := []event.Event{}
			for i := 0; i < d.total; i++ {
				e := event.New()
				e.SetID(uuid.NewString())
				e.SetType(t.Name())
				if i == d.sent {
					client.failed[e.ID()] = &e
				}
				if i > d.sent {
					client.failed[e.ID()] = nil
				}
				events = append(events, e)
			}

			k := Kafka{client: client}
			relayErrs := &outboxce.RelayErrors{}
			err := k.OutboxCERelayFunc()(context.Background(), events)
			switch d.total == d.sent {
			case false:
				require.Error(t, err)
				require.True(t, errors.As(err, &relayErrs))
				assert.Len(t, *relayErrs, d.total-d.sent)
				for _, err := range *relayErrs {
					_, ok := client.failed[err.Event.ID()]
					assert.True(t, ok)
				}

			case true:
				require.NoError(t, err)
			}
		})
	}
}
