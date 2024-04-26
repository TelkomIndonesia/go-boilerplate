package kafka

import (
	"context"
	"fmt"
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
	v, ok := os.LookupEnv("TEST_KAFKA_BROKERS")
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

	conn, err := kafka.DialLeader(ctx, "tcp", os.Getenv("TEST_KAFKA_BROKERS"), "test", 0)
	require.NoError(t, err, "should dial kafka", err)
	conn.Controller()
	defer conn.Close()

	msgs := [][]byte{
		[]byte("hello"),
		[]byte("world"),
	}

	err = k.Write(ctx, "test",
		Message{Value: msgs[0]},
		Message{Topic: "test", Value: msgs[1]},
	)
	require.NoError(t, err, "should successfully write to kafka")

	rmsgs := [][]byte{}
	group := t.Name()
	for i, _ := range msgs {
		t.Run(fmt.Sprintf("read-%d", i), func(t *testing.T) {
			r := kafka.NewReader(kafka.ReaderConfig{
				Brokers:   []string{os.Getenv("TEST_KAFKA_BROKERS")},
				Topic:     "test",
				Partition: 0,
				MaxBytes:  10e6, // 10MB
				GroupID:   group,
			})
			defer r.Close()
			m, err := r.FetchMessage(ctx)
			assert.NoError(t, err, "should read message")
			assert.NotNil(t, m.Value, "should not nil")
			rmsgs = append(rmsgs, m.Value)
			assert.NoError(t, r.CommitMessages(ctx, m), "should commit message")
		})
	}
	assert.ElementsMatch(t, msgs, rmsgs, "should read all message")
}
