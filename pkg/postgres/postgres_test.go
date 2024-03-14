package postgres

import (
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
var testAEAD, testMAC *keyset.Handle
var testPostgresSync, testKeysetHandleSync sync.Mutex

func getPostgres(t *testing.T) *Postgres {
	if testPostgres == nil {
		testPostgresSync.Lock()
		defer testPostgresSync.Unlock()
	}

	if testPostgres == nil {
		testPostgres = newPostgres(t)
	}

	return testPostgres
}

func newPostgres(t *testing.T, opts ...OptFunc) *Postgres {
	url, ok := os.LookupEnv("POSTGRES_URL")
	if !ok {
		t.Skip("no postgres url found")
	}

	p, err := New(append(opts,
		WithConnString(url),
		WithKeysets(getKeysetHandle(t)))...,
	)
	require.NoError(t, err, "should create postgres")
	return p
}

func getKeysetHandle(t *testing.T) (aeadh *keyset.Handle, mach *keyset.Handle) {
	if testAEAD == nil || testMAC == nil {
		testKeysetHandleSync.Lock()
		defer testKeysetHandleSync.Unlock()
	}

	if testAEAD == nil {
		aeadT, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
		require.NoError(t, err, "should create prf based key template")
		testAEAD, err = keyset.NewHandle(aeadT)
		require.NoError(t, err, "should create aead handle")
	}

	if testMAC == nil {
		macT, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag128KeyTemplate())
		require.NoError(t, err, "should create prf based key template")
		testMAC, err = keyset.NewHandle(macT)
		require.NoError(t, err, "should create mac handle")
	}

	return testAEAD, testMAC
}

func TestInstantiatePostgres(t *testing.T) {
	p := getPostgres(t)
	assert.NotNil(t, p, "should return non-nill struct")
}
