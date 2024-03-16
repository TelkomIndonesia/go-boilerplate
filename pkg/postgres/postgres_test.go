package postgres

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

var testPostgres *Postgres
var testAEAD *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
var testMAC *crypt.DerivableKeyset[crypt.PrimitiveMAC]
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

func tGetKeysetHandle(t *testing.T) (aeadh *crypt.DerivableKeyset[crypt.PrimitiveAEAD], mach *crypt.DerivableKeyset[crypt.PrimitiveMAC]) {
	if testAEAD == nil || testMAC == nil {
		testKeysetHandleSync.Lock()
		defer testKeysetHandleSync.Unlock()
	}

	if testAEAD == nil {
		aeadT, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
		require.NoError(t, err, "should create prf based key template")
		h, err := keyset.NewHandle(aeadT)
		require.NoError(t, err, "should create aead handle")
		testAEAD, err = crypt.NewDerivableKeyset(h, crypt.NewPrimitiveAEAD)
		require.NoError(t, err, "should create mac derivable keyset")
	}

	if testMAC == nil {
		macT, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag128KeyTemplate())
		require.NoError(t, err, "should create prf based key template")
		h, err := keyset.NewHandle(macT)
		require.NoError(t, err, "should create mac handle")
		testMAC, err = crypt.NewDerivableKeyset(h, crypt.NewPrimitiveMAC)
		require.NoError(t, err, "should create mac derivable keyset")
	}

	return testAEAD, testMAC
}

func TestInstantiatePostgres(t *testing.T) {
	p := tGetPostgres(t)
	assert.NotNil(t, p, "should return non-nill struct")
}
