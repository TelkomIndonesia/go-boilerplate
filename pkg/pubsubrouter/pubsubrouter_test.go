package pubsubrouter

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logtest"
)

type memKV struct {
	m cmap.ConcurrentMap[string, string]
}

func newMemKV() *memKV {
	return &memKV{m: cmap.New[string]()}
}

func (k *memKV) Set(ctx context.Context, key, value string) error {
	k.m.Set(key, value)
	return nil
}

func (k *memKV) Remove(ctx context.Context, key string) error {
	k.m.Remove(key)
	return nil
}

func (k *memKV) Get(ctx context.Context, key string) (string, error) {
	v, _ := k.m.Get(key)
	return v, nil
}

type memPubSub[T any] struct {
	t     *testing.T
	acks  *atomic.Int32
	nacks *atomic.Int32

	workerID string
	jobQueue chan Message[T]
	workers  cmap.ConcurrentMap[string, chan Message[T]]
}

func newMemPubSub[T any](t *testing.T) *memPubSub[T] {
	ps := &memPubSub[T]{
		t:     t,
		acks:  &atomic.Int32{},
		nacks: &atomic.Int32{},

		jobQueue: make(chan Message[T]),
		workers:  cmap.New[chan Message[T]](),
	}
	return ps
}

func (m *memPubSub[T]) Clone(workerID string) PubSubSvc[T] {
	m.workers.Set(workerID, make(chan Message[T]))
	return &memPubSub[T]{
		t:        m.t,
		acks:     m.acks,
		nacks:    m.nacks,
		workerID: workerID,
		jobQueue: m.jobQueue,
		workers:  m.workers,
	}
}
func (m *memPubSub[T]) MessageQueue(ctx context.Context) (<-chan Message[T], error) {
	return m.jobQueue, nil
}

func (m *memPubSub[T]) WorkerChannel(ctx context.Context) (<-chan Message[T], error) {
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
	result T,
) error {
	ch, ok := m.workers.Get(workerID)
	if !ok {
		return fmt.Errorf("can't publish for %s", workerID)
	}

	msg := Message[T]{
		ChannelID: channelID,
		Content:   result,
		ACK: func() {
			m.acks.Add(1)
		},
	}
	msg.NACK = func() {
		m.nacks.Add(1)
		go func() { ch <- msg }()
	}

	ch <- msg
	return nil
}

func TestMultipleWaitersReceiveResults(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	logger := logtest.NewLogger(t)

	workersNum := 10
	workerJobsNum := 50
	channelMessage := 50

	channels := map[string][]string{}
	channelIDFunc := func(i int) string {
		return fmt.Sprintf("job-%d", i)
	}
	for i := range workersNum * workerJobsNum {
		id := channelIDFunc(i)
		for j := range channelMessage {
			result := fmt.Sprintf("result-%s-%d", id, j)
			channels[id] = append(channels[id], result)
		}
	}

	kv := newMemKV()
	basepubsub := newMemPubSub[string](t)

	// start pubsub receiver
	var wgReceiverStart, wgReceiverFinish sync.WaitGroup
	wgReceiverStart.Add(workersNum * workerJobsNum)
	wgReceiverFinish.Add(workersNum * workerJobsNum)
	for i := range workersNum {
		workerID := fmt.Sprintf("worker-%d", i)
		psw := New(workerID, kv, basepubsub.Clone(workerID), logger)
		go func() {
			err := psw.Listen(ctx)
			if err != nil && err != ctx.Err() {
				assert.NoError(t, err)
			}
		}()

		for j := range workerJobsNum {
			channelID := channelIDFunc(i + (workersNum * j))
			go func() {
				defer wgReceiverFinish.Done()

				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				resultsChan, err := psw.Subscribe(ctx, channelID, 0)
				require.NoError(t, err)
				defer func() { resultsChan.Close(t.Context()) }()
				wgReceiverStart.Done()

				expected := channels[channelID]
				messages := []string{}

				randomTakeOver := func(m []string) (messages []string) {
					messages = m

					if rand.Int()%3 != 0 {
						return
					}

					oldChan := resultsChan
					defer oldChan.Close(t.Context())

					resultsChan, err = psw.Subscribe(ctx, channelID, 0)
					require.NoError(t, err)

					for {
						select {
						default:
							return

						case message, ok := <-oldChan.Messages():
							if !ok {
								return
							}

							messages = append(messages, message.Content)
							message.ACK()
						}
					}
				}

				for len(messages) < len(expected) {
					select {
					case <-ctx.Done():
					case message, ok := <-resultsChan.Messages():
						if !ok {
							break
						}

						messages = append(messages, message.Content)
						message.ACK()

						messages = randomTakeOver(messages)
						continue
					}
					break
				}

				assert.ElementsMatch(t, expected, messages, workerID, channelID)
				logger.Debug(ctx, "receiver done", log.String("worker-id", workerID), log.String("channel-id", channelID))
			}()
		}
	}
	wgReceiverStart.Wait()

	// simulate publish channel's messages
	var acks, nacks atomic.Int32
	for id, channel := range channels {
		channelID := id
		go func() {
			for _, content := range channel {
				logger.Debug(t.Context(), "publish", log.String("channel-id", channelID), log.String("content", content))
				msg := Message[string]{
					ChannelID: channelID,
					Content:   content,
					ACK:       func() { acks.Add(1) },
				}
				msg.NACK = func() {
					nacks.Add(1)
					go func() { basepubsub.jobQueue <- msg }()
				}
				basepubsub.jobQueue <- msg
			}
		}()
	}
	time.AfterFunc(5*time.Second, cancel)
	wgReceiverFinish.Wait()
	assert.Equal(t, int32(len(channels)*channelMessage), acks.Load())
	assert.Equal(t, int32(len(channels)*channelMessage), basepubsub.acks.Load())
	if a, b := nacks.Load(), basepubsub.nacks.Load(); a > 0 || b > 0 {
		logger.Warn(t.Context(), "non zero nacks", log.Int("message-queue", int(a)), log.Int("worker-channel", int(b)))
	}
}
