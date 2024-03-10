package tlswrapper

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDialer(t *testing.T) {
	c := &tls.Config{}
	d := dialer{c: &wrapper{cfgc: c}, d: &net.Dialer{}}

	// https://pkg.go.dev/crypto/tls#example-Dial
	conn, err := d.DialContext(context.Background(), "tcp", "mail.google.com:443")
	require.NoError(t, err, "should successfully establish connection")
	defer conn.Close()
}

func BenchmarkDialer(b *testing.B) {
	c := &tls.Config{}
	d := dialer{c: &wrapper{cfgc: c}, d: &net.Dialer{}}

	b.Run("dialer", func(b *testing.B) {
		d.DialContext(context.Background(), "tcp", "mail.google.com:443")
	})
	b.Run("tls-dialer", func(b *testing.B) {
		tls.DialWithDialer(d.d, "tcp", "mail.google.com:443", c)
	})
}
