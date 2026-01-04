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
	initiated bool

	ch         chan Message[T]
	finalClose func()

	chDone      <-chan struct{}
	signalClose func()

	close func(context.Context) error
}

func newChannel[T any](buflen int, beforeClose func(context.Context, Channel[T]) error) Channel[T] {
	ch := make(chan Message[T], buflen)
	chDone := make(chan struct{})

	var once1, once2 sync.Once
	finalClose := func() { once1.Do(func() { close(ch) }) }
	signalClose := func() { once2.Do(func() { close(chDone) }) }

	channel := Channel[T]{
		initiated: true,

		ch:     ch,
		chDone: chDone,

		finalClose:  finalClose,
		signalClose: signalClose,
	}
	Close := func(ctx context.Context) error {
		if err := beforeClose(ctx, channel); err != nil {
			return err
		}

		signalClose()
		return nil
	}
	channel.close = Close

	return channel
}

func (c Channel[T]) write(ctx context.Context, msg Message[T]) (err error) {
	if !c.initiated {
		return errors.New("channel not initiated")
	}

	select {
	default:
		log.FromContext(ctx).Warn(ctx, "NACK due to unresponsive subscriber",
			log.String("channel-id", msg.ChannelID))
		msg.NACK()

	case <-ctx.Done():
		log.FromContext(ctx).Warn(ctx, "NACK due to context cancellation",
			log.String("channel-id", msg.ChannelID))
		msg.NACK()
		return ctx.Err()

	case <-c.chDone:
		log.FromContext(ctx).Warn(ctx, "NACK due to channel closed",
			log.String("channel-id", msg.ChannelID))
		c.finalClose()
		msg.NACK()

	case c.ch <- msg:
		log.FromContext(ctx).Debug(ctx, "sent worker queue",
			log.String("channel-id", msg.ChannelID))
	}

	return
}

func (c Channel[T]) Messages() <-chan Message[T] {
	return c.ch
}

func (c Channel[T]) Close(ctx context.Context) (err error) {
	if !c.initiated {
		return errors.New("channel not initiated")
	}

	return c.close(ctx)
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
			p.logger.Warn(ctx, "NACK due to failure to lookup worker for message",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.NACK()
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
		if !ok || !channel.initiated {
			p.logger.Warn(ctx, "dropping data due to no receiver",
				log.String("worker-id", p.workerID), log.String("channel-id", msg.ChannelID))
			msg.ACK()
			continue
		}

		ctx := log.ContextWithLog(ctx, p.logger.WithAttrs(log.String("worker-id", p.workerID)))
		if err := channel.write(ctx, msg); err != nil {
			return err
		}
	}
}

func (p *PubSubRouter[T]) Subscribe(ctx context.Context, channelID string, buflen int) (channel Channel[T], err error) {
	channel = newChannel(buflen, func(ctx context.Context, rc Channel[T]) error {
		return p.doneRouting(ctx, channelID, rc)
	})

	p.chanmap.Upsert(channelID, channel, func(exist bool, old, new Channel[T]) Channel[T] {
		if exist && old.initiated {
			old.signalClose()
			return new
		}

		err = p.kvRepo.Set(ctx, channelID, p.workerID)
		if err != nil {
			channel = Channel[T]{}
			err = fmt.Errorf("failed to register to key value service")
			return channel
		}

		return new
	})

	return
}

func (p *PubSubRouter[T]) doneRouting(ctx context.Context, channelID string, channel Channel[T]) (err error) {
	p.chanmap.RemoveCb(channelID, func(key string, v Channel[T], exists bool) bool {
		if !exists || !v.initiated {
			return true
		}

		if v.ch != channel.ch {
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
