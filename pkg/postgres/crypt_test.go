package postgres

import (
	"bytes"
	"log"
	"testing"

	"github.com/google/uuid"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func TestMultiTenantKeyRotation(t *testing.T) {
	message := []byte("secret")
	adata := []byte(t.Name())
	tenantID, _ := uuid.NewV7()
	var chipertext, plaintext []byte

	template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
	if err != nil {
		log.Fatal(err)
	}
	mgr := keyset.NewManager()
	newHandle := func() *keyset.Handle {
		id, err := mgr.Add(template)
		if err != nil {
			t.Fatal("generate new key failed", err)
		}

		mgr.SetPrimary(id)
		t.Logf("new handle with id '%d' is set as primary", id)

		h, err := mgr.Handle()
		if err != nil {
			t.Fatal("get handle failed", err)
		}

		return h
	}

	t.Run("encrypt", func(t *testing.T) {
		handle := newHandle()
		m := multiTenantKeyset[primitiveAEAD]{
			master:      handle,
			constructur: newPrimitiveAEAD,
		}
		aead, err := m.GetPrimitive(tenantID)
		if err != nil {
			t.Fatal("should return aead primitive: %w", err)
		}
		chipertext, err = aead.Encrypt(message, adata)
		if err != nil {
			t.Fatal("should successfully encrypt plaintext: %w", err)
		}

	})

	t.Run("decrypt", func(t *testing.T) {
		handle := newHandle()
		m := multiTenantKeyset[primitiveAEAD]{
			master:      handle,
			constructur: newPrimitiveAEAD,
		}
		aead, err := m.GetPrimitive(tenantID)
		if err != nil {
			t.Fatal("should return aead primitive: %w", err)
		}
		plaintext, err = aead.Decrypt(chipertext, adata)
		if err != nil {
			t.Fatal("should successfully decrypt chiper: %w", err)
		}

	})

	if !bytes.Equal(message, plaintext) {
		t.Errorf("decrypted message (%s) should be equal original message (%s)", message, chipertext)
	}
}
