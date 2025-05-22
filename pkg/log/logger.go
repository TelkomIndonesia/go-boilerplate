package log

import (
	"context"
	"log/slog"
	"strings"
)

type Logger interface {
	LoggerBase
	LoggerExt
}

type LoggerBase interface {
	Enabled(context.Context, Level) bool

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

type Level string

func (l Level) Level() slog.Level {
	switch strings.ToLower(string(l)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "fatal":
		return slog.LevelError + 4
	}
	return slog.LevelInfo
}
