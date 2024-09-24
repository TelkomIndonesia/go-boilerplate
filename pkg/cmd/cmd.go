package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/crypto"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/zap"
	"github.com/telkomindonesia/go-boilerplate/pkg/otel"
	"github.com/telkomindonesia/go-boilerplate/pkg/tlswrapper"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
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
	BIDXDerivableKeysetPath *string `env:"BIDX_DERIVABLE_KEYSET_PATH,expand" json:"bidx_derivable_keyset_path"`
	BIDXLength              *int    `env:"BIDX_LENGTH,expand" envDefault:"16" json:"bidx_length"`
	TLSKeyPath              *string `env:"TLS_KEY_PATH,expand" json:"tls_key_path"`
	TLSCertPath             *string `env:"TLS_CERT_PATH,expand" json:"tls_cert_path"`
	TLSCAPath               *string `env:"TLS_CA_PATH,expand" json:"tls_ca_path"`
	TLSClientCAPath         *string `env:"TLS_CLIENT_CA_PATH,expand" json:"tls_client_ca_path"`
	TLSRootCAPath           *string `env:"TLS_ROOT_CA_PATH,expand" json:"tls_root_ca_path"`
	TLSMutualAuth           bool    `env:"TLS_MUTUAL_AUTH,expand" json:"tls_mutual_auth"`
	OtelTraceProvider       *string `env:"OTEL_TRACE_PROVIDER" json:"otel_trace_provider" `
	LogLevel                *string `env:"LOG_LEVEL" json:"log_level"`
	Version                 string  `json:"version"`

	tlscfg *tls.Config

	getLogger     func() (log.Logger, error)
	getTLSWrapper func() (tlswrapper.TLSWrapper, error)
	getAEAD       func() (*crypto.DerivableKeyset[crypto.PrimitiveAEAD], error)
	getMAC        func() (*crypto.DerivableKeyset[crypto.PrimitiveMAC], error)
	getBIDX       func() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error)
	getHTTPClient func() (httpclient.HTTPClient, error)
}

func New(opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		Version: util.Version(),
		tlscfg:  &tls.Config{},
	}
	for _, opt := range opts {
		if err = opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply options: %w", err)
		}
	}

	c.initLogger()
	c.initTLSWrapper()
	c.initAEADDerivableKeySet()
	c.initMACDerivableKeySet()
	c.initBIDXDerivableKeyset()
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
		tlswrapper.WithTLSConfig(cfg),
	}
	if c.TLSCAPath != nil {
		opts = append(opts, tlswrapper.WithCA(*c.TLSCAPath))
	}
	if c.TLSClientCAPath != nil {
		opts = append(opts, tlswrapper.WithClientCA(*c.TLSClientCAPath))
	}
	if c.TLSRootCAPath != nil {
		opts = append(opts, tlswrapper.WithRootCA(*c.TLSRootCAPath))
	}
	if c.TLSCertPath != nil && c.TLSKeyPath != nil {
		opts = append(opts, tlswrapper.WithLeafCert(*c.TLSKeyPath, *c.TLSCertPath))
	}
	if l, err := c.Logger(); err == nil && l != nil {
		opts = append(opts, tlswrapper.WithLogger(l))
	}

	t, err := tlswrapper.New(opts...)
	c.getTLSWrapper = func() (tlswrapper.TLSWrapper, error) { return t, err }
}

func (c CMD) TLSWrapper() (tlswrapper.TLSWrapper, error) {
	return c.getTLSWrapper()
}

func (c *CMD) initAEADDerivableKeySet() {
	if c.AEADDerivableKeysetPath == nil {
		c.getAEAD = func() (*crypto.DerivableKeyset[crypto.PrimitiveAEAD], error) { return nil, nil }
		return
	}

	a, err := crypto.NewInsecureCleartextDerivableKeyset(*c.AEADDerivableKeysetPath, crypto.NewPrimitiveAEAD)
	c.getAEAD = func() (*crypto.DerivableKeyset[crypto.PrimitiveAEAD], error) { return a, err }
}

func (c CMD) AEADDerivableKeyset() (*crypto.DerivableKeyset[crypto.PrimitiveAEAD], error) {
	return c.getAEAD()
}

func (c *CMD) initMACDerivableKeySet() {
	if c.MACDerivableKeysetPath == nil {
		c.getMAC = func() (*crypto.DerivableKeyset[crypto.PrimitiveMAC], error) { return nil, nil }
		return
	}

	m, err := crypto.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, crypto.NewPrimitiveMAC)
	c.getMAC = func() (*crypto.DerivableKeyset[crypto.PrimitiveMAC], error) { return m, err }
}

func (c CMD) MacDerivableKeyset() (*crypto.DerivableKeyset[crypto.PrimitiveMAC], error) {
	return c.getMAC()
}

func (c *CMD) initBIDXDerivableKeyset() {
	if c.MACDerivableKeysetPath == nil {
		c.getBIDX = func() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error) { return nil, nil }
		return
	}

	m, err := crypto.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, crypto.NewPrimitiveBIDXWithLen(*c.BIDXLength))
	c.getBIDX = func() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error) { return m, err }
}

func (c CMD) BIDXDerivableKeyset() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error) {
	return c.getBIDX()
}

func (c CMD) BIDXDerivableKeysetWithLen(len int) func() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error) {
	if c.MACDerivableKeysetPath == nil {
		return func() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error) { return nil, nil }
	}

	p := crypto.NewPrimitiveBIDX
	if len > 0 {
		p = crypto.NewPrimitiveBIDXWithLen(len)
	}
	m, err := crypto.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, p)
	return func() (*crypto.DerivableKeyset[crypto.PrimitiveBIDX], error) { return m, err }
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
