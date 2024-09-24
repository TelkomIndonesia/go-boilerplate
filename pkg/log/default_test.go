package log

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

func BenchmarkAppendNil(b *testing.B) {
	a := []string{"a", "b", "c"}
	var anil []string = nil
	var anotnil []string = []string{"d", "e", "f"}

	f := func(s ...string) {}

	b.Run("no append", func(b *testing.B) {
		f(a...)
	})
	b.Run("append nil", func(b *testing.B) {
		f(append(a, anil...)...)
	})
	b.Run("append not nil", func(b *testing.B) {
		f(append(a, anotnil...)...)
	})
}
