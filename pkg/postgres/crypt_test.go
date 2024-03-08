package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func TestMultiTenantKeyRotation(t *testing.T) {
	message, adata := []byte("secret"), []byte(t.Name())
	tenantID, _ := uuid.NewV7()
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
		m := multiTenantKeyset[primitiveAEAD]{
			master:      rotateKey(),
			constructur: newPrimitiveAEAD,
		}
		aead, err := m.GetPrimitive(tenantID)
		require.NoError(t, err, "should return aead primitive")

		chipertext, err = aead.Encrypt(message, adata)
		require.NoError(t, err, "should encrypt original message")
	})

	t.Run("decrypt", func(t *testing.T) {
		m := multiTenantKeyset[primitiveAEAD]{
			master:      rotateKey(),
			constructur: newPrimitiveAEAD,
		}
		aead, err := m.GetPrimitive(tenantID)
		require.NoError(t, err, "should return aead primitive")

		plaintext, err = aead.Decrypt(chipertext, adata)
		require.NoError(t, err, "should decrypt chipertext")

	})

	assert.Equal(t, message, plaintext, "decrypted message should be equal to original message")
}

func TestBlindIndexes(t *testing.T) {
	mgr := keyset.NewManager()
	hid, err := mgr.Add(mac.HMACSHA256Tag256KeyTemplate())
	require.NoError(t, err, "should add mac handle")
	err = mgr.SetPrimary(hid)
	require.NoError(t, err, "should set primary handle")
	handle, err := mgr.Handle()
	require.NoError(t, err, "should obtain mac handle")
	m, err := mac.New(handle)
	require.NoError(t, err, "should create mac primitive")

	data := []byte("asdasjdiu9lksdlfkjasopfijaposdpasi09ie283u023hj02i0t83089tu045jt054050j")
	v, err := m.ComputeMAC(data[:])
	require.NoError(t, err, "should compute mac")

	hid, err = mgr.Add(mac.HMACSHA256Tag128KeyTemplate())
	require.NoError(t, err, "should add new mac handle")
	err = mgr.SetPrimary(hid)
	require.NoError(t, err, "should set new primary handle")
	handle, err = mgr.Handle()
	require.NoError(t, err, "should obtain new mac handle")

	vs, err := getBlindIdxs(handle, data[:], len(v))
	require.NoError(t, err, "should compute multiple mac")

	assert.Contains(t, vs, v, "should contain previous mac")
}
