package otel

import (
	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
)

var _ logr.LogSink = logsink{l: nil}

type logsink struct {
	l      logger.Logger
	name   string
	fields []any
}

func (o logsink) Enabled(level int) bool {
	return true
}

func (o logsink) Error(err error, msg string, keysAndValues ...any) {
	o.l.Error(msg, logger.Any("error", err), logger.String("name", o.name), logger.String("name", o.name), logger.Any("fields", append(o.fields, keysAndValues...)))
}

func (o logsink) Info(level int, msg string, keysAndValues ...any) {
	o.l.Info(msg, logger.String("name", o.name), logger.Any("fields", append(o.fields, keysAndValues...)))
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
