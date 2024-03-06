package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

type OptFunc func(h *HTTPServer) error

func WithProfileRepository(pr profile.ProfileRepository) OptFunc {
	return func(h *HTTPServer) error {
		h.profileRepo = pr
		return nil
	}
}

func WithTLS(keyPath, certPath string) OptFunc {
	return func(h *HTTPServer) (err error) {
		logger := func(err error) {
			// TODO: share logger with `h`
			// e.g: h.logger().Log(err)
		}

		h.cw, err = newCertWatcher(keyPath, certPath, logger)
		if err != nil {
			return fmt.Errorf("failed to instantiate TLS Cert Watcher: %w", err)
		}

		return
	}
}

func WithAddr(addr string) OptFunc {
	return func(h *HTTPServer) error {
		h.addr = addr
		return nil
	}
}

func WithTracer(name string) OptFunc {
	return func(h *HTTPServer) error {
		h.tracerName = &name
		return nil
	}

}

type HTTPServer struct {
	profileRepo profile.ProfileRepository

	addr       string
	cw         *certWatcher
	handler    *echo.Echo
	tracerName *string
}

func New(opts ...OptFunc) (h *HTTPServer, err error) {
	h = &HTTPServer{
		handler: echo.New(),
		addr:    ":80",
	}
	for _, opt := range opts {
		if err = opt(h); err != nil {
			return
		}
	}
	err = h.buildHandlers()
	return
}

func (h HTTPServer) buildHandlers() (err error) {
	if h.tracerName != nil {
		h.handler.Use(otelecho.Middleware(*h.tracerName))
	}

	//TODO: build all handler here
	h.handler.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "")
	})
	return
}

func (h HTTPServer) Start(ctx context.Context) (err error) {
	s := &http.Server{
		Addr:    h.addr,
		Handler: h.handler,
	}
	if h.cw != nil {
		s.TLSConfig = &tls.Config{
			GetCertificate: h.cw.GetCertificateFunc(),
		}
	}
	go func() {
		<-ctx.Done()
		s.Close()
	}()

	return s.ListenAndServe()
}

func (h HTTPServer) Close() error {
	return h.cw.Close()
}
