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
	logger := logtest.NewLogger(t)

	// prepare data
	workerGroupNum, workerGroupReplica, workerChannelNum, workerChannelRequests, channelMsgNum := 10, 7, 8, 5, 50
	channels := map[string][]string{}
	channelIDFunc := func(i int) string { return fmt.Sprintf("job-%d", i) }
	for i := range workerGroupNum * workerChannelNum {
		id := channelIDFunc(i)
		for j := range channelMsgNum {
			result := fmt.Sprintf("result-%d-%d", i, j)
			channels[id] = append(channels[id], result)
		}
	}

	// start pubsub receiver
	var wgStart, wgFinish sync.WaitGroup
	wgStart.Add(workerGroupNum * workerGroupReplica * workerChannelNum * workerChannelRequests)
	wgFinish.Add(workerGroupNum * workerGroupReplica * workerChannelNum * workerChannelRequests)
	for i := range workerGroupNum {
		for j := range workerGroupReplica {
			worker := newWorker(t, *ts, fmt.Sprintf("worker-%d-%d", i, j), logger)
			for k := range workerChannelNum {
				channelID := channelIDFunc(i + (workerGroupNum * k))
				for range workerChannelRequests {
					go func() {
						defer wgFinish.Done()
						worker.handle(t, ctx, channelID, channelMsgNum, &wgStart, channels)
					}()
				}
			}
		}
	}
	wgStart.Wait()

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
	time.AfterFunc(10*time.Second, cancel)
	wgFinish.Wait()

	assert.Equal(t, int32(len(channels)*channelMsgNum), acks.Load())
	if a := nacks.Load(); a > 0 {
		logger.Warn(t.Context(), "non zero nacks", log.Int("message-queue", int(a)))
	}
}

type worker struct {
	workerID string
	psrt     *pubsubrt.PubSubRouter[string]
	logger   log.Logger
}

func newWorker(t *testing.T, ts TestSuiteNormal, workerID string, logger log.Logger) worker {
	psrt, err := pubsubrt.New(workerID, ts.KVFactory, ts.PubSubFactory, pubsubrt.WithLogger[string](logger))
	require.NoError(t, err)

	ch := make(chan struct{})
	go func() {
		close(ch)
		err := psrt.Listen(t.Context())
		if err != nil && err != t.Context().Err() {
			assert.NoError(t, err)
		}
	}()
	<-ch

	return worker{psrt: psrt, workerID: workerID, logger: logger}
}

func (w worker) handle(t *testing.T, ctx context.Context, channelID string, channelMessageNum int, wg *sync.WaitGroup, channels map[string][]string) {
	resultsChan, err := w.psrt.Subscribe(ctx, channelID, channelMessageNum)
	require.NoError(t, err)
	defer func() { resultsChan.Close(ctx) }()
	<-time.After(time.Second)
	wg.Done()

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

	assert.ElementsMatchf(t, expected, actual, "worker-id=%s channel-id=%s", w.workerID, channelID)
	w.logger.Debug(ctx, "receiver done", log.String("worker-id", w.workerID), log.String("channel-id", channelID))
}
