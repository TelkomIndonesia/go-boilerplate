package log

import (
	"context"
	"log"
)

type loggerW struct {
	p string
	f int
	l Logger
}

func (w loggerW) Write(p []byte) (n int, err error) {
	w.l.Error(context.Background(), w.p, String("log", string(p)))
	return len(p), nil
}
func NewStdLogger(l Logger, prefix string, flags int) *log.Logger {
	return log.New(loggerW{p: prefix, f: flags, l: l}, "", 0)
}
