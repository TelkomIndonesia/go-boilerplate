package zap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
)

func TestLog(t *testing.T) {
	l, err := New()
	require.NoError(t, err, "should create logger")
	msg := struct{ Hello string }{Hello: "world"}
	l.Info("test", logger.Any("hello", msg), logger.String("hi", "hello"))
}
