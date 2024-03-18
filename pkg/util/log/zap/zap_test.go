package zap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func TestLog(t *testing.T) {
	l, err := New()
	require.NoError(t, err, "should create logger")
	msg := struct{ Hello string }{Hello: "world"}
	l.Info("test", log.Any("hello", msg), log.String("hi", "hello"))
}
