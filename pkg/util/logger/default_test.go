package logger

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLog(t *testing.T) {
	b := bytes.NewBuffer(nil)
	l := deflogger{
		w: b,
	}

	l.Debug("hello", Any("one", "two"), Any("three", "four"))

	msg := defMessage{}
	require.NoError(t, json.NewDecoder(b).Decode(&msg))
	assert.Equal(t, msg.Level, "DEBUG")
	assert.Equal(t, msg.Message, "hello")
	assert.Equal(t, msg.Fields["one"], "two")
	assert.Equal(t, msg.Fields["three"], "four")
}
