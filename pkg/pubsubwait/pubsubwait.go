package pubsubwait

import (
	"context"
	"fmt"
	"log/slog"
)

type Job[T any] struct {
	ID string

	Result T
	ACK    func()
	NACK   func()
}

type KeyValueRepo interface {
	Register(ctx context.Context, key string, value string) error
	UnRegister(ctx context.Context, key string) error
	Lookup(ctx context.Context, key string) (value string, err error)
}

type PubSubSvc[T any] interface {
	SubscribeJobQueue(ctx context.Context) (<-chan Job[T], error)
	SubscribeWorkerChannel(ctx context.Context) (<-chan Job[T], error)
	PublishWorkerChannel(ctx context.Context, workerID string, jobID string, result T) error
}

type PubSubWait[T any] struct {
	workerID string

	kvRepo     KeyValueRepo
	pubsub     PubSubSvc[T]
	routineMap map[string]chan<- T
}

func (p *PubSubWait[T]) ListenJobQueue(ctx context.Context) (err error) {
	chanJob, err := p.pubsub.SubscribeJobQueue(ctx)
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
			slog.Default().WarnContext(ctx, "error looking for the worker", "error", err)
			job.NACK()
			continue
		}

		err = p.pubsub.PublishWorkerChannel(ctx, workerID, job.ID, job.Result)
		if err != nil {
			return fmt.Errorf("failed to publish worker channel: %w", err)
		}
		job.ACK()
	}
}

func (p *PubSubWait[T]) ListenWorkerChannel(ctx context.Context) (err error) {
	chanJob, err := p.pubsub.SubscribeWorkerChannel(ctx)
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
		resChan, ok := p.routineMap[job.ID]
		if !ok {
			job.ACK()
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case resChan <- job.Result:
			job.ACK()
		}
	}

}

func (p *PubSubWait[T]) WaitResult(ctx context.Context, jobID string) (results <-chan T, done func(ctx context.Context), err error) {
	resChan := make(chan T)
	p.routineMap[jobID] = resChan
	defer func() {
		delete(p.routineMap, jobID)
		close(resChan)
	}()

	if err := p.kvRepo.Register(ctx, jobID, p.workerID); err != nil {
		return nil, func(ctx context.Context) {}, err
	}

	done = func(ctx context.Context) { _ = p.doneWaiting(ctx, jobID) }
	return
}

func (p *PubSubWait[T]) doneWaiting(ctx context.Context, jobID string) (err error) {
	resChan, ok := p.routineMap[jobID]
	if ok {
		delete(p.routineMap, jobID)
		close(resChan)
	}

	if err := p.kvRepo.UnRegister(ctx, jobID); err != nil {
		return err
	}

	return
}
