package profile

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
)

var _ logger.Loggable = Profile{}

type Profile struct {
	TenantID uuid.UUID

	ID    uuid.UUID
	NIN   string
	Name  string
	Email string
	Phone string
	DOB   time.Time
}

func (p Profile) AsLog() any {
	return struct {
		TenantID uuid.UUID `json:"tenant_id"`
		ID       uuid.UUID `json:"id"`
		NIN      string    `json:"nin"`
		Name     string    `json:"name"`
		Email    string    `json:"email"`
		Phone    string    `json:"phone"`
		DOB      string    `json:"dob"`
	}{
		TenantID: p.TenantID,
		ID:       p.ID,
		NIN:      p.NIN[len(p.NIN)-3:],
		Name:     p.Name[:3],
		Email:    p.Email[:3],
		Phone:    p.Phone[:3],
		DOB:      p.DOB.Format(time.RFC3339)[:4],
	}
}

type ProfileRepository interface {
	StoreProfile(ctx context.Context, pr *Profile) (err error)
	FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *Profile, err error)
	FindProfileNames(ctx context.Context, tenantID uuid.UUID, query string) (names []string, err error)
	FindProfilesByName(ctx context.Context, tenantID uuid.UUID, name string) (prs []*Profile, err error)
}
