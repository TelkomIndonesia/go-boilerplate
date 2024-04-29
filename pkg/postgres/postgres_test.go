package postgres

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
)

var testPostgres *Postgres
var testAEAD *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
var testBIDX *crypt.DerivableKeyset[crypt.PrimitiveBIDX]
var testPostgresSync, testKeysetHandleSync sync.Mutex

func tGetPostgres(t *testing.T) *Postgres {
	if testPostgres == nil {
		testPostgresSync.Lock()
		defer testPostgresSync.Unlock()
	}

	if testPostgres == nil {
		testPostgres = tNewPostgres(t)
	}

	return testPostgres
}

func tNewPostgres(t *testing.T, opts ...OptFunc) *Postgres {
	url, ok := os.LookupEnv("TEST_POSTGRES_URL")
	if !ok {
		t.Skip("no postgres url found")
	}

	p, err := New(append(opts,
		WithConnString(url),
		WithDerivableKeysets(tGetKeysetHandle(t)))...,
	)
	require.NoError(t, err, "should create postgres")
	return p
}

func tGetKeysetHandle(t *testing.T) (aeadh *crypt.DerivableKeyset[crypt.PrimitiveAEAD], mach *crypt.DerivableKeyset[crypt.PrimitiveBIDX]) {
	if testAEAD == nil || testBIDX == nil {
		testKeysetHandleSync.Lock()
		defer testKeysetHandleSync.Unlock()
	}

	var err error
	if testAEAD == nil {
		testAEAD, err = crypt.NewInsecureCleartextDerivableKeyset("./testdata/tink-aead.json", crypt.NewPrimitiveAEAD)
		require.NoError(t, err, "should create aead derivable keyset")
	}

	if testBIDX == nil {
		testBIDX, err = crypt.NewInsecureCleartextDerivableKeyset("./testdata/tink-mac.json", crypt.NewPrimitiveBIDXWithLen(16))
		require.NoError(t, err, "should create mac derivable keyset")
	}

	return testAEAD, testBIDX
}

func TestInstantiatePostgres(t *testing.T) {
	p := tGetPostgres(t)
	assert.NotNil(t, p, "should return non-nill struct")
}

func TestOutboxLock(t *testing.T) {
	ctx := context.Background()
	p := tGetPostgres(t)
	var scan bool

	conn, err := p.db.Conn(ctx)
	require.NoError(t, err, "should not return error")
	defer conn.Close()
	err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, outboxLock).Scan(&scan)
	require.NoError(t, err, "should not return error")
	assert.True(t, scan, "should obtain lock")

	conn1, err := p.db.Conn(ctx)
	require.NoError(t, err, "should not return error")
	defer conn1.Close()
	err = conn1.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, outboxLock).Scan(&scan)
	require.NoError(t, err, "should not return error")
	assert.False(t, scan, "should not obtain lock")

	conn.Close()
	conn2, err := p.db.Conn(ctx)
	require.NoError(t, err, "should not return error")
	defer conn2.Close()
	err = conn2.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, outboxLock).Scan(&scan)
	require.NoError(t, err, "should not return error")
	assert.True(t, scan, "should obtain lock after the first one is closed")
}
