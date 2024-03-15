package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type OptFunc func(h *HTTPServer) error

func WithProfileRepository(pr profile.ProfileRepository) OptFunc {
	return func(h *HTTPServer) (err error) {
		h.profileRepo = pr
		return
	}
}

func WithTenantRepository(tr profile.TenantRepository) OptFunc {
	return func(h *HTTPServer) (err error) {
		h.tenantRepo = tr
		return nil
	}
}

func WithListener(l net.Listener) OptFunc {
	return func(h *HTTPServer) (err error) {
		h.listener = l
		return
	}
}

func WithTracer(name string) OptFunc {
	return func(h *HTTPServer) (err error) {
		h.tracerName = name
		h.tracer = otel.Tracer(name)
		return
	}
}

func WithLogger(logger logger.Logger) OptFunc {
	return func(h *HTTPServer) (err error) {
		h.logger = logger
		return
	}
}

type HTTPServer struct {
	profileRepo profile.ProfileRepository
	tenantRepo  profile.TenantRepository
	profileMgr  profile.ProfileManager

	listener   net.Listener
	handler    *echo.Echo
	server     *http.Server
	tracerName string
	tracer     trace.Tracer
	logger     logger.Logger
}

func New(opts ...OptFunc) (h *HTTPServer, err error) {
	h = &HTTPServer{
		handler:    echo.New(),
		tracerName: "httpserver",
		tracer:     otel.Tracer("httpserver"),
		logger:     logger.Global(),
	}
	for _, opt := range opts {
		if err = opt(h); err != nil {
			return
		}
	}

	if h.profileRepo == nil || h.tenantRepo == nil {
		return nil, fmt.Errorf("profile repo and tenant repo required")
	}
	h.profileMgr = profile.ProfileManager{PR: h.profileRepo, TR: h.profileMgr.TR}

	err = h.buildHandlers()
	return
}

func (h *HTTPServer) HealthCheckHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Server is healthy")
}

func (h *HTTPServer) registerHealthCheck() {
	h.handler.GET("/health", h.HealthCheckHandler)
}

func (h *HTTPServer) buildHandlers() (err error) {
	h.handler.Use(otelecho.Middleware(h.tracerName))
	h.setProfileGroup()
	h.tenantPassthrough()
	h.registerHealthCheck()

	h.server = &http.Server{
		Handler:  h.handler,
		ErrorLog: logger.NewGoLogger(h.logger, "http_server: ", 0),
	}
	return
}

func (h HTTPServer) Start(ctx context.Context) (err error) {
	if h.listener == nil {
		return h.server.ListenAndServe()
	}

	return h.server.Serve(h.listener)
}

func (h HTTPServer) Close(ctx context.Context) (err error) {
	err = h.server.Shutdown(ctx)
	return
}
