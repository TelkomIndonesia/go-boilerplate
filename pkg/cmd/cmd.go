package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/zaplogger"
	"github.com/telkomindonesia/go-boilerplate/pkg/otelloader"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/telkomindonesia/go-boilerplate/pkg/tlswrap"
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

	logger     func() (log.Logger, error)
	tlsWrap    func() (*tlswrap.TLSWrap, error)
	aead       func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error)
	mac        func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error)
	bidx       func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error)
	httpClient func() (httpclient.HTTPClient, error)
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
	c.initTLSWrap()
	c.initAEADDerivableKeySet()
	c.initMACDerivableKeySet()
	c.initBIDXDerivableKeyset()
	c.initHTTPClient()
	return
}

func (c *CMD) initLogger() {
	opts := []zaplogger.OptFunc{}
	if c.LogLevel != nil {
		opts = append(opts, zaplogger.WithLevelString(*c.LogLevel))
	}
	l, err := zaplogger.New(opts...)

	c.logger = func() (log.Logger, error) { return l, err }
}

func (c CMD) Logger() (log.Logger, error) {
	return c.logger()
}

func (c *CMD) initTLSWrap() {
	cfg := c.tlscfg
	if c.TLSMutualAuth {
		cfg = cfg.Clone()
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	opts := []tlswrap.OptFunc{
		tlswrap.WithTLSConfig(cfg),
	}
	if c.TLSCAPath != nil {
		opts = append(opts, tlswrap.WithCA(*c.TLSCAPath))
	}
	if c.TLSClientCAPath != nil {
		opts = append(opts, tlswrap.WithClientCA(*c.TLSClientCAPath))
	}
	if c.TLSRootCAPath != nil {
		opts = append(opts, tlswrap.WithRootCA(*c.TLSRootCAPath))
	}
	if c.TLSCertPath != nil && c.TLSKeyPath != nil {
		opts = append(opts, tlswrap.WithLeafCert(*c.TLSKeyPath, *c.TLSCertPath))
	}

	l, err := c.Logger()
	if err != nil {
		l = log.Global()
	}
	opts = append(opts, tlswrap.WithLogger(l.WithLog(log.String("logger-name", "tlswrap"))))

	t, err := tlswrap.New(opts...)
	c.tlsWrap = func() (*tlswrap.TLSWrap, error) { return t, err }
}

func (c CMD) TLSWrap() (*tlswrap.TLSWrap, error) {
	return c.tlsWrap()
}

func (c *CMD) initAEADDerivableKeySet() {
	if c.AEADDerivableKeysetPath == nil {
		c.aead = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error) { return nil, nil }
		return
	}

	a, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.AEADDerivableKeysetPath, tinkx.NewPrimitiveAEAD)
	c.aead = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error) { return a, err }
}

func (c CMD) AEADDerivableKeyset() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error) {
	return c.aead()
}

func (c *CMD) initMACDerivableKeySet() {
	if c.MACDerivableKeysetPath == nil {
		c.mac = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error) { return nil, nil }
		return
	}

	m, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, tinkx.NewPrimitiveMAC)
	c.mac = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error) { return m, err }
}

func (c CMD) MacDerivableKeyset() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error) {
	return c.mac()
}

func (c *CMD) initBIDXDerivableKeyset() {
	if c.MACDerivableKeysetPath == nil {
		c.bidx = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) { return nil, nil }
		return
	}

	m, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, tinkx.NewPrimitiveBIDXWithLen(*c.BIDXLength))
	c.bidx = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) { return m, err }
}

func (c CMD) BIDXDerivableKeyset() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) {
	return c.bidx()
}

func (c CMD) BIDXDerivableKeysetWithLen(len int) func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) {
	if c.MACDerivableKeysetPath == nil {
		return func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) { return nil, nil }
	}

	p := tinkx.NewPrimitiveBIDX
	if len > 0 {
		p = tinkx.NewPrimitiveBIDXWithLen(len)
	}
	m, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, p)
	return func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) { return m, err }
}

func (c *CMD) initHTTPClient() {
	opts := []httpclient.OptFunc{}
	if tlswrapper, err := c.tlsWrap(); err == nil && tlswrapper != nil {
		d := tlswrapper.Dialer(&net.Dialer{Timeout: 10 * time.Second})
		opts = append(opts, httpclient.WithDialTLS(d.DialContext))
	}
	h, err := httpclient.New(opts...)
	c.httpClient = func() (httpclient.HTTPClient, error) { return h, err }
}

func (c CMD) HTTPClient() (httpclient.HTTPClient, error) {
	return c.httpClient()
}

func (c CMD) LoadOtel(ctx context.Context) (deferer func()) {
	n := ""
	if c.OtelTraceProvider != nil {
		n = *c.OtelTraceProvider
	}
	l, err := c.Logger()
	if err != nil {
		l = log.Global()
	}

	return otelloader.WithTraceProvider(ctx, n, l.WithLog(log.String("logger-name", "otel-loader")))
}

func (c CMD) CancelOnExit(ctx context.Context) context.Context {
	return util.CancelOnExitSignal(ctx)
}
