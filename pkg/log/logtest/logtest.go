package logtest

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type logger struct {
	t   *testing.T
	l   log.Logger
	b   *bytes.Buffer
	s   *slog.Logger
	mux sync.Mutex
}

func NewLogger(t *testing.T) log.Logger {
	b := &bytes.Buffer{}
	h := slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})
	s := slog.New(h)
	return &logger{t: t, l: log.NewLogger(log.WithHandlers(h)), b: b, s: s}
}

func (l *logger) Enabled(context.Context, log.Level) bool {
	return true
}

func (l *logger) Debug(ctx context.Context, message string, attrs ...log.Attr) {
	l.t.Helper()
	l.log(l.l.Debug, ctx, slog.LevelWarn, message, attrs...)
}

func (l *logger) Info(ctx context.Context, message string, attrs ...log.Attr) {
	l.t.Helper()
	l.log(l.l.Info, ctx, slog.LevelWarn, message, attrs...)
}

func (l *logger) Warn(ctx context.Context, message string, attrs ...log.Attr) {
	l.t.Helper()
	l.log(l.l.Warn, ctx, slog.LevelWarn, message, attrs...)
}

func (l *logger) Error(ctx context.Context, message string, attrs ...log.Attr) {
	l.t.Helper()
	l.log(l.l.Error, ctx, slog.LevelWarn, message, attrs...)
}

func (l *logger) Fatal(ctx context.Context, message string, attrs ...log.Attr) {
	l.t.Helper()
	l.log(l.l.Error, ctx, slog.LevelWarn, message, attrs...)
	l.t.FailNow()
}

func (l *logger) log(fn func(context.Context, string, ...log.Attr), ctx context.Context, level slog.Level, message string, attrs ...log.Attr) {
	l.t.Helper()

	l.mux.Lock()
	defer l.mux.Unlock()

	fn(ctx, message, attrs...)
	l.t.Log(l.b.String())
	l.b.Reset()
}

func (l *logger) WithAttrs(attrs ...log.Attr) log.Logger {
	l.t.Helper()
	l.l = l.l.WithAttrs(attrs...)
	return l
}

func (l *logger) WithTrace() log.Logger {
	l.t.Helper()
	l.l = l.l.WithTrace()
	return l
}
