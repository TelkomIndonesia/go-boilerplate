package log

import "time"

type Logger interface {
	Debug(message string, fn ...LogContextFunc)
	Info(message string, fn ...LogContextFunc)
	Warn(message string, fn ...LogContextFunc)
	Error(message string, fn ...LogContextFunc)
	Fatal(message string, fn ...LogContextFunc)

	WithCtx(LogContextFunc) Logger
}

type LogContextFunc func(LogContext)

type LogContext interface {
	Any(key string, value any)
	Bool(key string, value bool)
	ByteString(key string, value []byte)
	String(key string, value string)
	Float64(key string, value float64)
	Int64(key string, value int64)
	Uint64(key string, value uint64)
	Time(key string, value time.Time)
	Error(key string, value error)
}

type Loggable interface {
	AsLog() any
}
