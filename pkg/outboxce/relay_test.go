package outboxce

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayError(t *testing.T) {
	var e error = &RelayError{Err: fmt.Errorf("test")}

	var errRelay *RelayError = &RelayError{}
	var errRelays *RelayErrors = &RelayErrors{}
	assert.True(t, errors.As(e, &errRelay))
	assert.True(t, errors.As(e, &errRelays))
	require.Len(t, *errRelays, 1)
	assert.Equal(t, errRelay, (*errRelays)[0])
}
