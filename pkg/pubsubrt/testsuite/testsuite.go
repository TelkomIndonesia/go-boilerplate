package testsuite

import (
	"context"
	"fmt"
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

	// prepare data
	workerGroupNum, workerGroupReplica, workerChannelNum, workerChannelRequests, channelMessageNum := 10, 7, 8, 5, 50
	channels := map[string][]string{}
	channelIDFunc := func(i int) string { return fmt.Sprintf("job-%d", i) }
	for i := range workerGroupNum * workerChannelNum {
		id := channelIDFunc(i)
		for j := range channelMessageNum {
			result := fmt.Sprintf("result-%s-%d", id, j)
			channels[id] = append(channels[id], result)
		}
	}

	// start pubsub receiver
	var wgReceiverStart, wgReceiverFinish sync.WaitGroup
	wgReceiverStart.Add(workerGroupNum * workerGroupReplica * workerChannelNum * workerChannelRequests)
	wgReceiverFinish.Add(workerGroupNum * workerGroupReplica * workerChannelNum * workerChannelRequests)
	for i := range workerGroupNum {
		for j := range workerGroupReplica {
			workerID := fmt.Sprintf("worker-%d-%d", i, j)
			psr, err := pubsubrt.New(workerID, ts.KVFactory, ts.PubSubFactory, pubsubrt.WithLogger[string](logger))
			require.NoError(t, err)

			go func() {
				err := psr.Listen(ctx)
				if err != nil && err != ctx.Err() {
					assert.NoError(t, err)
				}
			}()

			for k := range workerChannelNum {
				channelID := channelIDFunc(i + (workerGroupNum * k))

				for range workerChannelRequests {
					go func() {
						defer wgReceiverFinish.Done()

						ctx, cancel := context.WithCancel(ctx)
						defer cancel()

						resultsChan, err := psr.Subscribe(ctx, channelID, channelMessageNum)
						require.NoError(t, err)
						defer func() { resultsChan.Close(t.Context()) }()
						wgReceiverStart.Done()

						expected := channels[channelID]
						actual := []string{}
						for len(actual) < len(expected) {
							var message pubsubrt.Message[string]
							var ok bool

							select {
							case <-ctx.Done():
							case message, ok = <-resultsChan.Messages():
							}

							if !ok {
								break
							}
							actual = append(actual, message.Content)
							message.ACK()
						}

						assert.ElementsMatch(t, expected, actual, workerID, channelID)
						logger.Debug(ctx, "receiver done", log.String("worker-id", workerID), log.String("channel-id", channelID))
					}()
				}
			}
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
