package logtest

import (
	"testing"
)

func TestLogger(t *testing.T) {
	l := NewLogger(t)
	l.Info(t.Context(), "test")
	l.Info(t.Context(), "test2")
	t.Log("test")
}
