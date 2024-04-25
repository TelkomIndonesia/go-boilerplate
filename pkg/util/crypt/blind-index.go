package crypt

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/tink"
)

type BIDX interface {
	ComputePrimary(data []byte) (idx []byte, err error)
	ComputeAll(data []byte) (idxs [][]byte, err error)
}

var _ BIDX = bidx{}

type bidx struct {
	h *keyset.Handle
	m tink.MAC

	hs []*keyset.Handle
	ms []tink.MAC

	len int
}

func NewBIDX(h *keyset.Handle, length int) (BIDX, error) {
	m, err := mac.New(h)
	if err != nil {
		return nil, fmt.Errorf("fail to instantiate underlying mac primitive:%w", err)
	}
	b := bidx{
		h:   h,
		m:   m,
		hs:  make([]*keyset.Handle, 0, len(h.KeysetInfo().GetKeyInfo())),
		ms:  make([]tink.MAC, 0, len(h.KeysetInfo().GetKeyInfo())),
		len: length,
	}

	for _, k := range h.KeysetInfo().GetKeyInfo() {
		if k.GetKeyId() == h.KeysetInfo().GetPrimaryKeyId() {
			b.hs = append(b.hs, b.h)
			b.ms = append(b.ms, b.m)
			continue
		}

		h, err := cloneHandle(h)
		if err != nil {
			return nil, fmt.Errorf("fail to clone handler :%w", err)
		}

		keyset.NewManagerFromHandle(h).SetPrimary(k.GetKeyId())
		m, err := mac.New(h)
		if err != nil {
			return nil, fmt.Errorf("fail to instantiate primitive from key id %d: %w", k.GetKeyId(), err)
		}

		b.hs = append(b.hs, h)
		b.ms = append(b.ms, m)
	}
	return b, nil
}

func CopyBIDXWithLen(t BIDX, len int) (BIDX, error) {
	b, ok := t.(bidx)
	if !ok {
		pb, ok := t.(PrimitiveBIDX)
		if !ok {
			return nil, fmt.Errorf("unknwon BIDX implementation")
		}
		b, ok = pb.BIDX.(bidx)
		if !ok {
			return nil, fmt.Errorf("unknwon BIDX implementation")
		}
	}
	b.len = len
	return b, nil
}

func (b bidx) ComputePrimary(data []byte) (idx []byte, err error) {
	idx, err = b.m.ComputeMAC(data)
	if err != nil {
		return nil, fmt.Errorf("fail to compute blind index: %w", err)
	}
	if b.len != 0 && len(idx) > b.len {
		idx = idx[:b.len]
	}
	return
}

func (b bidx) ComputeAll(data []byte) (idxs [][]byte, err error) {
	idxs = make([][]byte, 0, len(b.hs))
	for i, m := range b.ms {
		idx, err := bidx{m: m, h: b.h, len: b.len}.ComputePrimary(data)
		if err != nil {
			return nil, fmt.Errorf("fail to compute mac from key id %d: %w", b.hs[i].KeysetInfo().GetPrimaryKeyId(), err)
		}
		idxs = append(idxs, idx)
	}
	return
}

var bufpool sync.Pool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func cloneHandle(h *keyset.Handle) (hc *keyset.Handle, err error) {
	b := bufpool.Get().(*bytes.Buffer)
	defer func() { b.Reset(); bufpool.Put(b) }()

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

// GetBlindIdxs
// Deprecated: use BlindIdx.ComputeAll() instead.
func GetBlindIdxs(h *keyset.Handle, key []byte, len int) (idxs [][]byte, err error) {
	b, err := NewBIDX(h, len)
	if err != nil {
		return nil, err
	}

	return b.ComputeAll(key)
}
