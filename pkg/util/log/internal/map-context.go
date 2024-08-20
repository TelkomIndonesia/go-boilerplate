package internal

import (
	"fmt"
	"time"
)

type MapContext map[string]any

func (d MapContext) Any(key string, value any) {
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

func (d MapContext) Bool(key string, value bool) {
	d[key] = value
}

func (d MapContext) ByteString(key string, value []byte) {
	d[key] = value
}

func (d MapContext) String(key string, value string) {
	d[key] = value
}

func (d MapContext) Float64(key string, value float64) {
	d[key] = value
}

func (d MapContext) Int64(key string, value int64) {
	d[key] = value
}

func (d MapContext) Uint64(key string, value uint64) {
	d[key] = value
}

func (d MapContext) Time(key string, value time.Time) {
	d[key] = value
}

func (d MapContext) Duration(key string, value time.Duration) {
	d[key] = value
}

func (d MapContext) Error(key string, value error) {
	d[key] = value.Error()
}
