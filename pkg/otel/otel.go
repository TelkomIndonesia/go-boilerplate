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
	default:
		return withTraceConsoleExporter(ctx)
	}
}
