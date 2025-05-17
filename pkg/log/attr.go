package log

import (
	"log/slog"
	"runtime"
	"time"
)

type Valuer interface {
	AsLog() any
}

type Attr struct {
	attr      slog.Attr
	withTrace bool
	err       error
}

func (attr Attr) Attr() slog.Attr {
	return attr.attr
}

func asSlogAttrs(attrs []Attr) []slog.Attr {
	slogAttrs := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		slogAttrs = append(slogAttrs, attr.attr)
	}
	return slogAttrs
}

func Any(key string, value any) Attr {
	if l, ok := value.(Valuer); ok {
		value = l.AsLog()
	}

	return Attr{attr: slog.Any(key, value)}
}

func Bool(key string, value bool) Attr {
	return Attr{attr: slog.Bool(key, value)}
}

func String(key string, value string) Attr {
	return Attr{attr: slog.String(key, value)}
}

func Float64(key string, value float64) Attr {
	return Attr{attr: slog.Float64(key, value)}
}

func Int(key string, value int) Attr {
	return Attr{attr: slog.Int(key, value)}
}

func Int64(key string, value int64) Attr {
	return Attr{attr: slog.Int64(key, value)}
}

func Uint(key string, value uint) Attr {
	return Attr{attr: slog.Int64(key, int64(value))}
}

func Uint64(key string, value uint64) Attr {
	return Attr{attr: slog.Int64(key, int64(value))}
}

func Time(key string, value time.Time) Attr {
	return Attr{attr: slog.Time(key, value)}
}

func Duration(key string, value time.Duration) Attr {
	return Attr{attr: slog.Duration(key, value)}
}

func Error(key string, value error) Attr {
	buf := make([]byte, 2048)
	n := runtime.Stack(buf, false)
	return Attr{attr: slog.String(key, value.Error()+"\n"+string(buf[:n])), err: value}
}

func WithTrace(attrs ...Attr) []Attr {
	for i := range attrs {
		attrs[i].withTrace = true
	}
	return attrs
}
