package logger

import "time"

type Logger interface {
	Debug(message string, fn ...LoggerContextFunc)
	Info(message string, fn ...LoggerContextFunc)
	Warn(message string, fn ...LoggerContextFunc)
	Error(message string, fn ...LoggerContextFunc)
	Fatal(message string, fn ...LoggerContextFunc)
}

type LoggerContextFunc func(LoggerContext)

type LoggerContext interface {
	Any(key string, value any)
	Bool(key string, value bool)
	ByteString(key string, value []byte)
	String(key string, value string)
	Float64(key string, value float64)
	Int64(key string, value int64)
	Uint64(key string, value uint64)
	Time(key string, value time.Time)
	Duration(key string, value time.Duration)
}

type Loggable interface {
	AsLog() any
}
