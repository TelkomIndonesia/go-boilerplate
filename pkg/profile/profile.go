package profile

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
)

var _ logger.Loggable = Profile{}

type Profile struct {
	TenantID uuid.UUID `json:"tenant_id"`
	ID       uuid.UUID `json:"id"`
	NIN      string    `json:"nin"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Phone    string    `json:"phone"`
	DOB      time.Time `json:"dob"`
}

func (p Profile) AsLog() any {
	var name, email, phone, nin string 
	if len(p.Name) < 3 {
		name = p.Name
	} else {
		name = p.Name[:3]
	}

	if len(p.Email) < 3 {
		email = p.Email
	} else {
		email = p.Email[:3]
	}

	if len(p.Phone) < 3 {
		phone = p.Phone
	} else {
		phone = p.Phone[:3]
	}

	if len(p.NIN) < 3 {
		nin = p.NIN
	} else {
		nin = p.NIN[:3]
	}

	return Profile{
		TenantID: p.TenantID,
		ID:       p.ID,
		NIN:      "***" + nin,
		Name:     name + "***",
		Email:    email + "***",
		Phone:    phone + "***",
		DOB:      time.Date(p.DOB.Year(), 1, 1, 0, 0, 0, 0, p.DOB.Location()),
	}
}

type ProfileRepository interface {
	StoreProfile(ctx context.Context, pr *Profile) (err error)
	FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *Profile, err error)
	FindProfileNames(ctx context.Context, tenantID uuid.UUID, query string) (names []string, err error)
	FindProfilesByName(ctx context.Context, tenantID uuid.UUID, name string) (prs []*Profile, err error)
}
