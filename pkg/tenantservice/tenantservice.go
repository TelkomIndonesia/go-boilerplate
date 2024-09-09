package tenantservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
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
		ts.base, err = util.ParseURLWithDefaultScheme(u, "https")
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.base.JoinPath("tenants", id.String()).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	res, err := ts.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke http request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code")
	}

	err = json.NewDecoder(res.Body).Decode(&t)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize tenant: %w", err)
	}
	return
}
