package tenantservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
)

type OptFunc func(*TenantService) error

func WithHTTPClient(hc *http.Client) OptFunc {
	return func(ts *TenantService) error {
		ts.hc = hc
		return nil
	}
}

var _ profile.TenantRepository = TenantService{}

type TenantService struct {
	base *url.URL
	hc   *http.Client
}

func New(opts ...OptFunc) (ts *TenantService, err error) {
	ts = &TenantService{
		hc: http.DefaultClient,
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
