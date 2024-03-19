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
	ctx := context.Background()

	profiles := map[uuid.UUID]*profile.Profile{}
	outboxes := make([]*profile.Profile, 0, 21)

	outboxesWG := sync.WaitGroup{}
	outboxesWG.Add(3)
	{
		ctx, cancel := context.WithCancel(ctx)
		defer time.AfterFunc(30*time.Second, cancel).Stop()

		for i := 0; i < 3; i++ {
			go func() {
				defer outboxesWG.Done()

				p := tNewPostgres(t,
					WithOutboxSender(func(ctx context.Context, obs []*Outbox) error {
						for _, o := range obs {
							o, err := o.AsUnEncrypted()
							assert.NoError(t, err, "should return unencrypted outbox")

							pr := &profile.Profile{}
							assert.NoError(t, json.Unmarshal(o.ContentByte(), &pr), "should return valid json")
							if _, ok := profiles[pr.ID]; !ok {
								continue
							}
							outboxes = append(outboxes, pr)
						}
						if len(outboxes) == cap(outboxes) {
							cancel()
						}
						return nil
					}),
				)
				defer p.Close(context.Background())

				<-ctx.Done()
			}()
		}
	}

	p := tGetPostgres(t)
	pr := &profile.Profile{
		TenantID: tRequireUUIDV7(t),
		ID:       tRequireUUIDV7(t),
		NIN:      "0123456789",
		Name:     "Dohn Joe",
		Email:    "dohnjoe@email.com",
		Phone:    "+1234567",
		DOB:      time.Date(1991, 1, 1, 1, 1, 1, 1, time.UTC),
	}
	t.Run("store", func(t *testing.T) {
		require.NoError(t, p.StoreProfile(ctx, pr), "should successfully store profile")
		profiles[pr.ID] = pr
		for i := 1; i < cap(outboxes); i++ {
			pr := &profile.Profile{
				TenantID: pr.TenantID,
				ID:       tRequireUUIDV7(t),
				NIN:      fmt.Sprintf("%s-%d", pr.NIN, i),
				Name:     fmt.Sprintf("%s-%d", pr.Name, i),
				Email:    fmt.Sprintf("%s-%d", pr.Email, i),
				Phone:    fmt.Sprintf("%s-%d", pr.Phone, i),
				DOB:      pr.DOB,
			}
			require.NoErrorf(t, p.StoreProfile(ctx, pr), "should successfully store profile for index %d", i)
			profiles[pr.ID] = pr
		}
	})

	t.Run("fetch", func(t *testing.T) {
		prf, err := p.FetchProfile(ctx, pr.TenantID, pr.ID)
		require.NoError(t, err, "should successfully fetch profile")
		assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
		assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
		assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
		assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
		assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")
	})

	t.Run("findName", func(t *testing.T) {
		names, err := p.FindProfileNames(ctx, pr.TenantID, pr.Name)
		require.NoError(t, err, "should successfully find name")
		assert.Len(t, names, len(profiles), "should only return 1 name")
		for _, name := range names {
			found := false
			for _, pr := range profiles {
				if pr.Name != name {
					continue
				}
				found = true
				break
			}
			assert.Truef(t, found, "returned name (%s) should be valid", name)
		}
	})

	t.Run("findByName", func(t *testing.T) {
		prsf, err := p.FindProfilesByName(ctx, pr.TenantID, pr.Name)
		require.NoError(t, err, "should successfully find profile")
		require.Len(t, prsf, 1, "should only return 1 profile")
		prf := prsf[0]
		require.NoError(t, err, "should successfully find profile")
		assert.Equal(t, pr.NIN, prf.NIN, "NIN should be equal")
		assert.Equal(t, pr.Name, prf.Name, "Name should be equal")
		assert.Equal(t, pr.Email, prf.Email, "Email should be equal")
		assert.Equal(t, pr.Phone, prf.Phone, "Phone should be equal")
		assert.Equal(t, pr.DOB, prf.DOB, "DOB should be equal")
	})

	t.Run("outboxSend", func(t *testing.T) {
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
		assert.Len(t, outboxes, cap(outboxes), "should send all outbox")
	})
}
