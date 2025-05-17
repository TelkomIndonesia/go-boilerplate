package log

import (
	"context"
)

type Logger interface {
	LoggerBase
	LoggerExt
}

type LoggerBase interface {
	Debug(ctx context.Context, message string, attrs ...Attr)
	Info(ctx context.Context, message string, attrs ...Attr)
	Warn(ctx context.Context, message string, attrs ...Attr)
	Error(ctx context.Context, message string, attrs ...Attr)
	Fatal(ctx context.Context, message string, attrs ...Attr)
}

type LoggerExt interface {
	WithAttrs(attrs ...Attr) Logger
	WithTrace() Logger
}
