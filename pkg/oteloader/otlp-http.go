package oteloader

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/trace"
)

func withTraceOTLPHTTPExporter(ctx context.Context, opts ...otlptracehttp.Option) func() {
	traceExporter, _ := otlptracehttp.New(ctx, opts...)

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(traceProvider)

	return func() { traceProvider.Shutdown(ctx) }
}
