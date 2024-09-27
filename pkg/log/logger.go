package log

import (
	"context"
)

type Logger interface {
	LoggerBase
	LoggerExt
}

type LoggerBase interface {
	Debug(message string, fn ...LogFunc)
	Info(message string, fn ...LogFunc)
	Warn(message string, fn ...LogFunc)
	Error(message string, fn ...LogFunc)
	Fatal(message string, fn ...LogFunc)
}

type LoggerExt interface {
	WithLog(...LogFunc) Logger
	WithTrace(ctx context.Context) Logger
}
