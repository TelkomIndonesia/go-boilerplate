package log

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"
)

var globalLogger Logger = deflogger{w: os.Stderr}

func Global() Logger {
	return globalLogger
}

func Register(l Logger) {
	globalLogger = l
}

func Any(key string, value any) LogContextFunc {
	if l, ok := value.(Loggable); ok {
		value = l.AsLog()
	}

	return func(lc LogContext) {
		lc.Any(key, value)
	}
}

func Bool(key string, value bool) LogContextFunc {
	return func(lc LogContext) {
		lc.Bool(key, value)
	}
}

func ByteString(key string, value []byte) LogContextFunc {
	return func(lc LogContext) {
		lc.ByteString(key, value)
	}
}

func String(key string, value string) LogContextFunc {
	return func(lc LogContext) {
		lc.String(key, value)
	}
}

func Float64(key string, value float64) LogContextFunc {
	return func(lc LogContext) {
		lc.Float64(key, value)
	}
}

func Int64(key string, value int64) LogContextFunc {
	return func(lc LogContext) {
		lc.Int64(key, value)
	}
}

func Uint64(key string, value uint64) LogContextFunc {
	return func(lc LogContext) {
		lc.Uint64(key, value)
	}
}

func Time(key string, value time.Time) LogContextFunc {
	return func(lc LogContext) {
		lc.Time(key, value)
	}
}

func Error(key string, value error) LogContextFunc {
	return func(lc LogContext) {
		lc.Error(key, value)
	}
}

func TraceContext(key string, ctx context.Context) LogContextFunc {
	return func(lc LogContext) {
		spanCtx := trace.SpanContextFromContext(ctx)
		if !spanCtx.HasTraceID() {
			return
		}
		lc.String(key, spanCtx.TraceID().String())
		return
	}
}
