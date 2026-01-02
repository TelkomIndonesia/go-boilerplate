package pubsubrouter

import (
	"context"
	"fmt"
	"log/slog"

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
}

func NewPubSubWait[T any](
	workerID string,
	kvRepo KeyValueSvc,
	pubsub PubSubSvc[T],
) *PubSubRouter[T] {
	return &PubSubRouter[T]{
		workerID: workerID,
		kvRepo:   kvRepo,
		pubsub:   pubsub,
		chanmap:  cmap.New[chan T](),
	}
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

		workerID, err := p.kvRepo.Lookup(ctx, job.ID)
		if err != nil {
			slog.WarnContext(ctx, "failed to lookup worker for job",
				"jobID", job.ID,
				"error", err,
			)
			job.NACK()
			continue
		}

		if workerID == "" {
			job.ACK()
			continue
		}

		err = p.pubsub.PublishResult(ctx, workerID, job.ID, job.Result)
		if err != nil {
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

	for {
		var job Job[T]

		select {
		case <-ctx.Done():
			return ctx.Err()

		case job = <-chanJob:
		}

		resChan, ok := p.chanmap.Get(job.ID)
		if !ok {
			job.ACK()
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case resChan <- job.Result:
			job.ACK()

		default:
			job.NACK()
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

	close(resChan)
	p.chanmap.Remove(jobID)
	return
}
