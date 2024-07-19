package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
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

func tNewManagerPostgres(t *testing.T, opts ...ManagerPostgresOptFunc) *postgres {
	url, ok := os.LookupEnv("TEST_POSTGRES_URL")
	if !ok {
		t.Skip("no postgres url found")
	}

	db, err := sql.Open("postgres", url)
	require.NoError(t, err)

	p, err := NewManagerPostgres(append(opts,
		ManagerPostgresWithDB(db, url),
		ManagerPostgresWithTenantAEAD(tGetKeysetHandle(t)))...,
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
			count := 30 + rand.Int()%10
			contents := map[string]map[string]interface{}{}
			outboxes := []Outbox{}

			// start replicas of manager that should wait for outboxes
			outboxesWG := sync.WaitGroup{}
			{
				ctx, cancel := context.WithTimeout(ctx, time.Minute)

				i := -1
				obSender := func(ctx context.Context, obs []Outbox) error {
					if i = i + 1; i%2 == 0 {
						return fmt.Errorf("simulated intermittent error")
					}

					outboxes = append(outboxes, obs...)

					if len(outboxes) >= count {
						time.AfterFunc(time.Second, cancel)
					}
					return nil
				}

				replica := 10
				outboxesWG.Add(replica)
				for i := 0; i < replica; i++ {
					go func() {
						defer outboxesWG.Done()

						p := tNewManagerPostgres(t, ManagerPostgresWithSender(obSender))
						p.maxIdle = time.Second
						p.limit = 10
						defer p.db.Close()

						WatchOutboxesLoop(ctx, p, nil)
					}()
				}
			}

			// store data
			{
				wg := sync.WaitGroup{}
				wg.Add(count)
				for i := 0; i < count; i++ {
					id := uuid.New().String()
					contents[id] = map[string]interface{}{
						"id":   id,
						"test": uuid.New().String(),
					}
					outbox, err := NewOutbox(uuid.New(), event, ctype, contents[id])
					require.NoError(t, err)

					go func() {
						defer wg.Done()

						tx, err := manager.db.Begin()
						require.NoError(t, err)
						defer tx.Commit()

						if isEncrypted {
							err = manager.StoreOutboxEncrypted(ctx, tx, outbox)
						} else {
							err = manager.StoreOutbox(ctx, tx, outbox)
						}
						require.NoError(t, err)
					}()
				}
				wg.Wait()
			}

			// check sent outboxes
			{
				outboxesWG.Wait()
				assert.Len(t, outboxes, count, "should send all new outbox")
				for _, o := range outboxes {
					assert.Equal(t, ctype, o.ContentType)
					assert.Equal(t, event, o.Event)
					assert.Equal(t, isEncrypted, o.IsEncrypted)

					o, err := o.AsUnEncrypted()
					assert.NoError(t, err, "should return unencrypted outbox")

					pr := map[string]interface{}{}
					assert.NoError(t, json.Unmarshal(o.ContentByte(), &pr), "should return valid json")

					c, ok := contents[pr["id"].(string)]
					assert.True(t, ok, "should contains expected content")
					assert.Equal(t, c, pr)
				}
			}
		})
	}

}
