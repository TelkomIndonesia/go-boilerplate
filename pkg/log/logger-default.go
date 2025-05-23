package log

import (
	"context"
	"log/slog"
	"os"

	"github.com/telkomindonesia/go-boilerplate/pkg/log/internal"
)

func NewTraceableHandler(h slog.Handler) slog.Handler {
	return internal.NewTraceableHandler(h)
}

type DefaultLoggerOpts func(*logger)

func WithHandlers(h ...slog.Handler) DefaultLoggerOpts {
	return func(l *logger) {
		l.h = h
		return
	}
}

type logger struct {
	h []slog.Handler
	l *slog.Logger
}

func NewLogger(opts ...DefaultLoggerOpts) Logger {
	l := logger{}
	for _, opt := range opts {
		opt(&l)
	}

	var handler slog.Handler
	switch len(l.h) {
	case 0:
		handler = internal.DefaultHandler
	case 1:
		handler = l.h[0]
	default:
		handler = l.h[0]
		for _, h := range l.h[1:] {
			handler = internal.NewBiHandler(handler, h)
		}
	}
	l.l = slog.New(handler)

	return WithLoggerExt(l)
}

func (l logger) Enabled(ctx context.Context, lvl Level) bool {
	return l.l.Enabled(ctx, lvl.Level())
}

func (l logger) Debug(ctx context.Context, message string, attrs ...Attr) {
	l.log(ctx, slog.LevelDebug, message, attrs...)
}

func (l logger) Info(ctx context.Context, message string, attrs ...Attr) {
	l.log(ctx, slog.LevelInfo, message, attrs...)
}

func (l logger) Warn(ctx context.Context, message string, attrs ...Attr) {
	l.log(ctx, slog.LevelWarn, message, attrs...)
}

func (l logger) Error(ctx context.Context, message string, attrs ...Attr) {
	l.log(ctx, slog.LevelError, message, attrs...)
}

func (l logger) Fatal(ctx context.Context, message string, attrs ...Attr) {
	l.l.LogAttrs(ctx, slog.LevelError+4, message, asSlogAttrs(attrs)...)
	os.Exit(1)
}

func (l logger) log(ctx context.Context, level slog.Level, message string, attrs ...Attr) {
	if !l.l.Enabled(ctx, level) {
		return
	}
	l.l.LogAttrs(ctx, level, message, asSlogAttrs(attrs)...)
}
