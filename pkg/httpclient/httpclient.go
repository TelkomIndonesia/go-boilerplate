package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type OptFunc func(*HTTPClient) error

type Dialer func(ctx context.Context, network string, addr string) (net.Conn, error)

func WithDialTLS(f Dialer) OptFunc {
	return func(h *HTTPClient) error {
		h.tr.DialTLSContext = f
		return nil
	}
}

func WithDial(f Dialer) OptFunc {
	return func(h *HTTPClient) error {
		h.tr.DialContext = f
		return nil
	}
}

type HTTPClient struct {
	*http.Client
	tr *http.Transport
}

func New(opts ...OptFunc) (h HTTPClient, err error) {
	h.tr = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	h.Client = &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, opt := range opts {
		if err = opt(&h); err != nil {
			return h, fmt.Errorf("failed to apply option: %w", err)
		}
	}
	h.Client.Transport = otelhttp.NewTransport(h.tr)
	return
}

func (h HTTPClient) Close(ctx context.Context) error {
	return nil
}
