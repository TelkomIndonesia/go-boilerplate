package log

import (
	"context"
	"os"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/log/internal"
	"go.opentelemetry.io/otel/trace"
)

var globalLogger Logger = WithLoggerExt(deflogger{w: os.Stderr})

func Global() Logger {
	return globalLogger
}

func Register(l Logger) {
	globalLogger = l
}

func Any(key string, value any) LogFunc {
	if l, ok := value.(Loggable); ok {
		value = l.AsLog()
	}

	return func(l Log) {
		l.Any(key, value)
	}
}

func Bool(key string, value bool) LogFunc {
	return func(l Log) {
		l.Bool(key, value)
	}
}

func ByteString(key string, value []byte) LogFunc {
	return func(l Log) {
		l.ByteString(key, value)
	}
}

func String(key string, value string) LogFunc {
	return func(l Log) {
		l.String(key, value)
	}
}

func Float64(key string, value float64) LogFunc {
	return func(l Log) {
		l.Float64(key, value)
	}
}

func Int64(key string, value int64) LogFunc {
	return func(l Log) {
		l.Int64(key, value)
	}
}

func Uint64(key string, value uint64) LogFunc {
	return func(l Log) {
		l.Uint64(key, value)
	}
}

func Time(key string, value time.Time) LogFunc {
	return func(l Log) {
		l.Time(key, value)
	}
}

func Error(key string, value error) LogFunc {
	return func(l Log) {
		l.Error(key, value)
	}
}

func WithTrace(ctx context.Context, fns ...LogFunc) LogFunc {
	return func(l Log) {
		if ctx != nil {
			span := trace.SpanFromContext(ctx)
			if span.SpanContext().HasTraceID() {
				l.String("trace-id", span.SpanContext().TraceID().String())
			}
			l = internal.LogWTrace{
				Log:  l,
				Span: span,
			}
		}
		for _, fn := range fns {
			fn(l)
		}
	}
}
