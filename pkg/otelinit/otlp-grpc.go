package otelinit

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"go.opentelemetry.io/otel/sdk/trace"
)

func withTraceOTLPGRPCExporter(ctx context.Context, opts ...otlptracegrpc.Option) func() {
	traceExporter, _ := otlptracegrpc.New(ctx, opts...)

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(traceProvider)

	return func() { traceProvider.Shutdown(ctx) }
}
