package otelloader

import (
	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

var _ logr.LogSink = logsink{l: nil}

type logsink struct {
	l      log.Logger
	name   string
	fields []any
}

func (o logsink) Enabled(level int) bool {
	return true
}

func (o logsink) Error(err error, msg string, keysAndValues ...any) {
	o.l.Error(msg, log.Error("error", err), log.String("name", o.name), log.String("name", o.name), log.Any("fields", append(o.fields, keysAndValues...)))
}

func (o logsink) Info(level int, msg string, keysAndValues ...any) {
	o.l.Info(msg, log.String("name", o.name), log.Any("fields", append(o.fields, keysAndValues...)))
}

func (o logsink) Init(info logr.RuntimeInfo) {}

func (o logsink) WithName(name string) logr.LogSink {
	return logsink{
		l:      o.l,
		fields: o.fields,
		name:   name,
	}
}

func (o logsink) WithValues(keysAndValues ...any) logr.LogSink {
	return logsink{
		l:      o.l,
		fields: append(o.fields, keysAndValues...),
		name:   o.name,
	}
}
