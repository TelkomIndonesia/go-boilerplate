package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

var testPostgres *Postgres
var testPostgresSync sync.Mutex

func getPostgres(t *testing.T) *Postgres {
	if testPostgres == nil {
		testPostgresSync.Lock()
		defer testPostgresSync.Unlock()
	}

	if testPostgres == nil {
		url, ok := os.LookupEnv("POSTGRES_URL")
		if !ok {
			t.Skip("no postgres url found")
		}

		aeadT, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
		require.NoError(t, err, "should create prf based key template")
		aead, err := keyset.NewHandle(aeadT)
		require.NoError(t, err, "should create aead handle")

		macT, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag128KeyTemplate())
		require.NoError(t, err, "should create prf based key template")
		mac, err := keyset.NewHandle(macT)
		require.NoError(t, err, "should create mac handle")

		p, err := New(
			WithConnString(url),
			WithKeysets(aead, mac),
		)
		require.NoError(t, err, "should create postgres")
		testPostgres = p
	}

	return testPostgres
}

func TestInstantiatePostgres(t *testing.T) {
	p := getPostgres(t)
	assert.NotNil(t, p, "should return non-nill struct")
}

func TestLock(t *testing.T) {
	p := getPostgres(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("index-%d", i), func(t *testing.T) {
			conn, err := p.db.Conn(context.Background())
			require.NoError(t, err)
			defer conn.Close()

			conn2, err := p.db.Conn(context.Background())
			require.NoError(t, err)
			defer conn2.Close()

			wg := sync.WaitGroup{}
			wg.Add(2)
			res := []bool{false, false}
			go func() {
				defer wg.Done()
				r := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(1)`)
				require.NoError(t, r.Scan(&res[0]))
			}()
			go func() {
				defer wg.Done()
				r := conn2.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(1)`)
				require.NoError(t, r.Scan(&res[1]))
			}()
			wg.Wait()

			count := 0
			for _, r := range res {
				if r {
					count++
				}
			}
			t.Log(res)
			assert.Equal(t, 1, count)
		})
	}
}
