package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

func withConsoleExporter(ctx context.Context, opts ...stdouttrace.Option) func() {
	traceExporter, _ := stdouttrace.New(
		append(opts,
			stdouttrace.WithPrettyPrint(),
		)...)

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(traceProvider)

	return func() { traceProvider.Shutdown(ctx) }
}
