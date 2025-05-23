package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

var DefaultHandler slog.Handler = NewTraceableHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

var _ slog.Handler = biHandler{}

type biHandler struct {
	a, b slog.Handler
}

func NewBiHandler(a, b slog.Handler) slog.Handler {
	return biHandler{a: a, b: b}
}

func (m biHandler) Enabled(ctx context.Context, l slog.Level) (e bool) {
	return m.a.Enabled(ctx, l) || m.b.Enabled(ctx, l)
}

func (m biHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := m.a.Handle(ctx, r); err != nil {
		return fmt.Errorf("failed to handle record with first handler: %w", err)
	}
	if err := m.b.Handle(ctx, r); err != nil {
		return fmt.Errorf("failed to handle record with second handler: %w", err)
	}

	return nil
}

func (m biHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	m.a = m.a.WithAttrs(attrs)
	m.b = m.b.WithAttrs(attrs)
	return m
}

func (m biHandler) WithGroup(name string) slog.Handler {
	m.a = m.a.WithGroup(name)
	m.b = m.b.WithGroup(name)
	return m
}

type traceableHandler struct {
	slog.Handler
}

func NewTraceableHandler(h slog.Handler) slog.Handler {
	return traceableHandler{Handler: h}
}

func (h traceableHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().TraceID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}
