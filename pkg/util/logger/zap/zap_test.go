package zap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
)

func TestLog(t *testing.T) {
	l, err := New()
	require.NoError(t, err, "should create logger")
	msg := struct{ Hello string }{Hello: "world"}
	l.Info("test", logger.Any("hello", msg), logger.String("hi", "hello"))
}

func BenchmarkAppendNil(b *testing.B) {
	a := []string{"a", "b", "c"}
	var anil []string = nil
	var anotnil []string = []string{"d"}

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
