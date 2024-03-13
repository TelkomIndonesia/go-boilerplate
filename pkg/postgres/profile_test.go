package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
)

func TestProfileBasic(t *testing.T) {
	p := getPostgres(t)
	ctx := context.Background()
	profiles := map[uuid.UUID]*profile.Profile{}
	outboxes := make([]*profile.Profile, 0, 21)

	outboxesWG := sync.WaitGroup{}
	outboxesWG.Add(1)
	go func() {
		defer outboxesWG.Done()

		ctx, cancel := context.WithCancel(ctx)
		defer time.AfterFunc(30*time.Second, cancel).Stop()

		p.obSender = func(ctx context.Context, o []*Outbox) error {
			for _, o := range o {
				pr := &profile.Profile{}
				require.NoError(t, json.Unmarshal(o.Content, &pr), "should return valid json")
				outboxes = append(outboxes, pr)
			}
			if len(outboxes) == cap(outboxes) {
				cancel()
			}
			return nil
		}
		p.watchOutboxes(ctx)

		assert.Len(t, outboxes, cap(outboxes), "should send all outbox")
	}()

	pr := &profile.Profile{
		TenantID: requireUUIDV7(t),
		ID:       requireUUIDV7(t),
		NIN:      "0123456789",
		Name:     "Dohn Joe",
		Email:    "dohnjoe@email.com",
		Phone:    "+1234567",
		DOB:      time.Date(1991, 1, 1, 1, 1, 1, 1, time.UTC),
	}
	profiles[pr.ID] = pr
	require.NoError(t, p.StoreProfile(ctx, pr), "should successfully store profile")
	for i := 1; i < cap(outboxes); i++ {
		pr := &profile.Profile{
			TenantID: pr.TenantID,
			ID:       requireUUIDV7(t),
			NIN:      fmt.Sprintf("%s-%d", pr.NIN, i),
			Name:     fmt.Sprintf("%s-%d", pr.Name, i),
			Email:    fmt.Sprintf("%s-%d", pr.Email, i),
			Phone:    fmt.Sprintf("%s-%d", pr.Phone, i),
			DOB:      pr.DOB,
		}
		profiles[pr.ID] = pr
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

	outboxesWG.Wait()
	for _, pr := range outboxes {
		prf, ok := profiles[pr.ID]
		require.True(t, ok, "should return stored outbox")
		assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
		assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
		assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
		assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
		assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")
	}
}
