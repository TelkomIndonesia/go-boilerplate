package logzap

import (
	"fmt"
	"strings"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.uber.org/zap"
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
	ctxFunc []log.LogFunc
}

func NewLogger(opts ...OptFunc) (l log.Logger, err error) {
	z, err := zap.NewProduction(zap.AddCallerSkip(2))
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
	return log.WithLoggerExt(zl), nil
}

func (l zaplogger) Debug(message string, fn ...log.LogFunc) {
	if l.lvl > LevelDebug {
		return
	}

	l.zap.Debug(message, newZapLog(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Info(message string, fn ...log.LogFunc) {
	if l.lvl > LevelInfo {
		return
	}

	l.zap.Info(message, newZapLog(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Warn(message string, fn ...log.LogFunc) {
	if l.lvl > LevelWarn {
		return
	}

	l.zap.Warn(message, newZapLog(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Error(message string, fn ...log.LogFunc) {
	if l.lvl > LevelError {
		return
	}

	l.zap.Error(message, newZapLog(append(fn, l.ctxFunc...)...).fields...)
}
func (l zaplogger) Fatal(message string, fn ...log.LogFunc) {
	if l.lvl > LevelFatal {
		return
	}

	l.zap.Fatal(message, newZapLog(append(fn, l.ctxFunc...)...).fields...)
}
