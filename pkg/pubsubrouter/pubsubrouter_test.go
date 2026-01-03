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
	t.Cleanup(cancel)

	var wgWorkerStart, wgWorkerFinish sync.WaitGroup
	wgWorkerStart.Add(100)
	wgWorkerFinish.Add(100)
	for i := range 10 {
		workerID := fmt.Sprintf("worker-%d", i)
		psw := NewPubSubRouter(workerID, kv, pubsub.Clone(workerID), logger)
		go func() {
			err := psw.Listen(ctx)
			if err != nil && err != ctx.Err() {
				assert.NoError(t, err)
			}
		}()

		for j := range 10 {
			jobID := jobIDFunc(i + (10 * j))
			go func() {
				defer wgWorkerFinish.Done()

				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				resultsChan, err := psw.WaitResult(ctx, jobID)
				require.NoError(t, err)
				defer func() { resultsChan.Close(t.Context()) }()
				wgWorkerStart.Done()
				logger.Debug(ctx, "waiting started", log.String("worker-id", workerID), log.String("job-id", jobID))

				expected := jobs[jobID]
				results := []string{}
				for len(results) < len(expected) {
					var result string
					var cont bool

					select {
					case <-ctx.Done():
					case result, cont = <-resultsChan.Chan():
					}

					if !cont {
						break
					}

					results = append(results, result)
				}

				assert.ElementsMatch(t, expected, results, workerID, jobID)
				logger.Debug(ctx, "waiting done", log.String("worker-id", workerID), log.String("job-id", jobID))
			}()
		}
	}
	wgWorkerStart.Wait()

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

	time.AfterFunc(5*time.Second, cancel)
	wgWorkerFinish.Wait()
}
