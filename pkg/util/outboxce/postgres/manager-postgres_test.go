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

	protobufce "github.com/cloudevents/sdk-go/binding/format/protobuf/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce/internal/sample"
	"google.golang.org/protobuf/proto"
)

var testPostgres *postgres
var testAEAD *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
var testPostgresSync, testKeysetHandleSync sync.Mutex

func tGetManagerPostgresTruncated(t *testing.T) *postgres {
	p := tGetManagerPostgres(t)

	sqlStatement := `TRUNCATE TABLE outboxce RESTART IDENTITY CASCADE`
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

	p, err := New(append(opts, WithDB(db, url))...)
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

			eventSource := "data/" + uuid.NewString()
			eventType := "data.incoming"
			contents := map[string]*sample.Outbox{}
			for i := 0; i < 30+rand.Int()%10; i++ {
				id := uuid.New().String()
				contents[id] = &sample.Outbox{Content: &sample.Outbox_Profile{
					Profile: &sample.Profile{
						ID:   id,
						Name: "name" + id,
					},
				}}
			}

			sentEvents := []event.Event{}

			// start replicas of manager that should wait for outboxes
			outboxesWG := sync.WaitGroup{}
			{
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				i := -1
				sender := func(ctx context.Context, obs []event.Event) error {
					if i = i + 1; i%2 == 0 {
						return fmt.Errorf("simulated intermittent error")
					}

					sentEvents = append(sentEvents, obs...)
					if len(sentEvents) >= len(contents) {
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

						outboxce.ObserveWithRetry(ctx, p, sender, nil)
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

						outbox := outboxce.New(eventSource, eventType, uuid.New(), content)
						if isEncrypted {
							outbox = outbox.WithEncryptor(outboxce.TenantAEAD(tGetKeysetHandle(t)))
						}
						err = manager.Store(ctx, tx, outbox)
						require.NoError(t, err, "should store outbox")
					}()
				}
				wg.Wait()
			}

			// check sent outboxes
			{
				outboxesWG.Wait()
				assert.Len(t, sentEvents, len(contents), "should send all new outbox")
				for _, e := range sentEvents {
					assert.Equal(t, eventSource, e.Context.GetSource(), "should contain valid content type")
					assert.Equal(t, eventType, e.Context.GetType(), "should contain valid event name")
					if isEncrypted {
						assert.Equal(t, outboxce.ContentTypeProtobufEncrypted, e.Context.GetDataContentType())
					} else {
						assert.Equal(t, protobufce.ContentTypeProtobuf, e.Context.GetDataContentType())
					}

					var ob sample.Outbox
					_, err := outboxce.FromEvent(e, outboxce.TenantAEAD(tGetKeysetHandle(t)), func(b []byte) (m proto.Message, err error) {
						err = proto.Unmarshal(b, &ob)
						return &ob, err
					})
					require.NoError(t, err)

					c, ok := contents[ob.GetProfile().ID]
					assert.True(t, ok, "should contains expected content")
					assert.Equal(t, sample.Profile{
						ID:       c.GetProfile().ID,
						TenantID: c.GetProfile().TenantID,
						Name:     c.GetProfile().Name,
						NIN:      c.GetProfile().NIN,
						Email:    c.GetProfile().Email,
						Phone:    c.GetProfile().Phone,
						DOB:      c.GetProfile().DOB,
					}, sample.Profile{
						ID:       ob.GetProfile().ID,
						TenantID: ob.GetProfile().TenantID,
						Name:     ob.GetProfile().Name,
						NIN:      ob.GetProfile().NIN,
						Email:    ob.GetProfile().Email,
						Phone:    ob.GetProfile().Phone,
						DOB:      ob.GetProfile().DOB,
					}, "should contains valid content")
				}
			}
		})
	}

}
