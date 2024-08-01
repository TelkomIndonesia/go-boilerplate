//go:generate go run github.com/bufbuild/buf/cmd/buf generate
package outbox

import (
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func FromProfile(pr *profile.Profile) *Outbox {
	return &Outbox{
		Content: &Outbox_Profile{
			Profile: &Profile{
				ID:       pr.ID.String(),
				TenantID: pr.TenantID.String(),
				NIN:      pr.NIN,
				Email:    pr.Email,
				Name:     pr.Name,
				Phone:    pr.Phone,
				DOB:      timestamppb.New(pr.DOB),
			},
		},
	}
}
