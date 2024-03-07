package otel

import (
	"context"
	"os"

	"github.com/joho/godotenv"
)

func FromEnv(ctx context.Context) (deferer func()) {
	godotenv.Load()
	switch os.Getenv("OPENTELEMETRY_PROVIDER") {
	case "datadog":
		return withProviderDatadog()
	default:
		return withConsoleExporter(ctx)
	}
}
