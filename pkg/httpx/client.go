package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type ClientOptFunc func(*Client) error

type Dialer func(ctx context.Context, network string, addr string) (net.Conn, error)

func ClientWithDialTLS(f Dialer) ClientOptFunc {
	return func(h *Client) error {
		h.tr.DialTLSContext = f
		return nil
	}
}

func ClientWithDial(f Dialer) ClientOptFunc {
	return func(h *Client) error {
		h.tr.DialContext = f
		return nil
	}
}

type Client struct {
	*http.Client
	tr *http.Transport
}

func NewClient(opts ...ClientOptFunc) (h Client, err error) {
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

func (h Client) Close(ctx context.Context) error {
	h.Client.CloseIdleConnections()
	return nil
}
