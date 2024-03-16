package otel

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"go.opentelemetry.io/otel"
)

func WithTraceProvider(ctx context.Context, name string, l logger.Logger) (deferer func()) {
	if l != nil {
		otel.SetLogger(logr.New(logsink{l: l, name: "otel"}))
	}

	switch name {
	case "datadog":
		return withTraceProviderDatadog()
	case "otlphttp":
		return withTraceOTLPHTTPExporter(ctx)
	case "console":
		return withTraceConsoleExporter(ctx)
	default:
		return
	}
}
