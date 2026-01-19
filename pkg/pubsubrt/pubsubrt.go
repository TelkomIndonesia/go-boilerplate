package pubsubrt

import (
	"context"
	"errors"
	"fmt"
	"iter"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type KeyValueSvc interface {
	SAdd(ctx context.Context, key string, value string) error
	SGet(ctx context.Context, key string) (value []string, err error)
	SRem(ctx context.Context, key string, value string) error
}

type PubSubSvc[T any] interface {
	MessageQueue(ctx context.Context) (iter.Seq2[Message[T], error], error)
	PublishWorkerMessage(ctx context.Context, workerID string, channelID string, content T) error
	WorkerChannel(ctx context.Context) (iter.Seq2[Message[T], error], error)
}

type PubSubRouter[T any] struct {
	workerID string

	kvRepo KeyValueSvc
	pubsub PubSubSvc[T]

	chanmap cmap.ConcurrentMap[string, *[]Channel[T]]

	logger log.Logger
}

type OptFunc[T any] func(*PubSubRouter[T]) error

func WithLogger[T any](logger log.Logger) OptFunc[T] {
	return func(p *PubSubRouter[T]) error {
		p.logger = logger
		return nil
	}
}

func New[T any](
	workerID string,
	kvRepo func() KeyValueSvc,
	pubsub func(workerID string) PubSubSvc[T],
	opts ...OptFunc[T],
) (*PubSubRouter[T], error) {
	p := &PubSubRouter[T]{
		workerID: workerID,
		kvRepo:   kvRepo(),
		pubsub:   pubsub(workerID),
		chanmap:  cmap.New[*[]Channel[T]](),
		logger:   log.Global(),
	}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("failed to apply options: %w", err)
		}
	}

	return p, nil
}

func (p *PubSubRouter[T]) Listen(ctx context.Context) (err error) {
	errs := make(chan error, 2)
	go func() { errs <- p.ListenWorkerChannel(ctx) }()
	go func() { errs <- p.ListenMessageQueue(ctx) }()

	for range len(errs) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case errx := <-errs:
			err = errors.Join(err, errx)
		}
	}

	return
}
func (p *PubSubRouter[T]) ListenMessageQueue(ctx context.Context) error {
	chanMessage, err := p.pubsub.MessageQueue(ctx)
	if err != nil {
		return err
	}

	for msg, err := range chanMessage {
		if err != nil {
			p.logger.Warn(ctx, "message queue errored", log.String("worker-id", p.workerID), log.Error("error", err))
			continue
		}

		p.logger.Debug(ctx, "receive message queue",
			log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))

		workers, err := p.kvRepo.SGet(ctx, msg.ChannelID)
		if err != nil {
			p.logger.Warn(ctx, "NACK due to failure to lookup worker for message",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK(NACKReason{
				Code:    NACKReasonKVError,
				Message: err.Error(),
			})
			continue
		}
		if len(workers) == 0 {
			p.logger.Warn(ctx, "dropping data due to inexistent worker",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.ACK()
			continue
		}

		failed := map[string]error{}
		for _, worker := range workers {
			errx := p.pubsub.PublishWorkerMessage(ctx, worker, msg.ChannelID, msg.Content)
			if errx != nil {
				err = errors.Join(err, errx)
				failed[worker] = errx
				continue
			}
		}
		if len(failed) == len(workers) {
			p.logger.Warn(ctx, "NACK due to failure to publish message to all workers",
				log.Any("worker-id", workers), log.String("channel-id", msg.ChannelID))
			msg.NACK(NACKReason{
				Code:    NACKReasonPubSubWorkerQueueError,
				Message: err.Error(),
			})
			continue
		}
		if len(failed) > 0 {
			p.logger.Warn(ctx, "failed to publish message to some workers",
				log.Any("workers", failed), log.String("channel-id", msg.ChannelID))
		}

		msg.ACK()
	}

	return nil
}

func (p *PubSubRouter[T]) ListenWorkerChannel(ctx context.Context) error {
	chanJob, err := p.pubsub.WorkerChannel(ctx)
	if err != nil {
		return err
	}

	for msg, err := range chanJob {
		if err != nil {
			p.logger.Warn(ctx, "worker queue errored",
				log.String("worker-id", p.workerID),
				log.Error("error", err))
			continue
		}

		p.logger.Debug(ctx, "receive worker queue",
			log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))

		var channels []Channel[T]
		// can't use get safely here because we are storing pointer
		// and it might be modified when removing from chanmap
		p.chanmap.RemoveCb(msg.ChannelID, func(key string, v *[]Channel[T], exists bool) bool {
			if exists {
				channels = *v
			}
			return false
		})
		if len(channels) == 0 {
			p.logger.Warn(ctx, "dropping data due to no receiver",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK(NACKReason{
				Code:    NACKReasonNoSubscriber,
				Message: "no receiver",
			})
			continue
		}

		for _, channel := range channels {
			if err := channel.write(ctx, msg); err != nil {
				p.logger.Warn(ctx, "channel write failed",
					log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID), log.Error("error", err))
				continue
			}

			p.logger.Debug(ctx, "channel successfully written",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
		}

	}

	return nil
}

func (p *PubSubRouter[T]) Subscribe(ctx context.Context, channelID string, buflen int) (channel Channel[T], err error) {
	channel = newChannel(channelID, buflen, p.doneRouting)

	p.chanmap.Upsert(channelID, nil, func(exist bool, old, new *[]Channel[T]) *[]Channel[T] {
		if exist && old != nil && len(*old) > 0 {
			a := append(make([]Channel[T], 0, len(*old)+1), *old...)
			a = append(a, channel)
			return &a
		}

		err = p.kvRepo.SAdd(ctx, channelID, p.workerID)
		if err != nil {
			err = fmt.Errorf("failed to register to key value service")
			return old
		}

		return &[]Channel[T]{channel}
	})

	return
}

func (p *PubSubRouter[T]) doneRouting(ctx context.Context, channel Channel[T]) (err error) {
	p.chanmap.RemoveCb(channel.id, func(key string, existing *[]Channel[T], exists bool) bool {
		if !exists || existing == nil || len(*existing) == 0 {
			return true
		}

		for i, v := range *existing {
			if v.equal(channel) {
				a := append(make([]Channel[T], 0, len(*existing)-1), (*existing)[:i]...)
				*existing = append(a, (*existing)[i+1:]...)
			}
		}
		if len(*existing) != 0 {
			return false
		}

		err = p.kvRepo.SRem(ctx, channel.id, p.workerID)
		if err != nil {
			err = fmt.Errorf("failed to unregister to key value service")
			return false
		}

		return true

	})

	return
}
