package tlswrap

import (
	"context"
	"crypto/tls"
	"net"
)

type Dialer interface {
	Dial(net string, addr string) (net.Conn, error)
	DialContext(ctx context.Context, net string, addr string) (net.Conn, error)
}

type dialer struct {
	tw *TLSWrap
	d  *net.Dialer
}

func (d dialer) Dial(net string, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), net, addr)
}

func (d dialer) DialContext(ctx context.Context, net string, addr string) (net.Conn, error) {
	return (&tls.Dialer{NetDialer: d.d, Config: d.tw.cfgc.Load()}).
		DialContext(ctx, net, addr)
}
