package pubsubrouter

import (
	"context"
	"errors"
	"fmt"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type KeyValueSvc interface {
	Set(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (value string, err error)
	Remove(ctx context.Context, key string) error
}

type PubSubSvc[T any] interface {
	MessageQueue(ctx context.Context) (<-chan Message[T], error)
	PublishWorkerMessage(ctx context.Context, workerID string, channelID string, content T) error
	WorkerChannel(ctx context.Context) (<-chan Message[T], error)
}

type PubSubRouter[T any] struct {
	workerID string

	kvRepo KeyValueSvc
	pubsub PubSubSvc[T]

	chanmap cmap.ConcurrentMap[string, Channel[T]]

	logger log.Logger
}

type PubSubRouterOptFunc[T any] func(*PubSubRouter[T]) error

func WithLogger[T any](logger log.Logger) PubSubRouterOptFunc[T] {
	return func(p *PubSubRouter[T]) error {
		p.logger = logger
		return nil
	}
}

func New[T any](
	workerID string,
	kvRepo func() KeyValueSvc,
	pubsub func(workerID string) PubSubSvc[T],
	opts ...PubSubRouterOptFunc[T],
) (*PubSubRouter[T], error) {
	p := &PubSubRouter[T]{
		workerID: workerID,
		kvRepo:   kvRepo(),
		pubsub:   pubsub(workerID),
		chanmap:  cmap.New[Channel[T]](),
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

	for {
		var msg Message[T]
		var ok bool

		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok = <-chanMessage:
		}

		if !ok {
			p.logger.Debug(ctx, "message queue closed", log.String("worker-id", p.workerID))
			return nil
		}

		p.logger.Debug(ctx, "receive message queue",
			log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))

		workerID, err := p.kvRepo.Get(ctx, msg.ChannelID)
		if err != nil {
			p.logger.Warn(ctx, "NACK due to failure to lookup worker for message",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK(NACKReason{
				Code:    NACKReasonKVError,
				Message: err.Error(),
			})
			continue
		}
		if workerID == "" {
			p.logger.Warn(ctx, "dropping data due to inexistent worker",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.ACK()
			continue
		}

		err = p.pubsub.PublishWorkerMessage(ctx, workerID, msg.ChannelID, msg.Content)
		if err != nil {
			p.logger.Warn(ctx, "NACK due to failure to publish message",
				log.String("worker-id", workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK(NACKReason{
				Code:    NACKReasonPubSubWorkerQueueError,
				Message: err.Error(),
			})
			continue
		}

		msg.ACK()
	}
}

func (p *PubSubRouter[T]) ListenWorkerChannel(ctx context.Context) error {
	chanJob, err := p.pubsub.WorkerChannel(ctx)
	if err != nil {
		return err
	}

	for {
		var msg Message[T]
		var ok bool

		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg, ok = <-chanJob:
		}

		if !ok {
			return nil
		}

		p.logger.Debug(ctx, "receive worker queue",
			log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))

		channel, ok := p.chanmap.Get(msg.ChannelID)
		if !ok {
			p.logger.Warn(ctx, "dropping data due to no receiver",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK(NACKReason{
				Code:    NACKReasonNoSubscriber,
				Message: "no receiver",
			})
			continue
		}

		if err := channel.write(ctx, msg); err != nil {
			p.logger.Warn(ctx, "channel write failed",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID), log.Error("error", err))
			continue
		}

		p.logger.Debug(ctx, "channel successfully written",
			log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
	}
}

func (p *PubSubRouter[T]) Subscribe(ctx context.Context, channelID string, buflen int) (channel Channel[T], err error) {
	channel = newChannel(buflen, func(ctx context.Context, rc Channel[T]) error {
		return p.doneRouting(ctx, channelID, rc)
	})

	p.chanmap.Upsert(channelID, channel, func(exist bool, old, new Channel[T]) Channel[T] {
		if exist && old.initiated() {
			old.stopWrite()
			return new
		}

		err = p.kvRepo.Set(ctx, channelID, p.workerID)
		if err != nil {
			err = fmt.Errorf("failed to register to key value service")
			return channel
		}

		return new
	})

	return
}

func (p *PubSubRouter[T]) doneRouting(ctx context.Context, channelID string, channel Channel[T]) (err error) {
	p.chanmap.RemoveCb(channelID, func(key string, v Channel[T], exists bool) bool {
		if !exists || !v.initiated() {
			return true
		}

		if !v.equal(channel) {
			return false
		}

		err = p.kvRepo.Remove(ctx, channelID)
		if err != nil {
			err = fmt.Errorf("failed to unregister to key value service")
			return false
		}

		return true
	})
	return
}
