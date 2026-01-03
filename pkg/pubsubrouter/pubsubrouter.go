package pubsubrouter

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type Message[T any] struct {
	Content T

	ChannelID string
	ACK       func()
	NACK      func()
}

type Channel[T any] struct {
	ch   chan Message[T]
	done <-chan struct{}

	close       func()
	signalClose func()
	Close       func(context.Context) error
}

func newChannel[T any](buflen int, beforeClose func(context.Context) error) Channel[T] {
	ch := make(chan Message[T], buflen)
	done := make(chan struct{})
	var once1, once2 sync.Once

	finalClose := func() { once1.Do(func() { close(ch) }) }
	signalClose := func() { once2.Do(func() { close(done) }) }
	return Channel[T]{
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

func (c Channel[T]) Messages() <-chan Message[T] {
	return c.ch
}

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

func New[T any](
	workerID string,
	kvRepo KeyValueSvc,
	pubsub PubSubSvc[T],
	logger log.Logger,
) *PubSubRouter[T] {
	return &PubSubRouter[T]{
		workerID: workerID,
		kvRepo:   kvRepo,
		pubsub:   pubsub,
		chanmap:  cmap.New[Channel[T]](),
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
		errs <- p.ListenMessageQueue(ctx)
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
			p.logger.Warn(ctx, "failed to lookup worker for message",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK()
			continue
		}
		if workerID == "" {
			p.logger.Warn(ctx, "no worker found",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.ACK()
			continue
		}

		err = p.pubsub.PublishWorkerMessage(ctx, workerID, msg.ChannelID, msg.Content)
		if err != nil {
			p.logger.Warn(ctx, "failed to publish message",
				log.String("worker-id", workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK()
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
			p.logger.Warn(ctx, "no receiver",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.ACK()
			continue
		}

		select {
		case <-ctx.Done():
			msg.NACK()
			return ctx.Err()

		case channel.ch <- msg:
			p.logger.Debug(ctx, "sent worker queue",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))

		case <-channel.done:
			p.logger.Debug(ctx, "closed worker queue",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			channel.close()
			msg.ACK()

		default:
			p.logger.Warn(ctx, "unresponsive subscriber",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.ACK()
		}
	}
}

func (p *PubSubRouter[T]) Subscribe(ctx context.Context, channelID string, buflen int) (channel Channel[T], err error) {
	rc := newChannel[T](buflen, func(ctx context.Context) error {
		return p.doneRouting(ctx, channelID)
	})

	updated := false
	p.chanmap.Upsert(channelID, rc, func(exist bool, old, new Channel[T]) Channel[T] {
		if exist {
			old.signalClose()
			updated = true
		}
		return new
	})
	if updated {
		return rc, nil
	}

	err = p.kvRepo.Set(ctx, channelID, p.workerID)
	if err != nil {
		p.chanmap.Remove(channelID)
		err = fmt.Errorf("failed to register to key value service")
		return
	}

	return rc, nil
}

func (p *PubSubRouter[T]) doneRouting(ctx context.Context, channelID string) (err error) {
	if err := p.kvRepo.Remove(ctx, channelID); err != nil {
		return fmt.Errorf("failed to unregister to key value service")
	}

	p.chanmap.Remove(channelID)
	return
}
