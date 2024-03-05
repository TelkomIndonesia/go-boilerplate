package profile

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	TenantID uuid.UUID

	ID    uuid.UUID
	NIN   string
	Name  string
	Email string
	Phone string
	DOB   time.Time
}

type ProfileRepository interface {
	StoreProfile(ctx context.Context, pr *Profile) (err error)
	FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *Profile, err error)
	FindProfileNames(ctx context.Context, tenantID uuid.UUID, query string) (names []string, err error)
	FindProfilesByName(ctx context.Context, tenantID uuid.UUID, name string) (prs []*Profile, err error)
}
