package oteloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/version"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	otelog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func FromEnv(ctx context.Context, l log.Logger) (deferer func(ctx context.Context), errs error) {
	if l != nil {
		otel.SetLogger(logr.New(logger{l: l, name: "otel"}))
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			l.Error(ctx, "otel error", log.String("error", err.Error()))
		}))
	}

	lp, lpCloser, err := logProviderFromEnv(ctx)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to create log provider: %w", errs))
	} else {
		global.SetLoggerProvider(lp)
	}

	tp, tpCloser, err := traceProviderFromEnv(ctx, l)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to create trace provider: %w", errs))
	} else {
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())
	}

	mp, mpCloser, err := meterProviderFromEnv(ctx)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to create meter provider: %w", errs))
	} else {
		otel.SetMeterProvider(mp)
	}

	deferer = func(ctx context.Context) {
		tpCloser(ctx)
		lpCloser(ctx)
		mpCloser(ctx)
	}
	return
}

func logProviderFromEnv(ctx context.Context) (otelog.LoggerProvider, func(context.Context), error) {
	ex, err := autoexport.NewLogExporter(ctx,
		autoexport.WithFallbackLogExporter(func(ctx context.Context) (sdklog.Exporter, error) {
			return noopLogExporter{}, nil
		}))
	if err != nil {
		return nil, noopDeferer, fmt.Errorf("failed to create log exporter: %w", err)
	}

	return sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(ex))),
		func(ctx context.Context) { ex.Shutdown(ctx) },
		nil
}

func traceProviderFromEnv(ctx context.Context, l log.Logger) (trace.TracerProvider, func(context.Context), error) {
	ex, err := autoexport.NewSpanExporter(ctx,
		autoexport.WithFallbackSpanExporter(func(ctx context.Context) (sdktrace.SpanExporter, error) {
			return noopSpanExporter{}, nil
		}))

	if (err != nil || autoexport.IsNoneSpanExporter(ex)) && "datadog" == strings.ToLower(os.Getenv("OTEL_TRACES_EXPORTER")) {
		opts := []tracer.StartOption{
			tracer.WithLogger(logger{l: l, name: "otel"}),
		}
		if _, ok := os.LookupEnv("DD_VERSION"); !ok {
			opts = append(opts, tracer.WithServiceVersion(version.Version()))
		}

		provider := ddotel.NewTracerProvider(opts...)
		return provider,
			func(context.Context) { provider.Shutdown() },
			nil
	}

	if err != nil {
		return nil,
			noopDeferer,
			fmt.Errorf("failed to create span exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(sdktrace.WithBatcher(ex)),
		func(ctx context.Context) { ex.Shutdown(ctx) },
		nil
}

func meterProviderFromEnv(ctx context.Context) (metric.MeterProvider, func(context.Context), error) {
	ex, err := autoexport.NewMetricReader(ctx,
		autoexport.WithFallbackMetricReader(func(ctx context.Context) (sdkmetric.Reader, error) {
			return newNoopMetricReader(), nil
		}),
	)
	if err != nil {
		return nil, noopDeferer, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(sdkmetric.WithReader(ex)),
		func(ctx context.Context) { ex.Shutdown(ctx) },
		nil
}
