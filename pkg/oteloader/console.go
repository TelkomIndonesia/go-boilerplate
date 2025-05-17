package oteloader

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

func withTraceConsoleExporter(ctx context.Context, opts ...stdouttrace.Option) func() {
	traceExporter, _ := stdouttrace.New(opts...)

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(traceProvider)

	return func() { traceProvider.Shutdown(ctx) }
}
