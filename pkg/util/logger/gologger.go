package logger

import "log"

type gologgerw struct {
	p string
	f int
	l Logger
}

func (g gologgerw) Write(p []byte) (n int, err error) {
	g.l.Error(g.p, ByteString("log", p))
	return len(p), nil
}

func NewGoLogger(l Logger, prefix string, flag int) *log.Logger {
	return log.New(gologgerw{
		l: l,
		f: flag,
		p: prefix,
	}, "", 0)
}
