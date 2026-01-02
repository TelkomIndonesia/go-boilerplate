package pubsubrouter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
)

type Job[T any] struct {
	ID string

	Result T
	ACK    func()
	NACK   func()
}

type KeyValueSvc interface {
	Register(ctx context.Context, key string, value string) error
	UnRegister(ctx context.Context, key string) error
	Lookup(ctx context.Context, key string) (value string, err error)
}

type PubSubSvc[T any] interface {
	JobQueue(ctx context.Context) (<-chan Job[T], error)
	WorkerChannel(ctx context.Context) (<-chan Job[T], error)
	PublishResult(ctx context.Context, workerID string, jobID string, result T) error
}

type PubSubRouter[T any] struct {
	workerID string

	kvRepo KeyValueSvc
	pubsub PubSubSvc[T]

	chanmap cmap.ConcurrentMap[string, chan T]

	stop chan struct{}
}

func NewPubSubRouter[T any](
	workerID string,
	kvRepo KeyValueSvc,
	pubsub PubSubSvc[T],
) *PubSubRouter[T] {
	return &PubSubRouter[T]{
		workerID: workerID,
		kvRepo:   kvRepo,
		pubsub:   pubsub,
		chanmap:  cmap.New[chan T](),
		stop:     make(chan struct{}),
	}
}

func (p *PubSubRouter[T]) Close() error {
	close(p.stop)
	return nil
}

func (p *PubSubRouter[T]) Listen(ctx context.Context) (err error) {
	pswWg := sync.WaitGroup{}
	defer pswWg.Wait()
	pswWg.Add(2)

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-ctx.Done():
		case <-p.stop:
			cancel()
		}
	}()

	go func() {
		defer pswWg.Done()
		err1 := p.ListenWorkerChannel(ctx)
		if err1 != nil && err1 != ctx.Err() {
			err = errors.Join(err, err1)
		}
	}()

	// Job queue router
	go func() {
		defer pswWg.Done()
		err2 := p.ListenJobQueue(ctx)
		if err2 != nil && err2 != ctx.Err() {
			err = errors.Join(err, err2)
		}
	}()

	return
}
func (p *PubSubRouter[T]) ListenJobQueue(ctx context.Context) error {
	chanJob, err := p.pubsub.JobQueue(ctx)
	if err != nil {
		return err
	}

	for {
		var job Job[T]

		select {
		case <-ctx.Done():
			return ctx.Err()

		case job = <-chanJob:
		}

		slog.Default().Info("receive job queue : ", "jobID", job.ID, "result", job.Result)

		workerID, err := p.kvRepo.Lookup(ctx, job.ID)
		if err != nil {
			slog.Default().WarnContext(ctx, "failed to lookup worker for job",
				"jobID", job.ID,
				"error", err,
			)
			job.NACK()
			continue
		}
		if workerID == "" {
			slog.Default().Info("no worker found ", "jobID", job.ID, "result", job.Result)
			job.ACK()
			continue
		}

		slog.Default().Info("matched job queue : ", "workerID", workerID, "jobID", job.ID, "result", job.Result)

		err = p.pubsub.PublishResult(ctx, workerID, job.ID, job.Result)
		if err != nil {
			job.NACK()
			return fmt.Errorf("publish worker channel failed: %w", err)
		}

		job.ACK()
	}
}

func (p *PubSubRouter[T]) ListenWorkerChannel(ctx context.Context) error {
	chanJob, err := p.pubsub.WorkerChannel(ctx)
	if err != nil {
		return err
	}

	i := 0
	for {
		var job Job[T]

		select {
		case <-ctx.Done():
			return ctx.Err()

		case job = <-chanJob:
		}

		i++
		slog.Default().Info("receive worker queue : ", "workerID", p.workerID, "jobID", job.ID, "result", job.Result, "i", i)

		resChan, ok := p.chanmap.Get(job.ID)
		if !ok {
			job.ACK()
			continue
		}

		select {
		case <-ctx.Done():
			slog.Default().Info("ctx done : ", "workerID", p.workerID, "jobID", job.ID, "result", job.Result, "i", i)
			job.NACK()
			return ctx.Err()

		case resChan <- job.Result:
			slog.Default().Info("sent worker queue : ", "workerID", p.workerID, "jobID", job.ID, "result", job.Result, "i", i)
			job.ACK()

		default:
			slog.Default().Info("buffer full : ", "workerID", p.workerID, "jobID", job.ID, "result", job.Result, "i", i)
			job.ACK()
		}
	}
}

func (p *PubSubRouter[T]) WaitResult(ctx context.Context, jobID string) (
	results <-chan T,
	done func(ctx context.Context) error,
	err error,
) {
	resChan := make(chan T)
	p.chanmap.Set(jobID, resChan)
	done = func(ctx context.Context) error {
		return p.doneWaiting(ctx, jobID, resChan)
	}

	if err := p.kvRepo.Register(ctx, jobID, p.workerID); err != nil {
		return nil, nil, err
	}

	return resChan, done, nil
}

func (p *PubSubRouter[T]) doneWaiting(ctx context.Context, jobID string, resChan chan T) (err error) {
	if err := p.kvRepo.UnRegister(ctx, jobID); err != nil {
		return err
	}

	// close(resChan)
	p.chanmap.Remove(jobID)
	return
}
