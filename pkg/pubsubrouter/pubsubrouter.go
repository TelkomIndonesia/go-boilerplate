package pubsubrouter

import (
	"context"
	"errors"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type chanWCloseSignal[T any] struct {
	ch   chan T
	done chan struct{}
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

	chanmap cmap.ConcurrentMap[string, chanWCloseSignal[T]]
	stop    chan struct{}

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
		chanmap:  cmap.New[chanWCloseSignal[T]](),
		stop:     make(chan struct{}),
		logger:   logger,
	}
}

func (p *PubSubRouter[T]) Close() error {
	close(p.stop)
	return nil
}

func (p *PubSubRouter[T]) Listen(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-p.stop:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	errs := make(chan error, 2)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case errx := <-errs:
				err = errors.Join(err, errx)
			}
		}
	}()

	pswWg := sync.WaitGroup{}
	defer pswWg.Wait()
	pswWg.Add(2)

	go func() {
		defer pswWg.Done()
		err1 := p.ListenWorkerChannel(ctx)
		if err1 != nil && err1 != ctx.Err() {
			errs <- err1
		}
	}()

	go func() {
		defer pswWg.Done()
		err2 := p.ListenJobQueue(ctx)
		if err2 != nil && err2 != ctx.Err() {
			errs <- err2
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
			p.logger.Debug(ctx, "worker channel closed", log.String("worker-id", p.workerID))
			return nil
		}

		p.logger.Debug(ctx, "receive worker queue",
			log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))

		resChan, ok := p.chanmap.Get(job.ID)
		if !ok {
			p.logger.Debug(context.Background(), "no waiter",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.ACK()
			continue
		}

		select {
		case <-ctx.Done():
			p.logger.Debug(context.Background(), "ctx done",
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
			close(resChan.ch)
			job.ACK()

		default:
			p.logger.Warn(ctx, "no waiter",
				log.String("worker-id", p.workerID), log.String("job-id", job.ID), log.Any("result", job.Result))
			job.ACK()
		}
	}
}

func (p *PubSubRouter[T]) WaitResult(ctx context.Context, jobID string) (
	results <-chan T,
	done func(ctx context.Context) error,
	err error,
) {
	resChan := chanWCloseSignal[T]{
		ch:   make(chan T),
		done: make(chan struct{}),
	}

	p.chanmap.Set(jobID, resChan)
	done = func(ctx context.Context) error {
		return p.doneWaiting(ctx, jobID, resChan)
	}

	if err := p.kvRepo.Register(ctx, jobID, p.workerID); err != nil {
		return nil, nil, err
	}

	return resChan.ch, done, nil
}

func (p *PubSubRouter[T]) doneWaiting(ctx context.Context, jobID string, resChan chanWCloseSignal[T]) (err error) {
	if err := p.kvRepo.UnRegister(ctx, jobID); err != nil {
		return err
	}

	p.logger.Debug(ctx, "done waiting", log.String("worker-id", p.workerID), log.String("job-id", jobID))
	p.chanmap.Remove(jobID)
	close(resChan.done)
	return
}
