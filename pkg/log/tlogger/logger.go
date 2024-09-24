package tlogger

import (
	"encoding/json"
	"testing"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/internal"
)

var _ log.Logger = &logger{}

type logger struct {
	t       *testing.T
	ctxFunc []log.LogContextFunc
}

func (l *logger) println(level string, message string, fn ...log.LogContextFunc) {
	l.t.Helper()

	ctx := internal.MapContext{}
	for _, fn := range append(l.ctxFunc, fn...) {
		fn(ctx)
	}

	m, err := json.Marshal(map[string]interface{}{
		"level":   level,
		"message": message,
		"fields":  ctx,
	})
	if err != nil {
		l.t.Error(err)
	}

	l.t.Log(string(m))
}

// Debug implements log.Logger.
func (l *logger) Debug(message string, fn ...log.LogContextFunc) {
	l.t.Helper()
	l.println("DEBUG", message, fn...)
}

// Error implements log.Logger.
func (l *logger) Error(message string, fn ...log.LogContextFunc) {
	l.t.Helper()
	l.println("ERROR", message, fn...)
}

// Fatal implements log.Logger.
func (l *logger) Fatal(message string, fn ...log.LogContextFunc) {
	l.t.Helper()
	l.println("FATAL", message, fn...)
}

// Info implements log.Logger.
func (l *logger) Info(message string, fn ...log.LogContextFunc) {
	l.t.Helper()
	l.println("INFO", message, fn...)
}

// Warn implements log.Logger.
func (l *logger) Warn(message string, fn ...log.LogContextFunc) {
	l.t.Helper()
	l.println("WARN", message, fn...)
}

// WithCtx implements log.Logger.
func (l *logger) WithCtx(c log.LogContextFunc) log.Logger {
	return &logger{ctxFunc: append(l.ctxFunc, c)}
}

func New(t *testing.T) log.Logger {
	return &logger{t: t}
}
