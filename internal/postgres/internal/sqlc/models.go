// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package sqlc

import (
	"time"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/internal/postgres/internal/sqlc/types"
)

type Profile struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Nin       types.AEADString
	NinBidx   types.BIDXString
	Name      types.AEADString
	NameBidx  types.BIDXString
	Phone     types.AEADString
	PhoneBidx types.BIDXString
	Email     types.AEADString
	EmailBidx types.BIDXString
	Dob       types.AEADTime
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TextHeap struct {
	TenantID uuid.UUID
	Type     string
	Content  string
}
