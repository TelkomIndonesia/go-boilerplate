package otel

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/joho/godotenv"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"go.opentelemetry.io/otel"
)

func FromEnv(ctx context.Context, l logger.Logger) (deferer func()) {
	godotenv.Load()

	otel.SetLogger(logr.New(logsink{l: l, name: "otel"}))

	switch os.Getenv("OPENTELEMETRY_TRACE_PROVIDER") {
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
