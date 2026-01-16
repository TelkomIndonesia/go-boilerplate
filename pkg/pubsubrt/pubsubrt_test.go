package pubsubrt_test

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logtest"
	"github.com/telkomindonesia/go-boilerplate/pkg/pubsubrt"
	"github.com/telkomindonesia/go-boilerplate/pkg/pubsubrt/testsuite"
)

type memKV struct {
	m cmap.ConcurrentMap[string, *[]string]
}

func newMemKV() *memKV {
	return &memKV{m: cmap.New[*[]string]()}
}

func (k *memKV) SAdd(ctx context.Context, key, value string) error {
	k.m.Upsert(key, nil, func(exist bool, valueInMap, newValue *[]string) *[]string {
		if !exist || valueInMap == nil || len(*valueInMap) == 0 {
			return &[]string{value}
		}

		if slices.Contains(*valueInMap, value) {
			return valueInMap
		}

		*valueInMap = append(*valueInMap, value)
		return valueInMap
	})
	return nil
}

func (k *memKV) SRem(ctx context.Context, key string, value string) error {
	k.m.RemoveCb(key, func(key string, current *[]string, exists bool) bool {
		if !exists || current == nil || len(*current) == 0 {
			return true
		}
		for i, v := range *current {
			if v == value {
				*current = append((*current)[:i], (*current)[i+1:]...)
			}
		}
		return len(*current) == 0
	})
	return nil
}

func (k *memKV) SGet(ctx context.Context, key string) (res []string, err error) {
	k.m.RemoveCb(key, func(key string, v *[]string, exists bool) bool {
		if exists {
			res = *v
		}
		return false
	})

	return
}

type memPubSub[T any] struct {
	t     *testing.T
	acks  *atomic.Int32
	nacks *atomic.Int32

	workerID string
	jobQueue chan pubsubrt.Message[T]
	workers  cmap.ConcurrentMap[string, chan pubsubrt.Message[T]]
}

func newMemPubSub[T any](t *testing.T) *memPubSub[T] {
	ps := &memPubSub[T]{
		t:     t,
		acks:  &atomic.Int32{},
		nacks: &atomic.Int32{},

		jobQueue: make(chan pubsubrt.Message[T]),
		workers:  cmap.New[chan pubsubrt.Message[T]](),
	}
	return ps
}

func (m *memPubSub[T]) Clone(workerID string) pubsubrt.PubSubSvc[T] {
	m.workers.Set(workerID, make(chan pubsubrt.Message[T]))
	return &memPubSub[T]{
		t:        m.t,
		acks:     m.acks,
		nacks:    m.nacks,
		workerID: workerID,
		jobQueue: m.jobQueue,
		workers:  m.workers,
	}
}
func (m *memPubSub[T]) MessageQueue(ctx context.Context) (<-chan pubsubrt.Message[T], error) {
	return m.jobQueue, nil
}

func (m *memPubSub[T]) WorkerChannel(ctx context.Context) (<-chan pubsubrt.Message[T], error) {
	ch, ok := m.workers.Get(m.workerID)
	if !ok {
		return nil, fmt.Errorf("no channel for %s", m.workerID)
	}
	return ch, nil
}

func (m *memPubSub[T]) PublishWorkerMessage(
	ctx context.Context,
	workerID string,
	channelID string,
	message T,
) error {
	ch, ok := m.workers.Get(workerID)
	if !ok {
		return fmt.Errorf("can't publish for %s", workerID)
	}

	msg := pubsubrt.Message[T]{
		ChannelID: channelID,
		Content:   message,
		ACK: func() {
			m.acks.Add(1)
		},
	}
	msg.NACK = func(pubsubrt.NACKReason) {
		m.nacks.Add(1)
		go func() { ch <- msg }()
	}

	ch <- msg
	return nil
}

func TestMultipleWaitersReceiveResults(t *testing.T) {
	kv := newMemKV()
	basepubsub := newMemPubSub[string](t)

	ts := &testsuite.TestSuiteNormal{
		KVFactory:     func() pubsubrt.KeyValueSvc { return kv },
		PubSubFactory: func(workerID string) pubsubrt.PubSubSvc[string] { return basepubsub.Clone(workerID) },
		Logger:        logtest.NewLogger(t),
		PublishToMessageQueue: func(msg pubsubrt.Message[string]) {
			basepubsub.jobQueue <- msg
		},
	}
	ts.Run(t)

}
