package pubsubrouter

import (
	"context"
	"fmt"
	"sync"
)

type Message[T any] struct {
	Content T

	ChannelID string
	ACK       func()
	NACK      func(reason NACKReason)
}

type NACKReason struct {
	Code    NACKReasonCode
	Message string
}

type NACKReasonCode int

const (
	NACKReasonKVError NACKReasonCode = iota
	NACKReasonPubSubWorkerQueueError
	NACKReasonNoSubscriber
	NACKReasonSlowSubscriber
	NACKReasonStoppedSubscriber
	NACKReasonContextCancelled
)

type Channel[T any] struct {
	ch        chan Message[T]
	terminate func()

	chWriteStop chan struct{}
	stopWrite   func()

	close func(context.Context) error
}

func newChannel[T any](buflen int, beforeClose func(context.Context, Channel[T]) error) Channel[T] {

	var once1, once2 sync.Once

	channel := Channel[T]{
		ch:          make(chan Message[T], buflen),
		chWriteStop: make(chan struct{}),
	}

	channel.terminate = func() {
		once1.Do(func() { close(channel.ch) })
		channel.ch = nil
	}
	channel.stopWrite = func() {
		once2.Do(func() { close(channel.chWriteStop) })
	}
	channel.close = func(ctx context.Context) (err error) {
		if err := beforeClose(ctx, channel); err != nil {
			return err
		}

		channel.stopWrite()
		return
	}

	return channel
}

func (c Channel[T]) equal(other Channel[T]) bool {
	return c.ch == other.ch
}

func (c Channel[T]) initiated() bool {
	return c.ch != nil
}

func (c Channel[T]) write(ctx context.Context, msg Message[T]) (err error) {
	if !c.initiated() {
		msg.NACK(NACKReason{
			Code:    NACKReasonNoSubscriber,
			Message: "no subscriber",
		})
		return fmt.Errorf("no subscriber")
	}

	select {
	case <-ctx.Done():
		msg.NACK(NACKReason{
			Code:    NACKReasonContextCancelled,
			Message: ctx.Err().Error(),
		})
		return ctx.Err()

	default:
		msg.NACK(NACKReason{
			Code:    NACKReasonSlowSubscriber,
			Message: "slow subscriber",
		})
		return fmt.Errorf("slow subscriber")

	case <-c.chWriteStop:
		msg.NACK(NACKReason{
			Code:    NACKReasonStoppedSubscriber,
			Message: "channel has been closed",
		})
		c.terminate()
		return fmt.Errorf("channel has been closed")

	case c.ch <- msg:
	}

	return
}

func (c Channel[T]) Messages() <-chan Message[T] {
	return c.ch
}

func (c Channel[T]) Close(ctx context.Context) (err error) {
	if c.close != nil {
		return nil
	}

	return c.close(ctx)
}
