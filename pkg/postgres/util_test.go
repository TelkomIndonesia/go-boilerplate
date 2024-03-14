package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func tRequireUUIDV7(t *testing.T) (u uuid.UUID) {
	u, err := uuid.NewV7()
	require.NoError(t, err, "should create uuid")
	return
}
