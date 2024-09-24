package profile

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Expire time.Time `json:"expire"`
}

type TenantRepository interface {
	FetchTenant(ctx context.Context, id uuid.UUID) (*Tenant, error)
}
