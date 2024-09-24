package types

import (
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx/sqlval"
)

// type aliases so that it can be used by sqlc
type (
	AEADString  = sqlval.AEAD[string, tinkx.PrimitiveAEAD]
	AEADBool    = sqlval.AEAD[bool, tinkx.PrimitiveAEAD]
	AEADFloat64 = sqlval.AEAD[float64, tinkx.PrimitiveAEAD]
	AEADTime    = sqlval.AEAD[time.Time, tinkx.PrimitiveAEAD]
	BIDXString  = sqlval.BIDX[string, tinkx.PrimitiveBIDX]
)
