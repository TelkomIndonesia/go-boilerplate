package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type OptFunc func(h *HTTPServer) error

func WithProfileRepository(pr profile.ProfileRepository) OptFunc {
	return func(h *HTTPServer) error {
		h.profileRepo = pr
		return nil
	}
}

func WithTenantRepository(tr profile.TenantRepository) OptFunc {
	return func(h *HTTPServer) error {
		h.tenantRepo = tr
		return nil
	}
}

func WithTLS(keyPath, certPath string) OptFunc {
	return func(h *HTTPServer) (err error) {
		h.cw, err = newCertWatcher(keyPath, certPath, h.logger)
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
		h.tracerName = name
		h.tracer = otel.Tracer(name)
		return nil
	}
}

func WithLogger(logger logger.Logger) OptFunc {
	return func(h *HTTPServer) error {
		h.logger = logger
		return nil
	}
}

type HTTPServer struct {
	profileRepo profile.ProfileRepository
	tenantRepo  profile.TenantRepository
	profileMgr  profile.ProfileManager

	addr       string
	cw         *certWatcher
	handler    *echo.Echo
	server     *http.Server
	tracerName string
	tracer     trace.Tracer
	logger     logger.Logger
}

func New(opts ...OptFunc) (h *HTTPServer, err error) {
	h = &HTTPServer{
		handler:    echo.New(),
		addr:       ":80",
		tracerName: "httpserver",
		tracer:     otel.Tracer("httpserver"),
		logger:     logger.Global(),
	}
	for _, opt := range opts {
		if err = opt(h); err != nil {
			return
		}
	}
	if h.cw != nil {
		h.cw.logger = h.logger
	}

	if h.profileRepo == nil || h.tenantRepo == nil {
		return nil, fmt.Errorf("profile repo and tenant repo required")
	}
	h.profileMgr = profile.ProfileManager{PR: h.profileRepo, TR: h.profileMgr.TR}

	err = h.buildHandlers()
	return
}

func (h HTTPServer) buildHandlers() (err error) {
	h.handler.Use(otelecho.Middleware(h.tracerName))
	h.setProfileGroup()
	h.server = &http.Server{
		Addr:    h.addr,
		Handler: h.handler,
	}
	if h.cw != nil {
		h.server.TLSConfig = &tls.Config{
			GetCertificate: h.cw.GetCertificateFunc(),
		}
	}
	return
}

func (h HTTPServer) Start(ctx context.Context) (err error) {
	return h.server.ListenAndServe()
}

func (h HTTPServer) Close(ctx context.Context) (err error) {
	err = h.server.Shutdown(ctx)
	err = errors.Join(err, h.cw.Close())
	return
}
