package tinkx

import (
	"os"
	"path/filepath"
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
		m := DerivableKeyset[PrimitiveAEAD]{
			master:      rotateKey(),
			constructur: NewPrimitiveAEAD,
		}
		aead, err := m.GetPrimitive(salt)
		require.NoError(t, err, "should return aead primitive")

		chipertext, err = aead.Encrypt(message, adata)
		require.NoError(t, err, "should encrypt original message")
	})

	t.Run("decrypt", func(t *testing.T) {
		m := DerivableKeyset[PrimitiveAEAD]{
			master:      rotateKey(),
			constructur: NewPrimitiveAEAD,
		}
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
		d, err := NewInsecureCleartextDerivableKeyset(k, NewPrimitiveAEAD)
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
		d, err := NewInsecureCleartextDerivableKeyset(k, NewPrimitiveMAC)
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
		d, err := NewInsecureCleartextDerivableKeyset(k, NewPrimitiveBIDX)
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
