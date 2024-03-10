package tlswrapper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDialer(t *testing.T) {
	pool, err := x509.SystemCertPool()
	require.NoError(t, err, "should load system cert pool")
	c := &tls.Config{RootCAs: pool}
	d := dialer{c: &wrapper{cfgc: c}, d: &net.Dialer{}}

	// https://pkg.go.dev/crypto/tls#example-Dial
	conn, err := d.DialContext(context.Background(), "tcp", "mail.google.com:443")
	require.NoError(t, err, "should successfully establish connection")
	defer conn.Close()
}

func BenchmarkDialer(b *testing.B) {
	pool, err := x509.SystemCertPool()
	require.NoError(b, err, "should load system cert pool")

	c := &tls.Config{RootCAs: pool}
	d := dialer{c: &wrapper{cfgc: c}, d: &net.Dialer{}}

	b.Run("dialer", func(b *testing.B) {
		d.DialContext(context.Background(), "tcp", "mail.google.com:443")
	})
	b.Run("tls-dialer", func(b *testing.B) {
		tls.DialWithDialer(d.d, "tcp", "mail.google.com:443", c)
	})
}
