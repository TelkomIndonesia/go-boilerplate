package oteloader

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
)

func withTraceConsoleExporter(ctx context.Context, l log.Logger, opts ...stdouttrace.Option) func() {
	traceExporter, err := stdouttrace.New(opts...)
	if err != nil {
		l.Error(ctx, "failed to create trace exporter", log.Error("error", err))
		return func() {}
	}

	traceProvider := trace.NewTracerProvider(trace.WithBatcher(traceExporter))
	otel.SetTracerProvider(traceProvider)

	return func() { traceProvider.Shutdown(ctx) }
}

func withLogConsoleExporter(ctx context.Context, l log.Logger, opts ...stdoutlog.Option) func() {
	exporter, err := stdoutlog.New(opts...)
	if err != nil {
		l.Error(ctx, "failed to create log exporter", log.Error("error", err))
		return func() {}
	}

	provider := otellog.NewLoggerProvider(otellog.WithProcessor(otellog.NewSimpleProcessor(exporter)))
	global.SetLoggerProvider(provider)

	return func() { provider.Shutdown(ctx) }
}
