package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce"
)

func (k *Kafka) OutboxRelayer() outboxce.RelayFunc {
	return func(ctx context.Context, o []event.Event) (err error) {
		msgs := make([]Message, 0, len(o))
		for _, o := range o {
			b, err := json.Marshal(o)
			if err != nil {
				return fmt.Errorf("fail to marshal: %w", err)
			}

			msgs = append(msgs, Message{Value: b})
		}
		return k.Write(ctx, k.topic, msgs...)
	}
}
