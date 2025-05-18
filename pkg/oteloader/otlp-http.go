package oteloader

import (
	"context"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
)

func withTraceOTLPHTTPExporter(ctx context.Context, l log.Logger, opts ...otlptracehttp.Option) func() {
	traceExporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		l.Error(ctx, "failed to create trace exporter", log.Error("error", err))
		return func() {}
	}

	traceProvider := trace.NewTracerProvider(trace.WithBatcher(traceExporter))
	otel.SetTracerProvider(traceProvider)

	return func() { traceProvider.Shutdown(ctx) }
}

func withLogOTLPHTTPExporter(ctx context.Context, l log.Logger, opts ...otlploghttp.Option) func() {
	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		l.Error(ctx, "failed to create log exporter", log.Error("error", err))
		return func() {}
	}

	provider := otellog.NewLoggerProvider(otellog.WithProcessor(otellog.NewSimpleProcessor(exporter)))
	global.SetLoggerProvider(provider)

	return func() { provider.Shutdown(ctx) }
}
