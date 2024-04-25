package postgres

import (
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
var testBIDXU *crypt.DerivableKeyset[crypt.PrimitiveBIDX]
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

func tGetKeysetHandle(t *testing.T) (aeadh *crypt.DerivableKeyset[crypt.PrimitiveAEAD], mach *crypt.DerivableKeyset[crypt.PrimitiveBIDX], machU *crypt.DerivableKeyset[crypt.PrimitiveBIDX]) {
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
	if testBIDXU == nil {
		testBIDXU, err = crypt.NewInsecureCleartextDerivableKeyset("./testdata/tink-mac.json", crypt.NewPrimitiveBIDX)
		require.NoError(t, err, "should create mac derivable keyset")
	}

	return testAEAD, testBIDX, testBIDXU
}

func TestInstantiatePostgres(t *testing.T) {
	p := tGetPostgres(t)
	assert.NotNil(t, p, "should return non-nill struct")
}
