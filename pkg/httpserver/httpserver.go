package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver/internal/oapi"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
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

func WithLogger(logger log.Logger) OptFunc {
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
	logger     log.Logger
}

func New(opts ...OptFunc) (h *HTTPServer, err error) {
	h = &HTTPServer{
		handler:    echo.New(),
		tracerName: "httpserver",
		tracer:     otel.Tracer("httpserver"),
		logger:     log.Global(),
	}
	for _, opt := range opts {
		if err = opt(h); err != nil {
			return
		}
	}

	if h.logger == nil {
		return nil, fmt.Errorf("missing logger")
	}
	if h.profileRepo == nil || h.tenantRepo == nil {
		return nil, fmt.Errorf("profile repo and tenant repo required")
	}
	h.profileMgr = profile.ProfileManager{PR: h.profileRepo, TR: h.tenantRepo}

	err = h.buildServer()
	return
}

func (h *HTTPServer) buildServer() (err error) {
	h.handler.Use(otelecho.Middleware(h.tracerName))
	h.registerHealthCheck().
		registerTenantPassthrough()

	oapi.RegisterHandlers(h.handler,
		oapi.NewStrictHandler(oapiServerImplementation{h: h}, nil))

	h.server = &http.Server{
		Handler:  h.handler,
		ErrorLog: log.NewGoLogger(h.logger, "http_server: ", 0),
	}
	return
}

func (h *HTTPServer) registerHealthCheck() *HTTPServer {
	h.handler.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "Server is healthy")
	})
	return h
}

func (h *HTTPServer) registerTenantPassthrough() *HTTPServer {
	h.handler.GET("/tenants/:tenantid", func(c echo.Context) error {
		tid, err := uuid.Parse(c.Param("tenantid"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid tenant id")
		}
		t, err := h.tenantRepo.FetchTenant(c.Request().Context(), tid)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, t)
	})
	return h
}

func (h HTTPServer) Start(ctx context.Context) (err error) {
	go func() {
		<-ctx.Done()
		err = errors.Join(err, h.server.Shutdown(ctx))
	}()

	if h.listener == nil {
		return h.server.ListenAndServe()
	}

	return errors.Join(err, h.server.Serve(h.listener))
}

func (h HTTPServer) Close(ctx context.Context) (err error) {
	return h.server.Shutdown(ctx)
}
