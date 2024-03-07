package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

func withTraceConsoleExporter(ctx context.Context, opts ...stdouttrace.Option) func() {
	traceExporter, _ := stdouttrace.New(
		append(opts,
			stdouttrace.WithPrettyPrint(),
		)...)

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(traceProvider)

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	return func() { traceProvider.Shutdown(ctx) }
}
