package internal

import (
	"fmt"
	"time"
)

var _ Log = LogMap{}

type LogMap map[string]any

func (d LogMap) Any(key string, value any) {
	if s, ok := value.(fmt.Stringer); ok {
		d.String(key, s.String())
		return
	}
	if s, ok := value.(error); ok {
		d.String(key, s.Error())
		return
	}
	d[key] = value
}

func (d LogMap) Bool(key string, value bool) {
	d[key] = value
}

func (d LogMap) ByteString(key string, value []byte) {
	d[key] = string(value)
}

func (d LogMap) String(key string, value string) {
	d[key] = value
}

func (d LogMap) Float64(key string, value float64) {
	d[key] = value
}

func (d LogMap) Int64(key string, value int64) {
	d[key] = value
}

func (d LogMap) Uint64(key string, value uint64) {
	d[key] = value
}

func (d LogMap) Time(key string, value time.Time) {
	d[key] = value
}

func (d LogMap) Error(key string, value error) {
	d[key] = value.Error()
}
