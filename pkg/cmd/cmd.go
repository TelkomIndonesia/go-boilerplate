package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/env"
	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/version"
	"github.com/telkomindonesia/go-boilerplate/pkg/ctxutil"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logzap"
	"github.com/telkomindonesia/go-boilerplate/pkg/oteloader"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/telkomindonesia/go-boilerplate/pkg/tlswrap"
)

type OptFunc func(*CMD) error

func WithEnv(prefix string, dotenv bool) OptFunc {
	return func(u *CMD) error {
		return env.Load(u, env.Options{
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

	Version string `json:"version"`
	tlscfg  *tls.Config

	LoggerE              func() (log.Logger, error)
	TLSWrapE             func() (*tlswrap.TLSWrap, error)
	AEADDerivableKeysetE func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error)
	MacDerivableKeysetE  func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error)
	BIDXDerivableKeysetE func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error)
	HTTPClientE          func() (httpclient.HTTPClient, error)
}

func New(opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		Version: version.Version(),
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
	opts := []logzap.OptFunc{}
	if c.LogLevel != nil {
		opts = append(opts, logzap.WithLevelString(*c.LogLevel))
	}
	l, err := logzap.NewLogger(opts...)

	c.LoggerE = func() (log.Logger, error) { return l, err }
}

func (c CMD) Logger() log.Logger {
	return require(c.LoggerE, log.Global())
}

func (c CMD) loggerOrGlobal() log.Logger {
	l, err := c.LoggerE()
	if err != nil {
		return log.Global()
	}
	return l
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

	l, err := c.LoggerE()
	if err != nil {
		l = log.Global()
	}
	opts = append(opts, tlswrap.WithLogger(l.WithLog(log.String("logger-name", "tlswrap"))))

	t, err := tlswrap.New(opts...)
	c.TLSWrapE = func() (*tlswrap.TLSWrap, error) { return t, err }
}

func (c *CMD) TLSWrap() *tlswrap.TLSWrap {
	return require(c.TLSWrapE, c.loggerOrGlobal())
}

func (c *CMD) initAEADDerivableKeySet() {
	if c.AEADDerivableKeysetPath == nil {
		c.AEADDerivableKeysetE = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error) { return nil, nil }
		return
	}

	a, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.AEADDerivableKeysetPath, tinkx.NewPrimitiveAEAD)
	c.AEADDerivableKeysetE = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error) { return a, err }
}

func (c *CMD) AEADDerivableKeyset() *tinkx.DerivableKeyset[tinkx.PrimitiveAEAD] {
	return require(c.AEADDerivableKeysetE, c.loggerOrGlobal())
}

func (c *CMD) initMACDerivableKeySet() {
	if c.MACDerivableKeysetPath == nil {
		c.MacDerivableKeysetE = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error) { return nil, nil }
		return
	}

	m, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, tinkx.NewPrimitiveMAC)
	c.MacDerivableKeysetE = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error) { return m, err }
}

func (c *CMD) MacDerivableKeyset() *tinkx.DerivableKeyset[tinkx.PrimitiveMAC] {
	return require(c.MacDerivableKeysetE, c.loggerOrGlobal())
}

func (c *CMD) initBIDXDerivableKeyset() {
	if c.MACDerivableKeysetPath == nil {
		c.BIDXDerivableKeysetE = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) { return nil, nil }
		return
	}

	m, err := tinkx.NewInsecureCleartextDerivableKeyset(*c.MACDerivableKeysetPath, tinkx.NewPrimitiveBIDXWithLen(*c.BIDXLength))
	c.BIDXDerivableKeysetE = func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error) { return m, err }
}

func (c *CMD) BIDXDerivableKeyset() *tinkx.DerivableKeyset[tinkx.PrimitiveBIDX] {
	return require(c.BIDXDerivableKeysetE, c.loggerOrGlobal())
}

func (c *CMD) initHTTPClient() {
	opts := []httpclient.OptFunc{}
	if tlswrapper, err := c.TLSWrapE(); err == nil && tlswrapper != nil {
		d := tlswrapper.Dialer(&net.Dialer{Timeout: 10 * time.Second})
		opts = append(opts, httpclient.WithDialTLS(d.DialContext))
	}
	h, err := httpclient.New(opts...)
	c.HTTPClientE = func() (httpclient.HTTPClient, error) { return h, err }
}

func (c *CMD) HTTPClient() httpclient.HTTPClient {
	return require(c.HTTPClientE, c.loggerOrGlobal())
}

func (c CMD) LoadOtel(ctx context.Context) (deferer func()) {
	n := ""
	if c.OtelTraceProvider != nil {
		n = *c.OtelTraceProvider
	}
	l, err := c.LoggerE()
	if err != nil {
		l = log.Global()
	}

	return oteloader.WithTraceProvider(ctx, n, l.WithLog(log.String("logger-name", "otel-loader")))
}

func (c CMD) CancelOnExit(ctx context.Context) context.Context {
	return ctxutil.WithExitSignal(ctx)
}
