package kafka

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce"
)

func (k *Kafka) OutboxRelayer() outboxce.RelayFunc {
	return func(ctx context.Context, o []event.Event) (err error) {
		for _, o := range o {
			result := k.client.Send(ctx, o)
			if !cloudevents.IsACK(result) {
				return fmt.Errorf("fail to send event: %w", result)
			}
		}
		return
	}
}
