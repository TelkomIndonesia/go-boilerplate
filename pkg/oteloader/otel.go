package oteloader

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
)

func WithTraceProvider(ctx context.Context, name string, l log.Logger) (deferer func()) {
	if l != nil {
		otel.SetLogger(logr.New(logsink{l: l, name: "otel"}))
	}

	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	switch name {
	case "console":
		return withTraceConsoleExporter(ctx, l)
	case "otlp", "otlpgrpc", "otlp-grpc":
		return withTraceOTLPGRPCExporter(ctx, l)
	case "otlphttp", "otlp-http":
		return withTraceOTLPHTTPExporter(ctx, l)
	case "datadog":
		return withTraceProviderDatadog()
	default:
		return func() {}
	}
}

func WithLogProvider(ctx context.Context, name string, l log.Logger) (deferer func()) {
	if l != nil {
		otel.SetLogger(logr.New(logsink{l: l, name: "otel"}))
	}

	switch name {
	case "console":
		return withLogConsoleExporter(ctx, l)
	case "otlp", "otlpgrpc", "otlp-grpc":
		return withLogOTLPGRPCExporter(ctx, l)
	case "otlphttp", "otlp-http":
		return withLogOTLPHTTPExporter(ctx, l)
	default:
		return func() {}
	}
}
