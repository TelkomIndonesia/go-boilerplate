package sqlval

import (
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func TestAEAD(t *testing.T) {
	data := []any{
		"test",
		[]byte("test"),
		int64(100),
		float64(100.10),
		true,
		false,
		time.Date(2022, 01, 01, 23, 59, 0, 0, time.UTC),
	}

	template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
	require.NoError(t, err)
	h, err := keyset.NewHandle(template)
	require.NoError(t, err)
	m, err := crypt.NewDerivableKeyset(h, crypt.NewPrimitiveAEAD)
	require.NoError(t, err)
	ad := []byte(t.Name())

	newSQLVal := func(v any) (driver.Valuer, sql.Scanner, func() any) {
		switch v := v.(type) {
		case string:
			a, b := AEADString(m.GetPrimitiveFunc(nil), v, ad), AEADString(m.GetPrimitiveFunc(nil), "", ad)
			return a, &b, func() any { return b.To() }

		case []byte:
			a, b := AEADByteArray(m.GetPrimitiveFunc(nil), v, ad), AEADByteArray(m.GetPrimitiveFunc(nil), nil, ad)
			return a, &b, func() any { return b.To() }

		case int64:
			a, b := AEADInt64(m.GetPrimitiveFunc(nil), v, ad), AEADInt64(m.GetPrimitiveFunc(nil), 0, ad)
			return a, &b, func() any { return b.To() }

		case float64:
			a, b := AEADFloat64(m.GetPrimitiveFunc(nil), v, ad), AEADFloat64(m.GetPrimitiveFunc(nil), 0, ad)
			return a, &b, func() any { return b.To() }

		case bool:
			a, b := AEADBool(m.GetPrimitiveFunc(nil), v, ad), AEADBool(m.GetPrimitiveFunc(nil), !v, ad)
			return a, &b, func() any { return b.To() }

		case time.Time:
			a, b := AEADTime(m.GetPrimitiveFunc(nil), v, ad), AEADTime(m.GetPrimitiveFunc(nil), time.Now(), ad)
			return a, &b, func() any { return b.To() }

		default:
			t.Errorf("unknwon type :%v", v)
		}
		return nil, nil, nil
	}

	for _, d := range data {
		val, scan, scanned := newSQLVal(d)
		dv, err := val.Value()
		require.NoError(t, err, "should succesfully produce drive.Value")
		_, ok := dv.([]byte)
		assert.True(t, ok, "should be a byte array")
		assert.NotEqual(t, d, dv, "should not equal plain value")
		require.NoError(t, scan.Scan(dv), "should be able to be scanned")
		assert.Equal(t, d, scanned(), "should be equal to plain value")
	}
}

func TestAEADNilable(t *testing.T) {
	template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
	require.NoError(t, err)
	h, err := keyset.NewHandle(template)
	require.NoError(t, err)
	m, err := crypt.NewDerivableKeyset(h, crypt.NewPrimitiveAEAD)
	require.NoError(t, err)
	ad := []byte(t.Name())

	s := AEADString(m.GetPrimitiveFunc(nil), "", ad)
	err = s.Scan(nil)
	require.NoError(t, err, "should not return error")
	assert.Empty(t, s.To(), "should return nil pointer")
	assert.Nil(t, s.ToP(), "should return empty value")
}
