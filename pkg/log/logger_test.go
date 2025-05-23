package log

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

type token string

func (token) LogValue() slog.Value {
	return slog.StringValue("REDACTED_TOKEN")
}

func (token) AsLog() any {
	return "REDACTED_TOKEN"
}

func TestLogger(t *testing.T) {
	otel.SetTracerProvider(trace.NewTracerProvider())
	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	t.Run("Default", func(t *testing.T) {
		l := NewLogger()
		l.WithTrace().Info(ctx, "test",
			String("hello", "world"),
			Any("token", token("world")),
			Error("error", errors.New("test")))
	})

	t.Run("MultipleAndTraceable", func(t *testing.T) {
		handler1 := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler2 := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler3 := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		l := NewLogger(WithHandlers(handler1, handler2, NewTraceableHandler(handler3)))
		l.WithTrace().Info(ctx, "test",
			String("hello", "world"),
			Any("token", token("world")),
			Error("error", errors.New("test")))
	})

}

func BenchmarkLogger(b *testing.B) {
	otel.SetTracerProvider(trace.NewTracerProvider())
	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})

	attrs := []Attr{}
	for range 50 {
		attrs = append(attrs, String("hello", "world"), Any("hello", token("world")))
	}

	l0 := slog.New(handler)
	b.Run("Raw", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l0.LogAttrs(ctx, slog.LevelInfo, "test", asSlogAttrs(attrs)...)
		}
	})

	l1 := NewLogger(WithHandlers(NewTraceableHandler(handler)))
	b.Run("Wrapped", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l1.Info(ctx, "test", attrs...)
		}
	})

	l2 := NewLogger(WithHandlers(NewTraceableHandler(handler), handler))
	b.Run("WrappedDouble", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l2.Info(ctx, "test", attrs...)
		}
	})

	l3 := NewLogger(WithHandlers(NewTraceableHandler(handler), handler))
	b.Run("WrappedDoubleWithTrace", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l3.WithTrace().Info(ctx, "test", attrs...)
		}
	})
}
