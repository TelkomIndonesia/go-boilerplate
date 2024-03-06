package postgres

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func getPostgres(t *testing.T) *Postgres {
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
	return p
}

func TestInstantiatePostgres(t *testing.T) {
	p := getPostgres(t)
	assert.NotNil(t, p, "should return non-nill struct")
}
