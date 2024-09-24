package types

import (
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt/sqlval"
)

// type aliases so that it can be used by sqlc
type (
	AEADString  = sqlval.AEAD[string, crypt.PrimitiveAEAD]
	AEADBool    = sqlval.AEAD[bool, crypt.PrimitiveAEAD]
	AEADFloat64 = sqlval.AEAD[float64, crypt.PrimitiveAEAD]
	AEADTime    = sqlval.AEAD[time.Time, crypt.PrimitiveAEAD]
	BIDXString  = sqlval.BIDX[string, crypt.PrimitiveBIDX]
)
