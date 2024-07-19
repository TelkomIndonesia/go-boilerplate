package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	for _, isEncrypted := range []bool{true, false} {
		t.Run(fmt.Sprintf("encrypted:%v", isEncrypted), func(t *testing.T) {
			ctype := "data" + uuid.NewString()
			event := "data_incoming" + uuid.NewString()
			contents := map[string]map[string]interface{}{}
			outboxes := make([]Outbox, 0, 21)

			ctx := context.Background()

			// start manager
			outboxesWG := sync.WaitGroup{}
			{
				ctx, cancel := context.WithCancel(ctx)
				defer time.AfterFunc(61*time.Second, cancel).Stop()

				obSender := func(ctx context.Context, obs []Outbox) error {
					outboxes = append(outboxes, obs...)
					if len(outboxes) >= len(contents) {
						cancel()
					}
					return nil
				}

				outboxesWG.Add(3)
				for i := 0; i < 3; i++ {
					go func() {
						defer outboxesWG.Done()

						p := tNewManagerPostgres(t, ManagerPostgresWithSender(obSender))
						WatchOutboxesLoop(ctx, p, nil)
					}()
				}

			}

			// store data
			{
				p := tGetManagerPostgres(t)
				sqlStatement := `TRUNCATE TABLE outbox RESTART IDENTITY CASCADE`
				_, err := p.db.Exec(sqlStatement)
				require.NoError(t, err)

				for i := 0; i < cap(outboxes); i++ {
					id := uuid.New().String()
					content := map[string]interface{}{"id": id, "test": uuid.New().String()}
					contents[id] = content
					outbox, err := NewOutbox(uuid.New(), event, ctype, content)
					require.NoError(t, err)

					func() {
						tx, err := p.db.Begin()
						require.NoError(t, err)
						defer tx.Commit()

						if isEncrypted {
							err = p.StoreOutboxEncrypted(ctx, tx, outbox)
						} else {
							err = p.StoreOutbox(ctx, tx, outbox)
						}
						require.NoError(t, err)
					}()
				}
			}

			// check sent outboxes
			{
				outboxesWG.Wait()
				for _, o := range outboxes {
					assert.Equal(t, ctype, o.ContentType)
					assert.Equal(t, event, o.Event)
					assert.Equal(t, isEncrypted, o.IsEncrypted)

					o, err := o.AsUnEncrypted()
					assert.NoError(t, err, "should return unencrypted outbox")

					pr := map[string]interface{}{}
					assert.NoError(t, json.Unmarshal(o.ContentByte(), &pr), "should return valid json")

					c, ok := contents[pr["id"].(string)]
					t.Log("send:", pr["id"].(string))
					require.True(t, ok, "should contains expected content")
					assert.Equal(t, c, pr)
				}
				assert.Len(t, outboxes, cap(outboxes), "should send all outbox")
			}
		})
	}

}
