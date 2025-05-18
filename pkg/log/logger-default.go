package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type DefaultLoggerOpts func(*logger)

func WithHandlers(h ...slog.Handler) DefaultLoggerOpts {
	return func(l *logger) {
		if len(h) == 1 {
			l.l = slog.New(h[0])
			return
		}
		l.l = slog.New(mhandler{handlers: h})
	}
}

type logger struct {
	l *slog.Logger
}

func NewLogger(opts ...DefaultLoggerOpts) Logger {
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	l := logger{
		l: slog.New(NewTraceableHandler(h)),
	}
	for _, opt := range opts {
		opt(&l)
	}
	return WithLoggerExt(l)
}

func (l logger) Debug(ctx context.Context, message string, attrs ...Attr) {
	if !l.l.Enabled(ctx, slog.LevelDebug) {
		return
	}
	l.log(ctx, slog.LevelDebug, message, attrs...)
}

func (l logger) Info(ctx context.Context, message string, attrs ...Attr) {
	if !l.l.Enabled(ctx, slog.LevelInfo) {
		return
	}

	l.log(ctx, slog.LevelInfo, message, attrs...)
}

func (l logger) Warn(ctx context.Context, message string, attrs ...Attr) {
	if !l.l.Enabled(ctx, slog.LevelWarn) {
		return
	}
	l.log(ctx, slog.LevelWarn, message, attrs...)
}

func (l logger) Error(ctx context.Context, message string, attrs ...Attr) {
	if !l.l.Enabled(ctx, slog.LevelError) {
		return
	}
	l.log(ctx, slog.LevelError, message, attrs...)
}

func (l logger) Fatal(ctx context.Context, message string, attrs ...Attr) {
	l.log(ctx, slog.LevelError, message, attrs...)
	os.Exit(1)
}

func (l logger) log(ctx context.Context, level slog.Level, message string, attrs ...Attr) {
	if !l.l.Enabled(ctx, level) {
		return
	}
	l.l.LogAttrs(ctx, level, message, asSlogAttrs(attrs)...)
}

type handler struct {
	slog.Handler
}

func NewTraceableHandler(h slog.Handler) slog.Handler {
	return handler{Handler: h}
}

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().TraceID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

var _ slog.Handler = mhandler{}

type mhandler struct {
	handlers []slog.Handler
}

func (m mhandler) Enabled(ctx context.Context, l slog.Level) (e bool) {
	for _, h := range m.handlers {
		e = e || h.Enabled(ctx, l)
	}
	return
}

func (m mhandler) Handle(ctx context.Context, r slog.Record) error {
	for i, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil {
			return fmt.Errorf("failed to handle record with handler %d: %w", i, err)
		}
	}
	return nil
}

func (m mhandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	for i := range m.handlers {
		m.handlers[i] = m.handlers[i].WithAttrs(attrs)
	}
	return m
}

func (m mhandler) WithGroup(name string) slog.Handler {
	for i := range m.handlers {
		m.handlers[i] = m.handlers[i].WithGroup(name)
	}
	return m
}

var globalLogger = NewLogger()

func Global() Logger {
	return globalLogger
}

func SetGlobal(l Logger) {
	globalLogger = l
	if l, ok := l.(*loggerExt); ok {
		if l, ok := l.l.(*logger); ok {
			slog.SetDefault(l.l)
		}
	}
}
