package logotel

import (
	"context"
	"errors"
	"testing"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestHandler(t *testing.T) {
	p := trace.NewTracerProvider()
	defer p.Shutdown(context.Background())

	otel.SetTracerProvider(p)
	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	exp, _ := stdoutlog.New()
	global.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp))))

	l := log.NewLogger()
	l.WithTrace().Info(ctx, "test",
		log.String("hello", "world"),
		log.Error("error", errors.New("test")))

}
