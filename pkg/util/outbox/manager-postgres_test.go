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
	ctype := "data"
	event := "data_incoming"
	contents := map[string]map[string]interface{}{}
	outboxes := make([]map[string]interface{}, 0, 21)

	ctx := context.Background()
	outboxesWG := sync.WaitGroup{}
	outboxesWG.Add(3)
	{
		ctx, cancel := context.WithCancel(ctx)
		defer time.AfterFunc(30*time.Second, cancel).Stop()

		obSender := func(ctx context.Context, obs []Outbox) error {
			for _, o := range obs {
				assert.Equal(t, ctype, o.ContentType)
				assert.Equal(t, event, o.Event)

				o, err := o.AsUnEncrypted()
				assert.NoError(t, err, "should return correctly encrypted outbox")

				pr := map[string]interface{}{}
				assert.NoError(t, json.Unmarshal(o.ContentByte(), &pr), "should return valid json")

				_, ok := contents[pr["id"].(string)]
				assert.True(t, ok, "should contain expected outbox")

				outboxes = append(outboxes, pr)
			}
			if len(outboxes) == cap(outboxes) {
				cancel()
			}
			return nil
		}

		for i := 0; i < 3; i++ {
			p := tNewManagerPostgres(t, ManagerPostgresWithSender(obSender))
			go func() {
				defer outboxesWG.Done()
				<-ctx.Done()
			}()
			go WatchOutboxesLoop(ctx, p, nil)
		}

	}

	p := tGetManagerPostgres(t)
	sqlStatement := `TRUNCATE TABLE outbox RESTART IDENTITY CASCADE`
	_, err := p.db.Exec(sqlStatement)
	require.NoError(t, err)

	for i := 0; i < cap(outboxes); i++ {
		t.Run(fmt.Sprintf("store-%d", i), func(t *testing.T) {
			tx, err := p.db.Begin()
			require.NoError(t, err)
			defer tx.Commit()

			id := uuid.New().String()
			content := map[string]interface{}{"id": id}
			contents[id] = content
			ob, err := NewOutbox(uuid.New(), event, ctype, content)
			require.NoError(t, err)

			err = p.StoreOutboxEncrypted(ctx, tx, ob)
			require.NoError(t, err)
		})
	}

	outboxesWG.Wait()
	for _, pr := range outboxes {
		c, ok := contents[pr["id"].(string)]
		require.True(t, ok, "should return stored outbox")
		assert.Equal(t, c, pr)
	}
	assert.Len(t, outboxes, cap(outboxes), "should send all outbox")
}
