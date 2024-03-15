package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type OptFunc func(k *Kafka) error

func WithBrokers(b []string) OptFunc {
	return func(k *Kafka) (err error) {
		k.brokers = b
		return
	}
}

func WithDefaultTopic(t string) OptFunc {
	return func(k *Kafka) (err error) {
		k.topic = t
		return
	}
}

type Kafka struct {
	brokers []string
	topic   string

	writer *kafka.Writer
}

func New(opts ...OptFunc) (k *Kafka, err error) {
	k = &Kafka{}
	for _, opt := range opts {
		if err = opt(k); err != nil {
			return nil, fmt.Errorf("fail to apply options: %w", err)
		}
	}

	dialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
	}
	k.writer = kafka.NewWriter(kafka.WriterConfig{
		Brokers:  k.brokers,
		Balancer: &kafka.Hash{},
		Dialer:   dialer,
	})

	return
}

func (k *Kafka) Write(ctx context.Context, topic string, msgs ...Message) (err error) {
	if topic == "" {
		topic = k.topic
	}
	kmsgs := make([]kafka.Message, 0, len(msgs))
	for _, msg := range msgs {
		kmsgs = append(kmsgs, msg.toKafkaMessage(topic))
	}

	err = k.writer.WriteMessages(ctx, kmsgs...)
	if err != nil {
		return fmt.Errorf("fail to write message: %w:", err)
	}
	return nil
}

type Message struct {
	Topic string
	Key   []byte
	Value []byte
}

func (m Message) toKafkaMessage(topic string) kafka.Message {
	kmsg := kafka.Message{
		Topic: m.Topic,
		Key:   m.Key,
		Value: m.Value,
	}
	if kmsg.Topic == "" {
		kmsg.Topic = topic
	}
	return kmsg
}

func (k *Kafka) Close(ctx context.Context) error {
	return k.writer.Close()
}
