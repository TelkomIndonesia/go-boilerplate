package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/telkomindonesia/go-boilerplate/pkg/log/internal"
)

var _ LoggerBase = deflogger{}

type OptFunc func(*deflogger) error

func WithWritter(w io.Writer) OptFunc {
	return func(d *deflogger) (err error) {
		d.w = w
		return
	}
}

type deflogger struct {
	w io.Writer

	ctxFunc []LogFunc
}

func New(opts ...OptFunc) (l Logger, err error) {
	dl := &deflogger{w: os.Stderr}
	for _, opt := range opts {
		err = opt(dl)
		if err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}
	return WithLoggerExt(dl), nil
}

type defMessage struct {
	Level   string          `json:"level"`
	Message string          `json:"message"`
	Fields  internal.LogMap `json:"fields"`
}

func (d deflogger) println(level string, message string, fn ...LogFunc) {
	ctx := internal.LogMap{}
	for _, fn := range append(d.ctxFunc, fn...) {
		fn(ctx)
	}
	json.NewEncoder(d.w).Encode(defMessage{
		Level:   level,
		Message: message,
		Fields:  ctx,
	})
}

func (d deflogger) Debug(message string, fn ...LogFunc) {
	d.println("DEBUG", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Info(message string, fn ...LogFunc) {
	d.println("INFO", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Warn(message string, fn ...LogFunc) {
	d.println("WARN", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Error(message string, fn ...LogFunc) {
	d.println("ERROR", message, append(fn, d.ctxFunc...)...)
}
func (d deflogger) Fatal(message string, fn ...LogFunc) {
	d.println("FATAL", message, append(fn, d.ctxFunc...)...)
	os.Exit(1)
}
