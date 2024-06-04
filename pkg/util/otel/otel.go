package otel

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
)

func WithTraceProvider(ctx context.Context, name string, l log.Logger) (deferer func()) {
	if l != nil {
		otel.SetLogger(logr.New(logsink{l: l, name: "otel"}))
	}

	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	switch name {
	case "datadog":
		return withTraceProviderDatadog()
	case "otlphttp":
		return withTraceOTLPHTTPExporter(ctx)
	case "console":
		return withTraceConsoleExporter(ctx)
	default:
		return func() {}
	}
}
