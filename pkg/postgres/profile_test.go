package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
)

func TestProfileBasic(t *testing.T) {
	p := getPostgres(t)
	pr := &profile.Profile{
		TenantID: requireUUIDV7(t),
		ID:       requireUUIDV7(t),
		NIN:      "0123456789",
		Name:     "Dohn Joe",
		Email:    "dohnjoe@email.com",
		Phone:    "+1234567",
		DOB:      time.Date(1991, 1, 1, 1, 1, 1, 1, time.Local),
	}
	ctx := context.Background()
	require.NoError(t, p.StoreProfile(ctx, pr), "should successfully store profile")

	prf, err := p.FetchProfile(ctx, pr.TenantID, pr.ID)
	require.NoError(t, err, "should successfully fetch profile")
	assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
	assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
	assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
	assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
	assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")

	prs, err := p.FindProfilesByName(ctx, pr.TenantID, pr.Name)
	require.NoError(t, err, "should successfully find profile")
	require.Len(t, prs, 1, "should only return 1 profile")
	prf = prs[0]
	assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
	assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
	assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
	assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
	assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")
}
