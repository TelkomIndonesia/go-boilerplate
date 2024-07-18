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
var testPostgresSync, testKeysetHandleSync sync.Mutex

func tGetPostgresTruncated(t *testing.T) *Postgres {
	p := tGetPostgres(t)

	sqlStatement := `
		DO
		$$
		DECLARE
			r RECORD;
		BEGIN
			-- Disable triggers temporarily if you have foreign key constraints
			EXECUTE 'SET session_replication_role = replica';

			-- Loop through all tables
			FOR r IN (SELECT tablename FROM pg_tables WHERE tableowner = (SELECT CURRENT_USER) AND schemaname != 'pg_catalog' AND schemaname != 'information_schema') LOOP
				EXECUTE 'TRUNCATE TABLE ' || quote_ident(r.tablename) || ' RESTART IDENTITY CASCADE';
			END LOOP;

			-- Re-enable triggers
			EXECUTE 'SET session_replication_role = DEFAULT';
		END
		$$;
    `
	_, err := p.db.Exec(sqlStatement)
	require.NoError(t, err)
	return p
}

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
	p := tGetPostgresTruncated(t)
	assert.NotNil(t, p, "should return non-nill struct")
}
