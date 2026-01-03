package pubsubrouter

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type ResultChan[T any] struct {
	ch   chan T
	done <-chan struct{}

	close       func()
	signalClose func()
	Close       func(context.Context) error
}

func newResultChan[T any](buflen int, beforeClose func(context.Context) error) ResultChan[T] {
	ch := make(chan T, buflen)
	done := make(chan struct{})
	var once1, once2 sync.Once

	finalClose := func() { once1.Do(func() { close(ch) }) }
	signalClose := func() { once2.Do(func() { close(done) }) }
	return ResultChan[T]{
		ch:   ch,
		done: done,

		close:       finalClose,
		signalClose: signalClose,

		Close: func(ctx context.Context) error {
			if err := beforeClose(ctx); err != nil {
				return err
			}
			signalClose()
			return nil
		},
	}
}

func (c ResultChan[T]) Chan() <-chan T {
	return c.ch
}

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

	chanmap cmap.ConcurrentMap[string, ResultChan[T]]

	logger log.Logger
}

func NewPubSubRouter[T any](
	workerID string,
	kvRepo KeyValueSvc,
	pubsub PubSubSvc[T],
	logger log.Logger,
) *PubSubRouter[T] {
	return &PubSubRouter[T]{
		workerID: workerID,
		kvRepo:   kvRepo,
		pubsub:   pubsub,
		chanmap:  cmap.New[ResultChan[T]](),
		logger:   logger,
	}
}

func (p *PubSubRouter[T]) Listen(ctx context.Context) (err error) {
	errs := make(chan error, 2)
	wg := sync.WaitGroup{}
	defer wg.Wait()
	wg.Add(2)

	go func() {
		defer wg.Done()
		errs <- p.ListenWorkerChannel(ctx)
	}()

	go func() {
		defer wg.Done()
		errs <- p.ListenJobQueue(ctx)
	}()

	for range 2 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case errx := <-errs:
			err = errors.Join(err, errx)
		}
	}

	return
}
func (p *PubSubRouter[T]) ListenJobQueue(ctx context.Context) error {
	chanJob, err := p.pubsub.JobQueue(ctx)
	if err != nil {
		return err
	}

	for {
		var job Job[T]
		var ok bool

		select {
		case <-ctx.Done():
			return ctx.Err()

		case job, ok = <-chanJob:
		}

		if !ok {
			p.logger.Debug(ctx, "job queue closed", log.String("worker-id", p.workerID))
			return nil
		}

		p.logger.Debug(ctx, "receive job queue",
			log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))

		workerID, err := p.kvRepo.Lookup(ctx, job.ID)
		if err != nil {
			p.logger.Warn(ctx, "failed to lookup worker for job",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.NACK()
			continue
		}
		if workerID == "" {
			p.logger.Warn(ctx, "no worker found",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.ACK()
			continue
		}

		err = p.pubsub.PublishResult(ctx, workerID, job.ID, job.Result)
		if err != nil {
			p.logger.Warn(ctx, "failed to publish result",
				log.String("worker-id", workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.NACK()
			continue
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
		var ok bool

		select {
		case <-ctx.Done():
			return ctx.Err()

		case job, ok = <-chanJob:
		}

		if !ok {
			return nil
		}

		p.logger.Debug(ctx, "receive worker queue",
			log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))

		resChan, ok := p.chanmap.Get(job.ID)
		if !ok {
			p.logger.Warn(ctx, "no handler",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.ACK()
			continue
		}

		select {
		case <-ctx.Done():
			p.logger.Debug(ctx, "ctx done",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.NACK()
			return ctx.Err()

		case resChan.ch <- job.Result:
			p.logger.Debug(ctx, "sent worker queue",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.ACK()

		case <-resChan.done:
			p.logger.Debug(ctx, "closed worker queue",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			resChan.close()
			job.ACK()

		default:
			p.logger.Warn(ctx, "unresponsive handler",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.ACK()
		}
	}
}

func (p *PubSubRouter[T]) HandleResults(ctx context.Context, jobID string, buflen int) (results ResultChan[T], err error) {
	rc := newResultChan[T](buflen, func(ctx context.Context) error {
		return p.doneRouting(ctx, jobID)
	})

	updated := false
	p.chanmap.Upsert(jobID, rc, func(exist bool, old, new ResultChan[T]) ResultChan[T] {
		if exist {
			old.signalClose()
			updated = true
		}
		return new
	})
	if updated {
		return rc, nil
	}

	err = p.kvRepo.Register(ctx, jobID, p.workerID)
	if err != nil {
		p.chanmap.Remove(jobID)
		err = fmt.Errorf("failed to register to key value service")
		return
	}

	return rc, nil
}

func (p *PubSubRouter[T]) doneRouting(ctx context.Context, jobID string) (err error) {
	if err := p.kvRepo.UnRegister(ctx, jobID); err != nil {
		return fmt.Errorf("failed to unregister to key value service")
	}

	p.chanmap.Remove(jobID)
	return
}
