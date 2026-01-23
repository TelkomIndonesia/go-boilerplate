package tlswrap

import (
	"crypto/tls"
	"net"
)

type listener struct {
	tw *TLSWrap
	l  net.Listener
}

func (c listener) Accept() (net.Conn, error) {
	conn, err := c.l.Accept()
	if err != nil || c.tw.cert.Load() == nil {
		return conn, err
	}

	return tls.Server(conn, c.tw.cfgs.Load()), nil
}

func (c listener) Close() error {
	return c.l.Close()
}

func (c listener) Addr() net.Addr {
	return c.l.Addr()
}
