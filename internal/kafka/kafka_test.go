package kafka

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testKafka *Kafka
var testKafkaSync = sync.Mutex{}

func tGetKafka(t *testing.T) *Kafka {
	if testKafka == nil {
		testKafkaSync.Lock()
		defer testKafkaSync.Unlock()
	}
	if testKafka == nil {
		testKafka = tNewKafka(t)
	}
	return testKafka
}

func tNewKafka(t *testing.T) *Kafka {
	v, ok := os.LookupEnv("TEST_KAFKA_BROKERS")
	if !ok {
		t.Skip("no kafka brokers was defined in env var")
	}
	k, err := New(WithBrokers([]string{v}), WithTopic(t.Name()+uuid.NewString()))
	require.NoError(t, err, "should instantiate kafka")

	tCreateTopic(t, k)
	return k
}

func tCreateTopic(t *testing.T, k *Kafka) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_1_0_0
	admin, err := sarama.NewClusterAdmin(k.brokers, config)
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close() })
	err = admin.CreateTopic(k.topic, &sarama.TopicDetail{
		NumPartitions:     1,
		ReplicationFactor: 1,
	}, false)
	require.NoError(t, err)
}

func TestReadWrite(t *testing.T) {
	ctx := context.Background()
	k := tGetKafka(t)

	events := map[string]event.Event{}
	for i := 0; i < 10; i++ {
		e := event.New()
		e.SetID(uuid.NewString())
		e.SetSource("test/" + t.Name())
		e.SetType("test")
		e.SetData(*cloudevents.StringOfApplicationJSON(), map[string]interface{}{
			"helo": "world",
			"time": time.Now(),
		})
		events[e.ID()] = e
	}

	receivedEvents := map[string]event.Event{}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	go func() {
		defer cancel()

		saramaConfig := sarama.NewConfig()
		saramaConfig.Version = sarama.V2_0_0_0
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
		receiver, err := kafka_sarama.NewConsumer(k.brokers, saramaConfig, t.Name()+uuid.NewString(), k.topic)
		require.NoError(t, err)
		defer receiver.Close(ctx)
		c, err := cloudevents.NewClient(receiver)
		require.NoError(t, err)

		err = c.StartReceiver(ctx, func(ctx context.Context, event cloudevents.Event) {
			receivedEvents[event.ID()] = event

			if len(receivedEvents) == len(events) {
				cancel()
			}
		})
		require.NoError(t, err)
	}()

	tosents := []event.Event{}
	for _, e := range events {
		tosents = append(tosents, e)
	}
	err := k.OutboxCERelayFunc()(ctx, tosents)
	require.NoError(t, err)

	<-ctx.Done()
	assert.Equal(t, len(events), len(receivedEvents))
	for _, v := range receivedEvents {
		exp := events[v.ID()]
		require.NotNil(t, exp)
		assert.Equal(t, exp.Source(), v.Source())
		assert.Equal(t, exp.Type(), v.Type())
		assert.Equal(t, exp.DataContentType(), v.DataContentType())
		assert.Equal(t, exp.Data(), v.Data())
	}
}
