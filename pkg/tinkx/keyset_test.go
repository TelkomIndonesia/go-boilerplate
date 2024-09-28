package tinkx

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func TestDerivedKeyRotation(t *testing.T) {
	message, adata := []byte("secret"), []byte(t.Name())
	u, _ := uuid.NewV7()
	salt := u[:]
	var chipertext, plaintext []byte

	template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
	require.NoError(t, err, "should create prf based key template")
	mgr := keyset.NewManager()
	rotateKey := func() *keyset.Handle {
		id, err := mgr.Add(template)
		require.NoError(t, err, "should add template")

		mgr.SetPrimary(id)
		t.Logf("new handle with id '%d' is set as primary", id)

		h, err := mgr.Handle()
		require.NoError(t, err, "should return handle")

		return h
	}

	t.Run("encrypt", func(t *testing.T) {
		m, err := NewDerivableKeyset(rotateKey(), NewPrimitiveAEAD, DerivableKeysetWithCapCache[PrimitiveAEAD](100))
		require.NoError(t, err)
		aead, err := m.GetPrimitive(salt)
		require.NoError(t, err, "should return aead primitive")

		chipertext, err = aead.Encrypt(message, adata)
		require.NoError(t, err, "should encrypt original message")
	})

	t.Run("decrypt", func(t *testing.T) {
		m, err := NewDerivableKeyset(rotateKey(), NewPrimitiveAEAD, DerivableKeysetWithCapCache[PrimitiveAEAD](100))
		require.NoError(t, err)
		aead, err := m.GetPrimitive(salt)
		require.NoError(t, err, "should return aead primitive")

		plaintext, err = aead.Decrypt(chipertext, adata)
		require.NoError(t, err, "should decrypt chipertext")

	})

	assert.Equal(t, message, plaintext, "decrypted message should be equal to original message")
}

func TestLoadKeysFromFile(t *testing.T) {

	t.Run("AEAD", func(t *testing.T) {
		tmpl, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
		require.NoError(t, err, "should create template")
		h, err := keyset.NewHandle(tmpl)
		require.NoError(t, err, "should create handle")
		k := filepath.Join(t.TempDir(), "key.json")
		f, err := os.Create(k)
		require.NoError(t, err, "should open file for write")
		insecurecleartextkeyset.Write(h, keyset.NewJSONWriter(f))
		d, err := NewInsecureCleartextDerivableKeyset(k, NewPrimitiveAEAD, DerivableKeysetWithCapCache[PrimitiveAEAD](100))
		require.NoError(t, err, "should read key")
		p1, err := d.GetPrimitive(nil)
		require.NoError(t, err, "should get primitive")
		p2, err := d.GetPrimitive(nil)
		require.NoError(t, err, "should get primitive")
		assert.Equal(t, p1, p2, "should return the same primitive")
		_, err = p1.Encrypt([]byte("test"), []byte("test"))
		require.NoError(t, err, "should encrypt")
	})

	t.Run("MAC", func(t *testing.T) {
		tmpl, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag256KeyTemplate())
		require.NoError(t, err, "should create template")
		h, err := keyset.NewHandle(tmpl)
		require.NoError(t, err, "should create handle")
		k := filepath.Join(t.TempDir(), "key.json")
		f, err := os.Create(k)
		require.NoError(t, err, "should open file for write")
		insecurecleartextkeyset.Write(h, keyset.NewJSONWriter(f))
		d, err := NewInsecureCleartextDerivableKeyset(k, NewPrimitiveMAC, DerivableKeysetWithCapCache[PrimitiveMAC](100))
		require.NoError(t, err, "should read key")
		m1, err := d.GetPrimitive(nil)
		require.NoError(t, err, "should get primitive")
		m2, err := d.GetPrimitive(nil)
		require.NoError(t, err, "should get primitive")
		assert.Equal(t, m1, m2, "should return the same primitive")
		_, err = m1.ComputeMAC([]byte("test"))
		require.NoError(t, err, "should hash")
	})

	t.Run("BIDX", func(t *testing.T) {
		tmpl, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag256KeyTemplate())
		require.NoError(t, err, "should create template")
		h, err := keyset.NewHandle(tmpl)
		require.NoError(t, err, "should create handle")
		k := filepath.Join(t.TempDir(), "key.json")
		f, err := os.Create(k)
		require.NoError(t, err, "should open file for write")
		insecurecleartextkeyset.Write(h, keyset.NewJSONWriter(f))
		d, err := NewInsecureCleartextDerivableKeyset(k, NewPrimitiveBIDX, DerivableKeysetWithCapCache[PrimitiveBIDX](100))
		require.NoError(t, err, "should read key")
		m1, err := d.GetPrimitive(nil)
		require.NoError(t, err, "should get primitive")
		m2, err := d.GetPrimitive(nil)
		require.NoError(t, err, "should get primitive")
		assert.Equal(t, m1, m2, "should return the same primitive")
		_, err = m1.ComputePrimary([]byte("test"))
		require.NoError(t, err, "should hash")
	})
}

type tCacheSyncMap[T any] struct{ m sync.Map }

// Get implements DerivationCache.
func (m *tCacheSyncMap[T]) Get(key []byte) (t T, ok bool) {
	v, ok := m.m.Load(string(key))
	if !ok {
		return
	}
	t, ok = v.(T)
	return
}

// Set implements DerivationCache.
func (m *tCacheSyncMap[T]) Set(key []byte, t T) (ok bool) {
	m.m.Store(string(key), t)
	return
}

func BenchmarkGetHandle(b *testing.B) {
	template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
	if err != nil {
		b.Fatal(err)
	}
	handle, err := keyset.NewHandle(template)
	if err != nil {
		b.Fatal(err)
	}

	n := 100
	b.Run("NoCache", func(b *testing.B) {
		dkeyset, err := NewDerivableKeyset(handle, NewPrimitiveAEAD)
		if err != nil {
			b.Fatal(err)
		}

		for i := 0; i < b.N; i++ {
			_, _, err := dkeyset.GetPrimitiveAndHandle([]byte(b.Name() + strconv.Itoa(i%n)))
			if err != nil {
				b.Error(err)
			}
		}
	})

	b.Run("Otter", func(b *testing.B) {
		dkeyset, err := NewDerivableKeyset(handle, NewPrimitiveAEAD, DerivableKeysetWithCapCache[PrimitiveAEAD](n))
		if err != nil {
			b.Fatal(err)
		}

		for i := 0; i < b.N; i++ {
			_, _, err := dkeyset.GetPrimitiveAndHandle([]byte(b.Name() + strconv.Itoa(i%n)))
			if err != nil {
				b.Error(err)
			}
		}
	})

	b.Run("SyncMap", func(b *testing.B) {
		dkeyset, err := NewDerivableKeyset(handle, NewPrimitiveAEAD)
		if err != nil {
			b.Fatal(err)
		}
		dkeyset.keys = &tCacheSyncMap[*keyset.Handle]{}
		dkeyset.primitives = &tCacheSyncMap[PrimitiveAEAD]{}

		for i := 0; i < b.N; i++ {
			_, _, err := dkeyset.GetPrimitiveAndHandle([]byte(b.Name() + strconv.Itoa(i%n)))
			if err != nil {
				b.Error(err)
			}
		}
	})
}
