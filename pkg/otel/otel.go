package otel

import (
	"context"
	"os"

	"github.com/joho/godotenv"
)

func FromEnv(ctx context.Context) (deferer func()) {
	godotenv.Load()
	switch os.Getenv("OPENTELEMETRY_TRACE_PROVIDER") {
	case "datadog":
		return withTraceProviderDatadog()
	case "otlphttp":
		return withTraceOTLPHTTPExporter(ctx)
	default:
		return withTraceConsoleExporter(ctx)
	}
}
