package internal

import (
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ Log = LogWTrace{}

type LogWTrace struct {
	Log
	Span trace.Span
}

func (d LogWTrace) Any(key string, value any) {
	switch v := value.(type) {
	case bool:
		d.Span.SetAttributes(attribute.Bool(key, v))
	case []bool:
		d.Span.SetAttributes(attribute.BoolSlice(key, v))
	case int:
		d.Span.SetAttributes(attribute.Int(key, v))
	case []int:
		d.Span.SetAttributes(attribute.IntSlice(key, v))
	case int64:
		d.Span.SetAttributes(attribute.Int64(key, v))
	case []int64:
		d.Span.SetAttributes(attribute.Int64Slice(key, v))
	case float64:
		d.Span.SetAttributes(attribute.Float64(key, v))
	case []float64:
		d.Span.SetAttributes(attribute.Float64Slice(key, v))
	case string:
		d.Span.SetAttributes(attribute.String(key, v))
	case []string:
		d.Span.SetAttributes(attribute.StringSlice(key, v))
	case fmt.Stringer:
		d.Span.SetAttributes(attribute.Stringer(key, v))
	default:
		b, err := json.Marshal(v)
		if err == nil {
			d.Span.SetAttributes(attribute.String(key, string(b)))
		}
	}
	d.Log.Any(key, value)
}

func (d LogWTrace) Bool(key string, value bool) {
	d.Span.SetAttributes(attribute.Bool(key, value))
	d.Log.Bool(key, value)
}

func (d LogWTrace) ByteString(key string, value []byte) {
	d.Span.SetAttributes(attribute.String(key, string(value)))
	d.Log.ByteString(key, value)
}

func (d LogWTrace) String(key string, value string) {
	d.Span.SetAttributes(attribute.String(key, value))
	d.Log.String(key, value)
}

func (d LogWTrace) Float64(key string, value float64) {
	d.Span.SetAttributes(attribute.Float64(key, value))
	d.Log.Float64(key, value)
}

func (d LogWTrace) Int64(key string, value int64) {
	d.Span.SetAttributes(attribute.Int64(key, value))
	d.Log.Int64(key, value)
}

func (d LogWTrace) Uint64(key string, value uint64) {
	d.Span.SetAttributes(attribute.Int64(key, int64(value)))
	d.Log.Uint64(key, value)
}

func (d LogWTrace) Time(key string, value time.Time) {
	d.Span.SetAttributes(attribute.String(key, value.String()))
	d.Log.Time(key, value)
}

func (d LogWTrace) Error(key string, value error) {
	if value == nil {
		return
	}
	d.Span.RecordError(value)
	d.Span.SetStatus(codes.Error, value.Error())
	d.Log.Error(key, value)
}
