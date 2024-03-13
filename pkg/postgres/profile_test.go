package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
)

func TestProfileBasic(t *testing.T) {
	p := getPostgres(t)

	prs := map[uuid.UUID]*profile.Profile{}
	pr := &profile.Profile{
		TenantID: requireUUIDV7(t),
		ID:       requireUUIDV7(t),
		NIN:      "0123456789",
		Name:     "Dohn Joe",
		Email:    "dohnjoe@email.com",
		Phone:    "+1234567",
		DOB:      time.Date(1991, 1, 1, 1, 1, 1, 1, time.UTC),
	}
	prs[pr.ID] = pr
	ctx := context.Background()
	require.NoError(t, p.StoreProfile(ctx, pr), "should successfully store profile")
	for i := 0; i < 20; i++ {
		pr := &profile.Profile{
			TenantID: pr.TenantID,
			ID:       requireUUIDV7(t),
			NIN:      fmt.Sprintf("%s-%d", pr.NIN, i),
			Name:     fmt.Sprintf("%s-%d", pr.Name, i),
			Email:    fmt.Sprintf("%s-%d", pr.Email, i),
			Phone:    fmt.Sprintf("%s-%d", pr.Phone, i),
			DOB:      pr.DOB,
		}
		prs[pr.ID] = pr
		require.NoErrorf(t, p.StoreProfile(ctx, pr), "should successfully store profile for index %d", i)
	}

	prf, err := p.FetchProfile(ctx, pr.TenantID, pr.ID)
	require.NoError(t, err, "should successfully fetch profile")
	assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
	assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
	assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
	assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
	assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")

	prsf, err := p.FindProfilesByName(ctx, pr.TenantID, pr.Name)
	require.NoError(t, err, "should successfully find profile")
	require.Len(t, prsf, 1, "should only return 1 profile")
	prf = prsf[0]
	assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
	assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
	assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
	assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
	assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")

	p.obSender = func(ctx context.Context, o []*Outbox) error {
		for _, o := range o {
			pr := &profile.Profile{}
			err := json.Unmarshal(o.Content, &pr)
			require.NoError(t, err, "should return valid json")

			prf, ok := prs[pr.ID]
			require.True(t, ok, "should return stored outbox")
			assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
			assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
			assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
			assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
			assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")
		}
		assert.Len(t, o, 21)
		return nil
	}
	err = p.sendOutbox(ctx, 100)
	assert.NoError(t, err, "should successfully send outboxes")
}
