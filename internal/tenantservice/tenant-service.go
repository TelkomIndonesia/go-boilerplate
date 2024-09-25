package tenantservice

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/internal/profile"
	"github.com/telkomindonesia/go-boilerplate/internal/tenantservice/internal/oapi/tenant"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type OptFunc func(*TenantService) error

func WithHTTPClient(hc *http.Client) OptFunc {
	return func(ts *TenantService) error {
		if hc == nil {
			return fmt.Errorf("nil http client passed")
		}
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
		ts.base, err = util.URLWithDefaultScheme(u, "https")
		return
	}
}

func WithLogger(l log.Logger) OptFunc {
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
	logger log.Logger

	tc tenant.ClientWithResponsesInterface
}

func New(opts ...OptFunc) (ts *TenantService, err error) {
	ts = &TenantService{
		hc:     http.DefaultClient,
		tracer: otel.Tracer("tenant-service"),
		logger: log.Global(),
	}
	ts.base, _ = url.Parse("http://localhost")
	for _, opt := range opts {
		if err = opt(ts); err != nil {
			return nil, fmt.Errorf("failed to instantiate tenant service")
		}
	}
	if ts.hc == nil {
		return nil, fmt.Errorf("missing http client")
	}
	ts.tc, err = tenant.NewClientWithResponses(ts.base.String(), tenant.WithHTTPClient(ts.hc))
	if ts.logger == nil {
		return nil, fmt.Errorf("missing logger")
	}
	return
}

func (ts TenantService) FetchTenant(ctx context.Context, id uuid.UUID) (t *profile.Tenant, err error) {
	ctx, span := ts.tracer.Start(ctx, "fetchTenant", trace.WithAttributes(
		attribute.Stringer("id", id),
	))
	defer span.End()

	res, err := ts.tc.GetTenantWithResponse(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tenant: %w", err)
	}
	if res.JSONDefault == nil {
		return nil, nil
	}

	t = &profile.Tenant{
		ID:     res.JSONDefault.Id,
		Name:   res.JSONDefault.Name,
		Expire: res.JSONDefault.Expire,
	}
	return
}
