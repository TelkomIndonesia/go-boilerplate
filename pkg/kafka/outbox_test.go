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

var _ cloudevents.Client = mockClient{}

type mockClient struct {
	t       *testing.T
	errored map[string]event.Event
}

// Request implements client.Client.
func (m mockClient) Request(ctx context.Context, event event.Event) (*event.Event, protocol.Result) {
	panic("unimplemented")
}

// Send implements client.Client.
func (m mockClient) Send(ctx context.Context, event event.Event) protocol.Result {
	_, ok := m.errored[event.ID()]
	if !ok {
		return protocol.ResultACK
	}
	if rand.Int()%2 == 0 {
		return protocol.ResultNACK
	} else {
		return &protocol.ErrTransportMessageConversion{}
	}
}

// StartReceiver implements client.Client.
func (m mockClient) StartReceiver(ctx context.Context, fn interface{}) error {
	panic("unimplemented")
}

func TestOutboxcePartialErrors(t *testing.T) {
	client := mockClient{
		t:       t,
		errored: map[string]event.Event{},
	}

	total, sent := 10, 5
	events := []event.Event{}
	for i := 0; i < total; i++ {
		e := event.New()
		e.SetID(uuid.NewString())
		e.SetType(t.Name())
		if i >= sent {
			client.errored[e.ID()] = e
		}
		events = append(events, e)
	}

	k := Kafka{client: client}
	relayErrs := &outboxce.RelayErrors{}
	err := k.OutboxceRelayFunc()(context.Background(), events)
	require.Error(t, err)
	require.True(t, errors.As(err, &relayErrs))
	assert.Len(t, *relayErrs, total-sent)
	for _, err := range *relayErrs {
		_, ok := client.errored[err.Event.ID()]
		assert.True(t, ok)
	}
}
