package kafka

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testKafka *Kafka
var testKafkaSync = sync.Mutex{}

func getTestKafka(t *testing.T) *Kafka {
	if testKafka == nil {
		testKafkaSync.Lock()
		defer testKafkaSync.Unlock()
	}
	if testKafka == nil {
		testKafka = newTestKafka(t)
	}
	return testKafka
}

func newTestKafka(t *testing.T) *Kafka {
	v, ok := os.LookupEnv("KAFKA_BROKERS")
	if !ok {
		t.Skip("no kafka brokers was defined in env var")
	}
	k, err := New(WithBrokers([]string{v}))
	require.NoError(t, err, "should instantiate kafka")
	return k
}

func TestReadWrite(t *testing.T) {
	ctx := context.Background()

	k := getTestKafka(t)

	conn, err := kafka.DialLeader(ctx, "tcp", os.Getenv("KAFKA_BROKERS"), "test", 0)
	require.NoError(t, err, "should dial kafka", err)
	conn.Controller()
	defer conn.Close()

	err = k.Write(ctx, "test",
		Message{Value: []byte("hello")},
		Message{Topic: "test", Value: []byte("world")},
	)
	require.NoError(t, err, "should successfully write to kafka")

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{os.Getenv("KAFKA_BROKERS")},
		Topic:     "test",
		Partition: 0,
		MaxBytes:  10e6, // 10MB
	})
	m1, err := r.ReadMessage(ctx)
	assert.NoError(t, err, "should read first message")
	assert.Equal(t, m1.Value, []byte("hello"))
	m2, err := r.ReadMessage(ctx)
	assert.NoError(t, err, "should read second message")
	assert.Equal(t, m2.Value, []byte("world"))
}
