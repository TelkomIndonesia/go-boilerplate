package oteloader

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
)

func TestInstantiate(t *testing.T) {
	ctx := context.Background()

	table := []struct {
		name string
		t    interface{}
	}{
		{
			name: "datadog",
			t:    &ddotel.TracerProvider{},
		},
		{
			name: "otlp",
			t:    &trace.TracerProvider{},
		},
		{
			name: "console",
			t:    &trace.TracerProvider{},
		},
	}

	for _, data := range table {
		t.Setenv("OTEL_TRACES_EXPORTER", data.name)
		FromEnv(ctx, log.Global())
		p := otel.GetTracerProvider()
		assert.IsType(t, data.t, p)
	}
}
