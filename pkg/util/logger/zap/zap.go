package zap

import (
	"fmt"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type OptFunc func(*zaplogger) error

type zaplogger struct {
	zap *zap.Logger
}

func New(opts ...OptFunc) (l logger.Logger, err error) {
	z, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("fail to instantiate zap")
	}

	zl := &zaplogger{zap: z}
	for _, opt := range opts {
		err = opt(zl)
		if err != nil {
			return nil, fmt.Errorf("fail to apply options: %w", err)
		}
	}
	return zl, nil
}

func (l zaplogger) Debug(message string, fn ...logger.LoggerContextFunc) {
	l.zap.Debug(message, newLoggerContext(fn...).fields...)
}
func (l zaplogger) Info(message string, fn ...logger.LoggerContextFunc) {
	l.zap.Info(message, newLoggerContext(fn...).fields...)
}
func (l zaplogger) Warn(message string, fn ...logger.LoggerContextFunc) {
	l.zap.Warn(message, newLoggerContext(fn...).fields...)
}
func (l zaplogger) Error(message string, fn ...logger.LoggerContextFunc) {
	l.zap.Error(message, newLoggerContext(fn...).fields...)
}
func (l zaplogger) Fatal(message string, fn ...logger.LoggerContextFunc) {
	l.zap.Fatal(message, newLoggerContext(fn...).fields...)
}

type loggerContext struct {
	fields []zap.Field
}

func newLoggerContext(fn ...logger.LoggerContextFunc) *loggerContext {
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
func (lc *loggerContext) Duration(key string, value time.Duration) {
	lc.fields = append(lc.fields, zap.Duration(key, value))
}
