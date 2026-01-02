package pubsubrouter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		jobQueue: make(chan Job[T], 10000),

		workers: cmap.New[chan Job[T]](),
	}
	ps.workers.Set(workerID, make(chan Job[T], 1000))
	return ps
}

func (m *memPubSub[T]) Clone(workerID string) PubSubSvc[T] {
	m.workers.Set(workerID, make(chan Job[T], 1000))
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
	kv := newMemKV()
	pubsub := newMemPubSub[string]("_")

	jobs := map[string][]string{}
	jobIDFunc := func(i int) string {
		return "job-" + string(rune('A'+i))
	}
	for i := range 100 {
		id := jobIDFunc(i)
		for j := range 10 {
			result := fmt.Sprintf("result-%s-%d", id, j)
			jobs[id] = append(jobs[id], result)
		}
	}

	var wgWait sync.WaitGroup
	defer wgWait.Wait()
	wgWait.Add(10)
	for i := range 10 {
		workerID := fmt.Sprintf("worker-%d", i)
		go func() {
			defer wgWait.Done()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			psw := NewPubSubRouter(workerID, kv, pubsub.Clone(workerID))
			defer psw.Close()
			go psw.Listen(ctx)

			for j := range 10 {
				jobID := jobIDFunc(i + (10 * j))
				go func() {

					expected := jobs[jobID]

					slog.Default().Info("start", "worker-id", workerID, "job-id", jobID)

					resultsChan, done, err := psw.WaitResult(ctx, jobID)
					require.NoError(t, err)

					results := []string{}

					timer := time.After(10 * time.Second)
					for {
						stop := false

						select {
						case res, ok := <-resultsChan:
							if ok {
								slog.Default().Info("result", "worker-id", workerID, "job-id", jobID, "result", res)
								results = append(results, res)
							}
							stop = !ok || len(results) == len(expected)
						case <-timer:
							t.Errorf("timeout waiting for result %s %s", workerID, jobID)
							stop = true
						}

						if stop {
							break
						}
					}
					assert.ElementsMatch(t, expected, results, workerID, jobID)
					done(ctx)
					slog.Default().Info("done", "worker-id", workerID, "job-id", jobID)

				}()

			}
		}()
	}

	var wgJobs sync.WaitGroup
	defer wgJobs.Wait()
	wgJobs.Add(len(jobs))
	for id, results := range jobs {
		jobID := id
		go func() {
			defer wgJobs.Done()
			for _, result := range results {
				slog.Default().Info("publish", "job-id", jobID, "result", result)
				pubsub.jobQueue <- Job[string]{
					ID:     jobID,
					Result: result,
					ACK:    func() {},
					NACK:   func() {},
				}
			}
		}()
	}
}
