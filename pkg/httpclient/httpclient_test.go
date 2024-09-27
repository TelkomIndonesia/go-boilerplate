package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/oteloader"
	opentelemetry "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TestOtel(t *testing.T) {
	ctx := context.Background()
	oteloader.WithTraceProvider(ctx, "datadog", log.Global())

	_, span := opentelemetry.Tracer("test").
		Start(ctx, "test", trace.WithAttributes(
			attribute.String("test", t.Name()),
		))
	span.End()

	header := http.Header{}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header
		fmt.Fprintf(w, "hello world")
	}))
	defer svr.Close()

	req, err := http.NewRequest("GET", svr.URL, nil)
	require.NoError(t, err)
	h, err := New()
	require.NoError(t, err)
	res, err := h.Do(req.WithContext(ctx))
	require.NoError(t, err)
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(b))
	assert.NotEmpty(t, header.Get("Traceparent"))
}
