package oteloader

import (
	"context"

	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

type noopSpanExporter struct{}

func (noopSpanExporter) ExportSpans(context.Context, []trace.ReadOnlySpan) error { return nil }

func (noopSpanExporter) Shutdown(context.Context) error { return nil }

type noopMetricReader struct{ *metric.ManualReader }

func newNoopMetricReader() noopMetricReader {
	return noopMetricReader{metric.NewManualReader()}
}

type noopLogExporter struct{}

var _ log.Exporter = noopLogExporter{}

func (e noopLogExporter) Export(context.Context, []log.Record) error { return nil }
func (e noopLogExporter) Shutdown(context.Context) error             { return nil }
func (e noopLogExporter) ForceFlush(context.Context) error           { return nil }
