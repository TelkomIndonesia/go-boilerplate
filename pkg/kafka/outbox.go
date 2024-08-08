package kafka

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce"
)

func (k *Kafka) OutboxRelayer() outboxce.RelayFunc {
	return func(ctx context.Context, events []event.Event) (err error) {
		idxErr := 0
		for i, e := range events {
			result := k.client.Send(ctx, e)
			if !cloudevents.IsACK(result) {
				err, idxErr = result, i
				break
			}
		}

		if err != nil {
			relayErrs := make(outboxce.RelayErrors, 0, len(events)-idxErr)
			for _, e := range events[idxErr:] {
				relayErrs = append(relayErrs, &outboxce.RelayError{Event: e, Err: err})
				err = nil
			}
			return &relayErrs
		}

		return
	}
}
