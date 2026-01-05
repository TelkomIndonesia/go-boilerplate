package testsuite

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logtest"
	"github.com/telkomindonesia/go-boilerplate/pkg/pubsubrt"
)

type TestSuiteNormal struct {
	KVFactory     func() pubsubrt.KeyValueSvc
	PubSubFactory func(workerID string) pubsubrt.PubSubSvc[string]

	PublishToMessageQueue func(msg pubsubrt.Message[string])

	Logger log.Logger
}

func (ts *TestSuiteNormal) Run(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	logger := logtest.NewLogger(t)

	workersNum, workerJobsNum, channelMessageNum := 10, 30, 50

	channels := map[string][]string{}
	channelIDFunc := func(i int) string { return fmt.Sprintf("job-%d", i) }
	for i := range workersNum * workerJobsNum {
		id := channelIDFunc(i)
		for j := range channelMessageNum {
			result := fmt.Sprintf("result-%s-%d", id, j)
			channels[id] = append(channels[id], result)
		}
	}

	// start pubsub receiver
	var wgReceiverStart, wgReceiverFinish sync.WaitGroup
	wgReceiverStart.Add(workersNum * workerJobsNum)
	wgReceiverFinish.Add(workersNum * workerJobsNum)
	for i := range workersNum {
		workerID := fmt.Sprintf("worker-%d", i)
		psw, err := pubsubrt.New(workerID, ts.KVFactory, ts.PubSubFactory, pubsubrt.WithLogger[string](logger))
		require.NoError(t, err)
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

				for len(messages) < len(expected) {
					select {
					case <-ctx.Done():
					case message, ok := <-resultsChan.Messages():
						if !ok {
							break
						}

						messages = append(messages, message.Content)
						message.ACK()

						resultsChan, messages = ts.randomTakeOverfunc(t, ctx, *psw, resultsChan, channelID, messages)
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
				msg := pubsubrt.Message[string]{
					ChannelID: channelID,
					Content:   content,
					ACK:       func() { acks.Add(1) },
				}
				msg.NACK = func(pubsubrt.NACKReason) {
					nacks.Add(1)
					go func() { ts.PublishToMessageQueue(msg) }()
				}
				ts.PublishToMessageQueue(msg)
			}
		}()
	}
	time.AfterFunc(5*time.Second, cancel)
	wgReceiverFinish.Wait()

	assert.Equal(t, int32(len(channels)*channelMessageNum), acks.Load())
	if a := nacks.Load(); a > 0 {
		logger.Warn(t.Context(), "non zero nacks", log.Int("message-queue", int(a)))
	}
}

func (ts *TestSuiteNormal) randomTakeOverfunc(
	t *testing.T,
	ctx context.Context,
	psw pubsubrt.PubSubRouter[string],
	oldChannel pubsubrt.Channel[string],
	channelID string,
	msgs []string,
) (
	channel pubsubrt.Channel[string],
	messages []string,
) {
	messages = msgs
	channel = oldChannel

	if rand.Int()%3 != 0 {
		return
	}

	defer oldChannel.Close(t.Context())

	channel, err := psw.Subscribe(ctx, channelID, 0)
	require.NoError(t, err)
	for {
		select {
		default:
			return

		case message, ok := <-oldChannel.Messages():
			if !ok {
				return
			}

			messages = append(messages, message.Content)
			message.ACK()
		}
	}
}
