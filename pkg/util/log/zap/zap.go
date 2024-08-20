package zap

import (
	"fmt"
	"strings"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type OptFunc func(*zaplogger) error

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func WithLevelString(s string) OptFunc {
	var l Level = -1
	switch strings.ToLower(s) {
	case "debug":
		l = 0
	case "info":
		l = 1
	case "warn":
		l = 2
	case "error":
		l = 3
	case "fatal":
		l = 4
	}
	return WithLevel(l)
}

func WithLevel(l Level) OptFunc {
	return func(z *zaplogger) (err error) {
		if l < LevelDebug || l > LevelFatal {
			return fmt.Errorf("invalid level: %d", l)
		}

		z.lvl = l
		return
	}
}

type zaplogger struct {
	zap     *zap.Logger
	lvl     Level
	ctxFunc []log.LogContextFunc
}

func New(opts ...OptFunc) (l log.Logger, err error) {
	z, err := zap.NewProduction(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate zap")
	}

	zl := &zaplogger{zap: z, lvl: LevelInfo}
	for _, opt := range opts {
		err = opt(zl)
		if err != nil {
			return nil, fmt.Errorf("failed to apply options: %w", err)
		}
	}
	return zl, nil
}

func (l zaplogger) Debug(message string, fn ...log.LogContextFunc) {
	if l.lvl > LevelDebug {
		return
	}

	l.zap.Debug(message, newLoggerContext(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Info(message string, fn ...log.LogContextFunc) {
	if l.lvl > LevelInfo {
		return
	}

	l.zap.Info(message, newLoggerContext(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Warn(message string, fn ...log.LogContextFunc) {
	if l.lvl > LevelWarn {
		return
	}

	l.zap.Warn(message, newLoggerContext(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Error(message string, fn ...log.LogContextFunc) {
	if l.lvl > LevelError {
		return
	}

	l.zap.Error(message, newLoggerContext(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Fatal(message string, fn ...log.LogContextFunc) {
	if l.lvl > LevelFatal {
		return
	}

	l.zap.Fatal(message, newLoggerContext(append(fn, l.ctxFunc...)...).fields...)
}

func (l zaplogger) WithCtx(fn log.LogContextFunc) log.Logger {
	l.ctxFunc = append(l.ctxFunc, fn)
	return l
}

type loggerContext struct {
	fields []zap.Field
}

func newLoggerContext(fn ...log.LogContextFunc) *loggerContext {
	lc := &loggerContext{fields: make([]zapcore.Field, 0, len(fn))}
	for _, fn := range fn {
		fn(lc)
	}
	return lc
}

func (lc *loggerContext) Any(key string, value any) {
	lc.fields = append(lc.fields, zap.Any(key, value))

}
func (lc *loggerContext) Bool(key string, value bool) {
	lc.fields = append(lc.fields, zap.Bool(key, value))

}
func (lc *loggerContext) ByteString(key string, value []byte) {
	lc.fields = append(lc.fields, zap.ByteString(key, value))

}
func (lc *loggerContext) String(key string, value string) {
	lc.fields = append(lc.fields, zap.String(key, value))

}
func (lc *loggerContext) Float64(key string, value float64) {
	lc.fields = append(lc.fields, zap.Float64(key, value))

}
func (lc *loggerContext) Int64(key string, value int64) {
	lc.fields = append(lc.fields, zap.Int64(key, value))

}
func (lc *loggerContext) Uint64(key string, value uint64) {
	lc.fields = append(lc.fields, zap.Uint64(key, value))

}
func (lc *loggerContext) Time(key string, value time.Time) {
	lc.fields = append(lc.fields, zap.Time(key, value))
}
func (lc *loggerContext) Error(key string, value error) {
	lc.fields = append(lc.fields, zap.NamedError(key, value))
}
