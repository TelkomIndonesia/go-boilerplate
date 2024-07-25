package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox/postgres"
)

func TestProfileBasic(t *testing.T) {
	ctx := context.Background()

	profiles := map[uuid.UUID]*profile.Profile{}

	p := tGetPostgresTruncated(t)
	pr := &profile.Profile{
		TenantID: tRequireUUIDV7(t),
		ID:       tRequireUUIDV7(t),
		NIN:      "0123456789",
		Name:     "Dohn Joe",
		Email:    "dohnjoe@email.com",
		Phone:    "+1234567",
		DOB:      time.Date(1991, 1, 1, 1, 1, 1, 1, time.Local),
	}

	profiles[pr.ID] = pr
	require.NoError(t, p.StoreProfile(ctx, pr), "should successfully store profile")
	for i := 1; i < 20; i++ {
		pr := &profile.Profile{
			TenantID: pr.TenantID,
			ID:       tRequireUUIDV7(t),
			NIN:      fmt.Sprintf("%s-%d", pr.NIN, i),
			Name:     fmt.Sprintf("%s-%d", pr.Name, i),
			Email:    fmt.Sprintf("%s-%d", pr.Email, i),
			Phone:    fmt.Sprintf("%s-%d", pr.Phone, i),
			DOB:      pr.DOB,
		}
		profiles[pr.ID] = pr
		require.NoErrorf(t, p.StoreProfile(ctx, pr), "should successfully store profile for index %d", i)
	}

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

	t.Run("outbox", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		ob, err := postgres.New(
			postgres.WithDB(p.db, p.dbUrl),
			postgres.WithTenantAEAD(p.aead),
			postgres.WithMaxNotifyWait(0))
		require.NoError(t, err)

		i := 0
		ob.Observe(ctx, func(ctx context.Context, obs []outbox.Outbox[outbox.Serialized]) error {
			for _, ob := range obs {
				if i++; i >= len(profiles) {
					cancel()
				}

				assert.Equal(t, outboxEventProfileStored, ob.EventName, "should store correct event name")
				assert.Equal(t, outboxTypeProfile, ob.ContentType, "shoulld store correct content type")
				assert.True(t, ob.IsEncrypted, "should store as encrypted ")

				var p profile.Profile
				require.NoError(t, ob.Content.Unmarshal(&p))
				require.NotNil(t, profiles[p.ID], "should store correct profile")
				assert.Equal(t, *profiles[p.ID], p, "should store correct profile")
			}
			return nil
		})
		assert.Equal(t, len(profiles), i, "should store all profile")
	})
}
