package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/util"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log/zap"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/otel"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/tlswrapper"
)

type OptFunc func(*CMD) error

func WithEnv(prefix string, dotenv bool) OptFunc {
	return func(u *CMD) error {
		return util.LoadEnv(u, util.LoadEnvOptions{
			Prefix: prefix,
			DotEnv: dotenv,
		})
	}
}
func WithTLSConfig(cfg *tls.Config) OptFunc {
	return func(u *CMD) (err error) {
		u.tlscfg = cfg
		return
	}
}

type CMD struct {
	MACDerivableKeysetPath  *string `env:"MAC_DERIVABLE_KEYSET_PATH,expand" json:"mac_derivable_keyset_path"`
	AEADDerivableKeysetPath *string `env:"AEAD_DERIVABLE_KEYSET_PATH,expand" json:"aead_derivable_keyset_path"`
	TLSKeyPath              *string `env:"TLS_KEY_PATH,expand" json:"tls_key_path"`
	TLSCertPath             *string `env:"TLS_CERT_PATH,expand" json:"tls_cert_path"`
	TLSCAPath               *string `env:"TLS_CA_PATH,expand" json:"tls_ca_path"`
	TLSMutualAuth           bool    `env:"TLS_MUTUAL_AUTH,expand" json:"tls_mutual_auth"`
	OtelTraceProvider       *string `env:"OTEL_TRACE_PROVIDER" json:"otel_trace_provider" `
	LogLevel                *string `env:"LOG_LEVEL" json:"log_level"`

	tlscfg *tls.Config

	getLogger     func() (log.Logger, error)
	getTLSWrapper func() (tlswrapper.TLSWrapper, error)
	getAEAD       func() (*crypt.DerivableKeyset[crypt.PrimitiveAEAD], error)
	getMAC        func() (*crypt.DerivableKeyset[crypt.PrimitiveMAC], error)
	getHTTPClient func() (httpclient.HTTPClient, error)
}

func New(opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		tlscfg: &tls.Config{},
	}
	for _, opt := range opts {
		if err = opt(c); err != nil {
			return nil, fmt.Errorf("fail to apply options: %w", err)
		}
	}

	c.initLogger()
	c.initTLSWrapper()
	c.initAEADDerivableKeySet()
	c.initMACDerivableKeySet()
	c.initHTTPClient()
	return
}

func (c *CMD) initLogger() {
	opts := []zap.OptFunc{}
	if c.LogLevel != nil {
		opts = append(opts, zap.WithLevelString(*c.LogLevel))
	}
	l, err := zap.New(opts...)

	c.getLogger = func() (log.Logger, error) { return l, err }
}

func (c CMD) Logger() (log.Logger, error) {
	return c.getLogger()
}

func (c *CMD) initTLSWrapper() {
	cfg := c.tlscfg
	if c.TLSMutualAuth {
		cfg = cfg.Clone()
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	opts := []tlswrapper.OptFunc{
		tlswrapper.WithTLSConfig(c.tlscfg),
	}
	if c.TLSCAPath != nil {
		opts = append(opts, tlswrapper.WithCA(*c.TLSCAPath))
	}
	if c.TLSCertPath != nil && c.TLSKeyPath != nil {
		opts = append(opts, tlswrapper.WithLeafCert(*c.TLSKeyPath, *c.TLSCertPath))
	}

	t, err := tlswrapper.New(opts...)
	c.getTLSWrapper = func() (tlswrapper.TLSWrapper, error) { return t, err }
}

func (c CMD) TLSWrapper() (tlswrapper.TLSWrapper, error) {
	return c.getTLSWrapper()
}

func (c *CMD) initAEADDerivableKeySet() {
	if c.AEADDerivableKeysetPath == nil {
		c.getAEAD = func() (*crypt.DerivableKeyset[crypt.PrimitiveAEAD], error) { return nil, nil }
		return
	}

	a, err := crypt.NewInsecureCleartextDerivableKeyset(*c.AEADDerivableKeysetPath, crypt.NewPrimitiveAEAD)
	c.getAEAD = func() (*crypt.DerivableKeyset[crypt.PrimitiveAEAD], error) { return a, err }
}

func (c CMD) AEADDerivableKeyset() (*crypt.DerivableKeyset[crypt.PrimitiveAEAD], error) {
	return c.getAEAD()
}

func (c *CMD) initMACDerivableKeySet() {
	if c.MACDerivableKeysetPath == nil {
		c.getMAC = func() (*crypt.DerivableKeyset[crypt.PrimitiveMAC], error) { return nil, nil }
		return
	}

	m, err := crypt.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, crypt.NewPrimitiveMAC)
	c.getMAC = func() (*crypt.DerivableKeyset[crypt.PrimitiveMAC], error) { return m, err }
}

func (c CMD) MacDerivableKeyset() (*crypt.DerivableKeyset[crypt.PrimitiveMAC], error) {
	return c.getMAC()
}

func (c *CMD) initHTTPClient() {
	opts := []httpclient.OptFunc{}
	if tlswrapper, err := c.getTLSWrapper(); err == nil && tlswrapper != nil {
		d := tlswrapper.WrapDialer(&net.Dialer{Timeout: 10 * time.Second})
		opts = append(opts, httpclient.WithDialTLS(d.DialContext))
	}
	h, err := httpclient.New(opts...)
	c.getHTTPClient = func() (httpclient.HTTPClient, error) { return h, err }
}

func (c CMD) HTTPClient() (httpclient.HTTPClient, error) {
	return c.getHTTPClient()
}

func (c CMD) LoadOtelTraceProvider(ctx context.Context) (deferer func()) {
	n := ""
	if c.OtelTraceProvider != nil {
		n = *c.OtelTraceProvider
	}
	l, _ := c.Logger()
	return otel.WithTraceProvider(ctx, n, l)
}

func (c CMD) CancelOnExitSignal(ctx context.Context) context.Context {
	return util.CancelOnExitSignal(ctx)
}
