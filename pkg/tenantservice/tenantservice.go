package tenantservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type OptFunc func(*TenantService) error

func WithHTTPClient(hc *http.Client) OptFunc {
	return func(ts *TenantService) error {
		ts.hc = hc
		return nil
	}
}

func WithTracer(name string) OptFunc {
	return func(ts *TenantService) error {
		ts.tracer = otel.Tracer(name)
		return nil
	}
}

func WithBaseUrl(u string) OptFunc {
	return func(ts *TenantService) (err error) {
		ts.base, err = url.Parse(u)
		return
	}
}

func WithLogger(l logger.Logger) OptFunc {
	return func(ts *TenantService) (err error) {
		ts.logger = l
		return
	}
}

var _ profile.TenantRepository = TenantService{}

type TenantService struct {
	base   *url.URL
	hc     *http.Client
	tracer trace.Tracer
	logger logger.Logger
}

func New(opts ...OptFunc) (ts *TenantService, err error) {
	ts = &TenantService{
		hc:     http.DefaultClient,
		tracer: otel.Tracer("tenant-service"),
		logger: logger.Global(),
	}
	ts.base, _ = url.Parse("http://localhost")
	for _, opt := range opts {
		if err = opt(ts); err != nil {
			return nil, fmt.Errorf("fail to instantiate tenant service")
		}
	}
	return
}

func (ts TenantService) FetchTenant(ctx context.Context, id uuid.UUID) (t *profile.Tenant, err error) {
	_, span := ts.tracer.Start(ctx, "fetchTenant", trace.WithAttributes(
		attribute.Stringer("id", id),
	))
	defer span.End()

	res, err := ts.hc.Get(ts.base.JoinPath("tenants", id.String()).String())
	if err != nil {
		return nil, fmt.Errorf("fail to invoke http request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	err = json.NewDecoder(res.Body).Decode(&t)
	if err != nil {
		return nil, fmt.Errorf("fail to deserialize tenant: %w", err)
	}
	return
}
