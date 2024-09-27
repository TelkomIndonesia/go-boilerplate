package internal

import "time"

type Log interface {
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
