package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/cert"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type OptFunc func(*HTTPClient) error

func WithCAWatcher(path string, l logger.Logger) OptFunc {
	return func(h *HTTPClient) error {
		cw, err := cert.NewCAWatcher(h.tr.TLSClientConfig, path, true, l)
		if err != nil {
			return fmt.Errorf("failt to instantiate ca watcher")
		}
		h.cw = cw
		return nil
	}
}

type HTTPClient struct {
	*http.Client
	tr *http.Transport
	cw *cert.CAWatcher
}

func New(opts ...OptFunc) (h HTTPClient, err error) {
	h.tr = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSClientConfig:     &tls.Config{},
		TLSHandshakeTimeout: 5 * time.Second,
	}
	h.Client = &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, opt := range opts {
		if err = opt(&h); err != nil {
			return h, fmt.Errorf("fail to apply option: %w", err)
		}
	}
	h.Client.Transport = otelhttp.NewTransport(h.tr)
	return
}

func (h HTTPClient) Close(ctx context.Context) error {
	return h.cw.Close(ctx)
}
