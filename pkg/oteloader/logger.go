package oteloader

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
)

var _ logr.LogSink = logger{l: nil}
var _ ddtrace.Logger = logger{l: nil}

type logger struct {
	l      log.Logger
	name   string
	fields []any
}

func (o logger) Log(msg string) {
	if o.l == nil {
		return
	}
	o.l.Info(context.Background(), msg, log.String("name", o.name))
}

func (o logger) Enabled(level int) bool {
	return true
}

func (o logger) Error(err error, msg string, keysAndValues ...any) {
	if o.l == nil {
		return
	}
	o.l.Error(context.Background(), msg, log.Error("error", err), log.String("name", o.name), log.String("name", o.name), log.Any("fields", append(o.fields, keysAndValues...)))
}

func (o logger) Info(level int, msg string, keysAndValues ...any) {
	if o.l == nil {
		return
	}
	o.l.Info(context.Background(), msg, log.String("name", o.name), log.Any("fields", append(o.fields, keysAndValues...)))
}

func (o logger) Init(info logr.RuntimeInfo) {}

func (o logger) WithName(name string) logr.LogSink {
	return logger{
		l:      o.l,
		fields: o.fields,
		name:   name,
	}
}

func (o logger) WithValues(keysAndValues ...any) logr.LogSink {
	return logger{
		l:      o.l,
		fields: append(o.fields, keysAndValues...),
		name:   o.name,
	}
}
