package util

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func CancelOnExitSignal(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		<-done
		cancel()
	}()
	return ctx
}
