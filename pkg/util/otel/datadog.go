package otel

import (
	"go.opentelemetry.io/otel"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func withTraceProviderDatadog(opts ...tracer.StartOption) func() {
	provider := ddotel.NewTracerProvider(opts...)
	otel.SetTracerProvider(provider)

	return func() { provider.Shutdown() }
}
