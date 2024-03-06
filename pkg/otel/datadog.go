package otel

import (
	"go.opentelemetry.io/otel"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
)

func WithProviderDatadog() (deferer func()) {
	provider := ddotel.NewTracerProvider()
	deferer = func() { provider.Shutdown() }

	otel.SetTracerProvider(provider)
	return
}
