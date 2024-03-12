package otel

import (
	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
)

var _ logr.LogSink = otellogger{l: nil}

type otellogger struct {
	l      logger.Logger
	name   string
	fields []any
}

func (o otellogger) Enabled(level int) bool {
	return true
}

func (o otellogger) Error(err error, msg string, keysAndValues ...any) {
	o.l.Error(msg, logger.Any("error", err), logger.String("name", o.name), logger.Any("additional", append(o.fields, keysAndValues...)))
}

func (o otellogger) Info(level int, msg string, keysAndValues ...any) {
	o.l.Info(msg, logger.String("name", o.name), logger.Any("additional", append(o.fields, keysAndValues...)))
}

func (o otellogger) Init(info logr.RuntimeInfo) {}

func (o otellogger) WithName(name string) logr.LogSink {
	return otellogger{
		l:      o.l,
		fields: o.fields,
		name:   name,
	}
}

func (o otellogger) WithValues(keysAndValues ...any) logr.LogSink {
	return otellogger{
		l:      o.l,
		fields: append(o.fields, keysAndValues...),
		name:   o.name,
	}
}
