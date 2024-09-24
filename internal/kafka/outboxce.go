package kafka

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce"
)

func (k *Kafka) OutboxCERelayFunc() outboxce.RelayFunc {
	return func(ctx context.Context, events []event.Event) (err error) {
		idxErr := 0
		for i, event := range events {
			result := k.client.Send(
				kafka_sarama.WithMessageKey(context.Background(), sarama.StringEncoder(event.Subject())),
				event,
			)
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
