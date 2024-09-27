package types

import (
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx/tsqlval"
)

// type aliases so that it can be used by sqlc
type (
	AEADString  = tsqlval.AEAD[string, tinkx.PrimitiveAEAD]
	AEADBool    = tsqlval.AEAD[bool, tinkx.PrimitiveAEAD]
	AEADFloat64 = tsqlval.AEAD[float64, tinkx.PrimitiveAEAD]
	AEADTime    = tsqlval.AEAD[time.Time, tinkx.PrimitiveAEAD]
	BIDXString  = tsqlval.BIDX[string, tinkx.PrimitiveBIDX]
)
