package log

import (
	"context"
	"log/slog"
)

type contextKey struct{}

func ContextWithLog(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

func FromContext(ctx context.Context) Logger {
	v, ok := ctx.Value(contextKey{}).(Logger)
	if !ok {
		return NewLogger(WithHandlers(slog.DiscardHandler))
	}
	return v
}
