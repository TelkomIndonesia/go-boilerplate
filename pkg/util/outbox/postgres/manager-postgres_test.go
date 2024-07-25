package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox"
)

var testPostgres *postgres
var testAEAD *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
var testPostgresSync, testKeysetHandleSync sync.Mutex

func tGetManagerPostgresTruncated(t *testing.T) *postgres {
	p := tGetManagerPostgres(t)

	sqlStatement := `TRUNCATE TABLE outbox RESTART IDENTITY CASCADE`
	_, err := p.db.Exec(sqlStatement)
	require.NoError(t, err)

	return testPostgres
}

func tGetManagerPostgres(t *testing.T) *postgres {
	if testPostgres == nil {
		testPostgresSync.Lock()
		defer testPostgresSync.Unlock()
	}

	if testPostgres == nil {
		testPostgres = tNewManagerPostgres(t)
	}

	return testPostgres
}

func tNewManagerPostgres(t *testing.T, opts ...OptFunc) *postgres {
	url, ok := os.LookupEnv("TEST_POSTGRES_URL")
	if !ok {
		t.Skip("no postgres url found")
	}

	db, err := sql.Open("postgres", url)
	require.NoError(t, err)

	p, err := New(append(opts,
		WithDB(db, url),
		WithTenantAEAD(tGetKeysetHandle(t)))...,
	)
	require.NoError(t, err, "should create postgres")
	return p.(*postgres)
}

func tGetKeysetHandle(t *testing.T) (aeadh *crypt.DerivableKeyset[crypt.PrimitiveAEAD]) {
	if testAEAD == nil {
		testKeysetHandleSync.Lock()
		defer testKeysetHandleSync.Unlock()
	}

	var err error
	if testAEAD == nil {
		testAEAD, err = crypt.NewInsecureCleartextDerivableKeyset("./testdata/tink-aead.json", crypt.NewPrimitiveAEAD)
		require.NoError(t, err, "should create aead derivable keyset")
	}

	return testAEAD
}

func TestNewManagerPostgres(t *testing.T) {
	p := tGetManagerPostgres(t)
	require.NotNil(t, p)
}

func TestPostgresOutbox(t *testing.T) {
	manager := tGetManagerPostgresTruncated(t)

	for _, isEncrypted := range []bool{false, true, false, true} {
		t.Run(fmt.Sprintf("encrypted:%v", isEncrypted), func(t *testing.T) {
			ctx := context.Background()

			ctype := "data" + uuid.NewString()
			event := "data_incoming" + uuid.NewString()
			contents := map[string]map[string]interface{}{}
			for i := 0; i < 30+rand.Int()%10; i++ {
				id := uuid.New().String()
				contents[id] = map[string]interface{}{
					"id":   id,
					"test": uuid.New().String(),
				}
			}

			sentOutboxes := []outbox.Outbox[outbox.Serialized]{}

			// start replicas of manager that should wait for outboxes
			outboxesWG := sync.WaitGroup{}
			{
				ctx, cancel := context.WithTimeout(ctx, time.Minute)

				i := -1
				sender := func(ctx context.Context, obs []outbox.Outbox[outbox.Serialized]) error {
					if i = i + 1; i%2 == 0 {
						return fmt.Errorf("simulated intermittent error")
					}

					sentOutboxes = append(sentOutboxes, obs...)

					if len(sentOutboxes) >= len(contents) {
						time.AfterFunc(time.Second, cancel)
					}
					return nil
				}

				replica := 10
				outboxesWG.Add(replica)
				for i := 0; i < replica; i++ {
					go func() {
						defer outboxesWG.Done()

						p := tNewManagerPostgres(t)
						p.maxNotifyWait = time.Second
						p.limit = 10
						defer p.db.Close()

						outbox.ObserveWithRetry(ctx, p, sender, nil)
					}()
				}
			}

			// store data
			{
				wg := sync.WaitGroup{}
				wg.Add(len(contents))
				for _, content := range contents {
					go func() {
						defer wg.Done()

						tx, err := manager.db.Begin()
						require.NoError(t, err)
						defer tx.Commit()

						outbox, err := outbox.New(uuid.New(), event, ctype, content)
						require.NoError(t, err)

						if isEncrypted {
							err = manager.StoreAsEncrypted(ctx, tx, outbox)
						} else {
							err = manager.Store(ctx, tx, outbox)
						}
						require.NoError(t, err, "should store outbox")
					}()
				}
				wg.Wait()
			}

			// check sent outboxes
			{
				outboxesWG.Wait()
				assert.Len(t, sentOutboxes, len(contents), "should send all new outbox")
				for _, o := range sentOutboxes {
					assert.Equal(t, ctype, o.ContentType, "should contain valid content type")
					assert.Equal(t, event, o.EventName, "should contain valid event name")
					assert.Equal(t, isEncrypted, o.IsEncrypted, "should contains correct encryption status")

					pr := map[string]interface{}{}
					assert.NoError(t, o.Content.Unmarshal(&pr), "should return valid json")

					c, ok := contents[pr["id"].(string)]
					assert.True(t, ok, "should contains expected content")
					assert.Equal(t, c, pr, "should contains valid content")
				}
			}
		})
	}

}
