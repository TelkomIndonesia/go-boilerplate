package otelinit

import (
	"os"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/version"
	"go.opentelemetry.io/otel"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func withTraceProviderDatadog(opts ...tracer.StartOption) func() {
	if _, ok := os.LookupEnv("DD_VERSION"); !ok {
		opts = append(opts, tracer.WithServiceVersion(version.Version()))
	}

	provider := ddotel.NewTracerProvider(opts...)
	otel.SetTracerProvider(provider)

	return func() { provider.Shutdown() }
}
