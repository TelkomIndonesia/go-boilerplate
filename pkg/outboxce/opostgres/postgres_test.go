package opostgres

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
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logtest"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce/internal/sample"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"google.golang.org/protobuf/proto"
)

var testPostgres *postgres
var testAEAD *tinkx.DerivableKeyset[tinkx.PrimitiveAEAD]
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

	db, err := sql.Open("pgx", url)
	require.NoError(t, err)

	p, err := NewManager(append(opts, WithDB(db, url), WithLogger(logtest.NewLogger(t).WithAttrs(log.String("name", t.Name()))))...)
	require.NoError(t, err, "should create postgres")
	return p.(*postgres)
}

func tGetKeysetHandle(t *testing.T) (aeadh *tinkx.DerivableKeyset[tinkx.PrimitiveAEAD]) {
	if testAEAD == nil {
		testKeysetHandleSync.Lock()
		defer testKeysetHandleSync.Unlock()
	}

	var err error
	if testAEAD == nil {
		testAEAD, err = tinkx.NewInsecureCleartextDerivableKeyset("./testdata/tink-aead.json", tinkx.NewPrimitiveAEAD)
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

	for i, isEncrypted := range []bool{false, true, false, true} {
		t.Run(fmt.Sprintf("idx%dEncrypted%v", i, isEncrypted), func(t *testing.T) {
			ctx := context.Background()

			eventSource := "data/" + uuid.NewString()
			eventType := "data.incoming"
			outboxes := map[string]*sample.Outbox{}
			for i := 0; i < 30+rand.Int()%10; i++ {
				id := uuid.New().String()
				outboxes[id] = &sample.Outbox{Id: id, Messsage: "hello world"}
			}

			sentEvents := []event.Event{}

			// start replicas of manager that should wait for outboxes
			outboxesWG := sync.WaitGroup{}
			{
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				i := -1
				sender := func(ctx context.Context, obs []event.Event) (err error) {
					if i = i + 1; i%2 == 0 {
						return fmt.Errorf("simulated full error")
					}

					if j := len(obs) / 2; j > 0 {
						relayErrors := &outboxce.RelayErrors{}
						for _, e := range obs[j:] {
							*relayErrors = append(*relayErrors, &outboxce.RelayError{
								Err:   fmt.Errorf("simulated (partial) relay errors"),
								Event: e,
							})
						}
						err = relayErrors
						obs = obs[:j]
					}

					sentEvents = append(sentEvents, obs...)
					if len(sentEvents) >= len(outboxes) {
						time.AfterFunc(time.Second, cancel)
					}

					return
				}

				replica := 10
				outboxesWG.Add(replica)
				for i := 0; i < replica; i++ {
					go func() {
						defer outboxesWG.Done()

						p := tNewManagerPostgres(t)
						p.maxWaitNotif = 0
						p.maxRelaySize = 10
						defer p.db.Close()

						outboxce.RelayLoopWithRetry(ctx, p, sender, logtest.NewLogger(t).WithAttrs(log.Int64("replica-id", int64(i))))
					}()
				}
			}

			// store data
			{
				wg := sync.WaitGroup{}
				wg.Add(len(outboxes))
				for _, outbox := range outboxes {
					go func() {
						defer wg.Done()

						tx, err := manager.db.Begin()
						require.NoError(t, err)
						defer tx.Commit()

						outbox := outboxce.New(eventSource, eventType, outbox).WithTenantID(uuid.New())
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
				assert.Len(t, sentEvents, len(outboxes), "should send all new outbox")
				for _, e := range sentEvents {
					assert.Equal(t, eventSource, e.Context.GetSource(), "should contain valid content type")
					assert.Equal(t, eventType, e.Context.GetType(), "should contain valid event name")
					if isEncrypted {
						assert.Equal(t, outboxce.ContentTypeProtobufEncrypted, e.Context.GetDataContentType())
					} else {
						assert.Equal(t, protobufce.ContentTypeProtobuf, e.Context.GetDataContentType())
					}

					var oSent sample.Outbox
					_, err := outboxce.FromEvent(e, outboxce.TenantAEAD(tGetKeysetHandle(t)), func(b []byte) (m proto.Message, err error) {
						err = proto.Unmarshal(b, &oSent)
						return &oSent, err
					})
					require.NoError(t, err)

					o := outboxes[oSent.Id]
					require.NotNil(t, o)
					assert.Equal(t,
						sample.Outbox{
							Id:       o.Id,
							Messsage: o.Messsage,
						},
						sample.Outbox{
							Id:       oSent.Id,
							Messsage: oSent.Messsage,
						},
						"should contains valid content")
				}
			}
		})
	}
}
