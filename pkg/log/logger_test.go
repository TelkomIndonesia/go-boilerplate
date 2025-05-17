package log

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
)

// A token is a secret value that grants permissions.
type Token string

// LogValue implements slog.LogValuer.
// It avoids revealing the token.
func (Token) LogValue() slog.Value {
	return slog.StringValue("REDACTED_TOKEN")
}

func (Token) AsLog() any {
	return "REDACTED_TOKEN"
}

type Level slog.Level

const LevelDebug Level = Level(slog.LevelDebug)

func Print(l Level) {}

func TestSLog(t *testing.T) {
	exp, err := stdoutlog.New()
	require.NoError(t, err)
	provider := otellog.NewLoggerProvider(otellog.WithProcessor(otellog.NewSimpleProcessor(exp)))
	global.SetLoggerProvider(provider)
	logger := otelslog.NewLogger("my/pkg/name", otelslog.WithSource(true))
	err = errors.New("test")
	data := struct {
		A string
		B int
	}{
		A: "a",
		B: 1,
	}
	logger.LogAttrs(context.Background(), slog.LevelDebug, "message",
		slog.Any("hello", Token("world")),
		slog.Any("err", err),
		slog.Any("data", data))

	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}))
	logger.LogAttrs(context.Background(), slog.LevelDebug, "message",
		slog.Any("hello", Token("world")),
		slog.Any("err", err),
		slog.Any("data", data))

	Print(LevelDebug)

}

func TestLogger(t *testing.T) {
	s, _ := stdouttrace.New()
	p := trace.NewTracerProvider(
		trace.WithBatcher(s),
	)
	defer p.Shutdown(context.Background())

	otel.SetTracerProvider(p)
	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	handler1 := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	handler2 := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	l := NewLogger(WithHandlers(handler1, NewTraceableHandler(handler2)))
	l.WithTrace().Info(ctx, "test",
		String("hello", "world"),
		Any("token", Token("world")),
		Error("error", errors.New("test")))
}

func BenchmarkLogger(b *testing.B) {
	otel.SetTracerProvider(trace.NewTracerProvider())
	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})

	attrs := []Attr{}
	for range 50 {
		attrs = append(attrs, String("hello", "world"), Any("hello", Token("world")))
	}

	l0 := slog.New(handler)
	b.Run("unwrapped", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l0.LogAttrs(ctx, slog.LevelInfo, "test", asSlogAttrs(attrs)...)
		}
	})

	l1 := NewLogger(WithHandlers(NewTraceableHandler(handler)))
	b.Run("wrapped", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l1.Info(ctx, "test", attrs...)
		}
	})

	l2 := NewLogger(WithHandlers(NewTraceableHandler(handler), handler))
	b.Run("wrapped2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l2.Info(ctx, "test", attrs...)
		}
	})

	l3 := NewLogger(WithHandlers(NewTraceableHandler(handler), handler))
	b.Run("wrapped3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l3.WithTrace().Info(ctx, "test", attrs...)
		}
	})

}
