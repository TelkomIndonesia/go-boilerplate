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
	jobQueue chan Job[T]

	workers cmap.ConcurrentMap[string, chan Job[T]]
}

func newMemPubSub[T any]() *memPubSub[T] {
	return &memPubSub[T]{
		jobQueue: make(chan Job[T], 64),
		workers:  cmap.New[chan Job[T]](),
	}
}

func (m *memPubSub[T]) JobQueue(ctx context.Context) (<-chan Job[T], error) {
	return m.jobQueue, nil
}

func (m *memPubSub[T]) WorkerChannel(ctx context.Context) (<-chan Job[T], error) {

	workerID := ctx.Value("workerID").(string)
	ch := make(chan Job[T], 64)
	m.workers.Set(workerID, ch)
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

	kv := newMemKV()
	pubsub := newMemPubSub[string]()

	workerID := "worker-1"

	psw := NewPubSubWait(workerID, kv, pubsub)

	// Worker channel listener
	pswWg := sync.WaitGroup{}
	defer pswWg.Wait()
	pswWg.Add(2)
	workerCtx := context.WithValue(ctx, "workerID", workerID)
	go func() {
		defer pswWg.Done()
		err := psw.ListenWorkerChannel(workerCtx)
		if err != nil && err != workerCtx.Err() {
			t.Error(err)
		}
	}()

	// Job queue router
	go func() {
		defer pswWg.Done()
		err := psw.ListenJobQueue(ctx)
		if err != nil && err != workerCtx.Err() {
			t.Error(err)
		}
	}()

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
			case <-time.After(time.Second):
				t.Errorf("timeout waiting for result %s", jobID)
			}
		}(jobID, expected)
	}

	wg.Wait()
}
