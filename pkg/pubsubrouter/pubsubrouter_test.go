package pubsubrouter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
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

	workers cmap.ConcurrentMap[string, chan Job[T]]
}

func newMemPubSub[T any](workerChans cmap.ConcurrentMap[string, chan Job[T]], workerID string) *memPubSub[T] {
	return &memPubSub[T]{
		workerID: workerID,
		jobQueue: make(chan Job[T], 64),

		workers: workerChans,
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workerID := "worker-1"
	workerChans := cmap.New[chan Job[string]]()
	workerChans.Set(workerID, make(chan Job[string], 64))

	kv := newMemKV()
	pubsub := newMemPubSub(workerChans, "worker-1")
	psw := NewPubSubRouter(workerID, kv, pubsub)
	go psw.Listen(ctx)
	defer psw.Close()

	const jobs = 10
	var wg sync.WaitGroup
	wg.Add(jobs)
	for i := range jobs {
		jobID := "job-" + string(rune('A'+i))
		expected := "result-" + jobID

		go func(jobID, expected string) {
			defer wg.Done()

			results, done, err := psw.WaitResult(ctx, jobID)
			if err != nil {
				t.Errorf("WaitResult error: %v", err)
				return
			}
			defer done(ctx)

			// Simulate async job completion
			pubsub.jobQueue <- Job[string]{
				ID:     jobID,
				Result: expected,
				ACK:    func() {},
				NACK:   func() {},
			}

			select {
			case res := <-results:
				if res != expected {
					t.Errorf("unexpected result: got %q want %q", res, expected)
				}
			case <-time.After(5 * time.Minute):
				t.Errorf("timeout waiting for result %s", jobID)
			}
		}(jobID, expected)
	}

	wg.Wait()
}
