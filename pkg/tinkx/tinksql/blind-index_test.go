package tinksql

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func TestBlindIndex(t *testing.T) {
	data := []any{
		"test",
		[]byte("test"),
		int64(100),
		float64(100.10),
		true,
		false,
		time.Date(2022, 01, 01, 23, 59, 0, 0, time.UTC),
	}

	template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag256KeyTemplate())
	require.NoError(t, err)
	mgr := keyset.NewManager()
	for i := 0; i < 3; i++ {
		id, err := mgr.Add(template)
		require.NoError(t, err)
		mgr.SetPrimary(id)
	}
	h, err := mgr.Handle()
	require.NoError(t, err)
	m, err := tinkx.NewDerivableKeyset(h, tinkx.NewPrimitiveBIDXWithLen(16))
	require.NoError(t, err)

	rwrap := func(b [][]byte) driver.Valuer { return NewArrayValuer(b) }
	newSQLVal := func(v any) (driver.Valuer, driver.Valuer) {
		switch v := v.(type) {
		case string:
			return BIDXString(m.GetPrimitiveFunc(nil), v),
				BIDXString(m.GetPrimitiveFunc(nil), v).ForRead(rwrap)

		case []byte:
			return BIDXByteArray(m.GetPrimitiveFunc(nil), v),
				BIDXByteArray(m.GetPrimitiveFunc(nil), v).ForRead(rwrap)

		case int64:
			return BIDXInt64(m.GetPrimitiveFunc(nil), v),
				BIDXInt64(m.GetPrimitiveFunc(nil), v).ForRead(rwrap)

		case float64:
			return BIDXFloat64(m.GetPrimitiveFunc(nil), v),
				BIDXFloat64(m.GetPrimitiveFunc(nil), v).ForRead(rwrap)

		case bool:
			return BIDXBool(m.GetPrimitiveFunc(nil), v),
				BIDXBool(m.GetPrimitiveFunc(nil), v).ForRead(rwrap)

		case time.Time:
			return BIDXTime(m.GetPrimitiveFunc(nil), v),
				BIDXTime(m.GetPrimitiveFunc(nil), v).ForRead(rwrap)

		default:
			t.Errorf("unknwon type :%v", v)
		}
		return nil, nil
	}

	for _, d := range data {
		w, r := newSQLVal(d)
		dv, err := w.Value()
		require.NoError(t, err, "should successfully produce driver.Value")
		dvs, err := r.Value()
		require.NoError(t, err, "should successfully produce driver.Value")
		assert.Len(t, dvs, 3, "should contains indices as much as key")
		assert.Contains(t, dvs, dv, "index for write should be included in indices for read")
		for _, v := range dvs.([][]byte) {
			assert.Len(t, v, 16, "should be truncated")
		}
	}
}
