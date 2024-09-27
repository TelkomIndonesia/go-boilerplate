package ltesting

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/internal"
)

type logger struct {
	t       *testing.T
	ctx     context.Context
	logFunc []log.LogFunc
}

func NewLogger(t *testing.T) log.Logger {
	return logger{t: t}
}

func (l logger) Debug(message string, fn ...log.LogFunc) {
	l.t.Helper()
	l.println("DEBUG", message, fn...)
}

func (l logger) Error(message string, fn ...log.LogFunc) {
	l.t.Helper()
	l.println("ERROR", message, fn...)
}

func (l logger) Fatal(message string, fn ...log.LogFunc) {
	l.t.Helper()
	l.println("FATAL", message, fn...)
}

func (l logger) Info(message string, fn ...log.LogFunc) {
	l.t.Helper()
	l.println("INFO", message, fn...)
}

func (l logger) Warn(message string, fn ...log.LogFunc) {
	l.t.Helper()
	l.println("WARN", message, fn...)
}

func (l logger) WithLog(c ...log.LogFunc) log.Logger {
	l.logFunc = append(l.logFunc, c...)
	return l
}

func (l logger) WithTrace(ctx context.Context) log.Logger {
	return l
}

func (l logger) println(level string, message string, fn ...log.LogFunc) {
	l.t.Helper()

	mlog := internal.LogMap{}
	log.WithTrace(l.ctx, append(fn, l.logFunc...)...)(mlog)

	m, err := json.Marshal(map[string]interface{}{
		"level":   level,
		"message": message,
		"fields":  mlog,
	})
	if err != nil {
		l.t.Error(err)
	}

	l.t.Log(string(m))
}
