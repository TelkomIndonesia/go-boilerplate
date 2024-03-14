package crypt

import (
	"bytes"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
)

func GetBlindIdxs(h *keyset.Handle, key []byte, length int) (idxs [][]byte, err error) {
	h, err = cloneHandle(h)
	if err != nil {
		return nil, err
	}

	idxs = make([][]byte, 0, len(h.KeysetInfo().GetKeyInfo()))
	mgr := keyset.NewManagerFromHandle(h)
	for _, i := range h.KeysetInfo().GetKeyInfo() {
		mgr.SetPrimary(i.GetKeyId())
		m, err := mac.New(h)
		if err != nil {
			return nil, fmt.Errorf("fail to instantiate primitive from key id %d: %w", i.GetKeyId(), err)
		}

		b, err := m.ComputeMAC(key)
		if err != nil {
			return nil, fmt.Errorf("fail to compute mac from key id %d: %w", i.GetKeyId(), err)
		}
		idxs = append(idxs, b[:min(len(b), length)])
	}
	return
}

func cloneHandle(h *keyset.Handle) (hc *keyset.Handle, err error) {
	b := new(bytes.Buffer)

	w := keyset.NewBinaryWriter(b)
	err = insecurecleartextkeyset.Write(h, w)
	if err != nil {
		return nil, fmt.Errorf("fail to copy handle to memory: %w", err)
	}

	r := keyset.NewBinaryReader(b)
	hc, err = insecurecleartextkeyset.Read(r)
	if err != nil {
		return nil, fmt.Errorf("fail to copy handle from memory: %w", err)
	}

	return hc, nil
}
