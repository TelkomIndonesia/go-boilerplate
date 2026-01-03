package pubsubrouter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logtest"
)

//
// --------------------
// In-memory KV repo
// --------------------
//

type memKV struct {
	m cmap.ConcurrentMap[string, string]
}

func newMemKV() *memKV {
	return &memKV{m: cmap.New[string]()}
}

func (k *memKV) Register(ctx context.Context, key, value string) error {
	k.m.Set(key, value)
	return nil
}

func (k *memKV) UnRegister(ctx context.Context, key string) error {
	k.m.Remove(key)
	return nil
}

func (k *memKV) Lookup(ctx context.Context, key string) (string, error) {
	v, _ := k.m.Get(key)
	return v, nil
}

type memPubSub[T any] struct {
	workerID string
	jobQueue chan Job[T]
	workers  cmap.ConcurrentMap[string, chan Job[T]]
}

func newMemPubSub[T any](workerID string) *memPubSub[T] {
	ps := &memPubSub[T]{
		workerID: workerID,
		jobQueue: make(chan Job[T]),

		workers: cmap.New[chan Job[T]](),
	}
	ps.workers.Set(workerID, make(chan Job[T]))
	return ps
}

func (m *memPubSub[T]) Clone(workerID string) PubSubSvc[T] {
	m.workers.Set(workerID, make(chan Job[T]))
	return &memPubSub[T]{
		workerID: workerID,
		jobQueue: m.jobQueue,
		workers:  m.workers,
	}
}
func (m *memPubSub[T]) JobQueue(ctx context.Context) (<-chan Job[T], error) {
	return m.jobQueue, nil
}

func (m *memPubSub[T]) WorkerChannel(ctx context.Context) (<-chan Job[T], error) {
	ch, ok := m.workers.Get(m.workerID)
	if !ok {
		return nil, fmt.Errorf("no channel for %s", m.workerID)
	}
	return ch, nil
}

func (m *memPubSub[T]) PublishResult(
	ctx context.Context,
	workerID string,
	jobID string,
	result T,
) error {
	ch, ok := m.workers.Get(workerID)
	if !ok {
		return fmt.Errorf("can't publish for %s", workerID)
	}

	ch <- Job[T]{
		ID:     jobID,
		Result: result,
		ACK:    func() {},
		NACK:   func() {},
	}
	return nil
}

func TestMultipleWaitersReceiveResults(t *testing.T) {
	logger := logtest.NewLogger(t)

	kv := newMemKV()
	pubsub := newMemPubSub[string]("_")

	jobs := map[string][]string{}
	jobIDFunc := func(i int) string {
		return fmt.Sprintf("job-%d", i)
	}
	for i := range 100 {
		id := jobIDFunc(i)
		for j := range 10 {
			result := fmt.Sprintf("result-%s-%d", id, j)
			jobs[id] = append(jobs[id], result)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())

	var wgWorker, wgWorkerStarted, wgWorkerDone sync.WaitGroup
	defer wgWorker.Wait()
	wgWorker.Add(10)
	wgWorkerStarted.Add(100)
	wgWorkerDone.Add(100)
	for i := range 10 {
		workerID := fmt.Sprintf("worker-%d", i)
		go func() {
			defer wgWorker.Done()

			psw := NewPubSubRouter(workerID, kv, pubsub.Clone(workerID), logger)
			defer psw.Close()
			go psw.Listen(ctx)

			wg := sync.WaitGroup{}
			defer wg.Wait()
			wg.Add(10)
			for j := range 10 {
				jobID := jobIDFunc(i + (10 * j))
				go func() {
					defer wg.Done()

					resultsChan, done, err := psw.WaitResult(ctx, jobID)
					require.NoError(t, err)
					defer done(ctx)
					wgWorkerStarted.Done()
					logger.Debug(ctx, "start", log.String("worker-id", workerID), log.String("job-id", jobID))

					expected := jobs[jobID]
					results := []string{}
					for {
						stop := false

						select {
						case <-ctx.Done():
							t.Errorf("timeout waiting for result %s %s", workerID, jobID)
							stop = true

						case res, ok := <-resultsChan:
							if ok {
								logger.Debug(ctx, "result", log.String("worker-id", workerID), log.String("job-id", jobID), log.String("result", res))
								results = append(results, res)
							} else {
								logger.Debug(ctx, "channel closed", log.String("worker-id", workerID), log.String("job-id", jobID))
							}
							stop = !ok || len(results) == len(expected)
						}

						if stop {
							break
						}
					}
					assert.ElementsMatch(t, expected, results, workerID, jobID)
					wgWorkerDone.Done()
					logger.Debug(ctx, "done", log.String("worker-id", workerID), log.String("job-id", jobID))
				}()
			}
		}()
	}

	wgWorkerStarted.Wait()

	var wgJobs sync.WaitGroup
	wgJobs.Add(len(jobs))
	for id, results := range jobs {
		jobID := id
		go func() {
			defer wgJobs.Done()
			for _, result := range results {
				logger.Debug(t.Context(), "publish", log.String("job-id", jobID), log.String("result", result))
				pubsub.jobQueue <- Job[string]{
					ID:     jobID,
					Result: result,
					ACK:    func() {},
					NACK:   func() {},
				}
			}
		}()
	}
	wgJobs.Wait()

	time.AfterFunc(10*time.Second, cancel)
	wgWorkerDone.Wait()
}
