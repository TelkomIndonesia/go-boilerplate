package types

import (
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/crypto"
	"github.com/telkomindonesia/go-boilerplate/pkg/crypto/sqlval"
)

// type aliases so that it can be used by sqlc
type (
	AEADString  = sqlval.AEAD[string, crypto.PrimitiveAEAD]
	AEADBool    = sqlval.AEAD[bool, crypto.PrimitiveAEAD]
	AEADFloat64 = sqlval.AEAD[float64, crypto.PrimitiveAEAD]
	AEADTime    = sqlval.AEAD[time.Time, crypto.PrimitiveAEAD]
	BIDXString  = sqlval.BIDX[string, crypto.PrimitiveBIDX]
)
