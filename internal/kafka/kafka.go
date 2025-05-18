package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type OptFunc func(k *Kafka) error

func WithBrokers(b []string) OptFunc {
	return func(k *Kafka) (err error) {
		k.brokers = b
		return
	}
}

func WithTopic(t string) OptFunc {
	return func(k *Kafka) (err error) {
		k.topic = t
		return
	}
}

type Kafka struct {
	brokers []string
	topic   string

	sender *kafka_sarama.Sender
	client cloudevents.Client
}

func New(opts ...OptFunc) (k *Kafka, err error) {
	k = &Kafka{}
	for _, opt := range opts {
		if err = opt(k); err != nil {
			return nil, fmt.Errorf("failed to apply options: %w", err)
		}
	}
	if len(k.brokers) == 0 {
		return nil, fmt.Errorf("missing brokers")
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_0_0_0
	sender, err := kafka_sarama.NewSender(k.brokers, saramaConfig, k.topic)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate cloudevents kafka sender")
	}

	k.client, err = cloudevents.NewClient(sender, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate cloudevents client")
	}
	return
}

func (k *Kafka) Close(ctx context.Context) error {
	if k.sender == nil {
		return nil
	}
	return k.sender.Close(ctx)
}
