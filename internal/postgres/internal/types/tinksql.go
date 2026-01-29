package types

import (
	"time"

	"github.com/telkomindonesia/go-boilerplate/internal/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx/tinksql"
)

// type aliases so that it can be used by sqlc
type (
	AEADString  = tinksql.AEAD[string, tinkx.PrimitiveAEAD]
	AEADTime    = tinksql.AEAD[time.Time, tinkx.PrimitiveAEAD]
	BIDXString  = tinksql.BIDX[string, tinkx.PrimitiveBIDX]
	AEADProfile = tinksql.AEAD[profile.Profile, tinkx.PrimitiveAEAD] // use tinksql.AEADMsgpack to instantiate
)
