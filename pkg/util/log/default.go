package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

var _ Logger = deflogger{}

type OptFunc func(*deflogger) error

func WithWritter(w io.Writer) OptFunc {
	return func(d *deflogger) (err error) {
		d.w = w
		return
	}
}

type deflogger struct {
	w io.Writer

	ctxFunc []LogContextFunc
}

func New(opts ...OptFunc) (l Logger, err error) {
	dl := &deflogger{w: os.Stderr}
	for _, opt := range opts {
		err = opt(dl)
		if err != nil {
			return nil, fmt.Errorf("fail to apply option: %w", err)
		}
	}
	return dl, nil
}

type defMessage struct {
	Level   string     `json:"level"`
	Message string     `json:"message"`
	Fields  defContext `json:"fields"`
}

func (d deflogger) println(level string, message string, fn ...LogContextFunc) {
	ctx := defContext{}
	for _, fn := range fn {
		fn(ctx)
	}
	json.NewEncoder(d.w).Encode(defMessage{
		Level:   level,
		Message: message,
		Fields:  ctx,
	})
}

func (d deflogger) Debug(message string, fn ...LogContextFunc) {
	d.println("DEBUG", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Info(message string, fn ...LogContextFunc) {
	d.println("INFO", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Warn(message string, fn ...LogContextFunc) {
	d.println("WARN", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Error(message string, fn ...LogContextFunc) {
	d.println("ERROR", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Fatal(message string, fn ...LogContextFunc) {
	d.println("FATAL", message, append(fn, d.ctxFunc...)...)
	os.Exit(1)
}
func (d deflogger) WithCtx(f LogContextFunc) Logger {
	d.ctxFunc = append(d.ctxFunc, f)
	return d
}

var _ LogContext = defContext{}

type defContext map[string]any

func (d defContext) Any(key string, value any) {
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

func (d defContext) Bool(key string, value bool) {
	d[key] = value
}

func (d defContext) ByteString(key string, value []byte) {
	d[key] = value
}

func (d defContext) String(key string, value string) {
	d[key] = value
}

func (d defContext) Float64(key string, value float64) {
	d[key] = value
}

func (d defContext) Int64(key string, value int64) {
	d[key] = value
}

func (d defContext) Uint64(key string, value uint64) {
	d[key] = value
}

func (d defContext) Time(key string, value time.Time) {
	d[key] = value
}

func (d defContext) Duration(key string, value time.Duration) {
	d[key] = value
}
