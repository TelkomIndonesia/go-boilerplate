package logger

import (
	"os"
	"time"
)

var globalLogger Logger = deflogger{w: os.Stderr}

func Global() Logger {
	return globalLogger
}

func Register(l Logger) {
	globalLogger = l
}

func Any(key string, value any) LoggerContextFunc {
	if l, ok := value.(Loggable); ok {
		value = l.AsLog()
	}

	return func(lc LoggerContext) {
		lc.Any(key, value)
	}
}
func Bool(key string, value bool) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.Bool(key, value)
	}
}
func ByteString(key string, value []byte) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.ByteString(key, value)
	}
}
func String(key string, value string) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.String(key, value)
	}
}
func Float64(key string, value float64) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.Float64(key, value)
	}
}
func Int64(key string, value int64) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.Int64(key, value)
	}
}
func Uint64(key string, value uint64) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.Uint64(key, value)
	}
}
func Time(key string, value time.Time) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.Time(key, value)
	}
}
func Duration(key string, value time.Duration) LoggerContextFunc {
	return func(lc LoggerContext) {
		lc.Duration(key, value)
	}
}
