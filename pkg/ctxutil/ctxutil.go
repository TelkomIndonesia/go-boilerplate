package ctxutil

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

type contextkey struct{}

func WithMatcher(pctx context.Context) (ctx context.Context, matcher func(ctx context.Context) bool) {
	ctx = context.WithValue(pctx, contextkey{}, pctx)
	matcher = func(ctx context.Context) bool {
		if ctx == nil {
			return false
		}

		p, ok := ctx.Value(contextkey{}).(context.Context)
		if !ok {
			return false
		}
		return p == pctx
	}
	return
}

func WithExitSignal(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
		case <-done:
			cancel()
		}
	}()
	return ctx
}
