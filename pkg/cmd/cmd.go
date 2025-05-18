package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/env"
	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/version"
	"github.com/telkomindonesia/go-boilerplate/pkg/ctxutil"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/oteloader"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/telkomindonesia/go-boilerplate/pkg/tlswrap"

	"go.opentelemetry.io/contrib/bridges/otelslog"
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
	MACDerivableKeysetPath  *string           `env:"MAC_DERIVABLE_KEYSET_PATH,expand" json:"mac_derivable_keyset_path"`
	AEADDerivableKeysetPath *string           `env:"AEAD_DERIVABLE_KEYSET_PATH,expand" json:"aead_derivable_keyset_path"`
	BIDXDerivableKeysetPath *string           `env:"BIDX_DERIVABLE_KEYSET_PATH,expand" json:"bidx_derivable_keyset_path"`
	BIDXLength              *int              `env:"BIDX_LENGTH,expand" envDefault:"16" json:"bidx_length"`
	TLSKeyPath              *string           `env:"TLS_KEY_PATH,expand" json:"tls_key_path"`
	TLSCertPath             *string           `env:"TLS_CERT_PATH,expand" json:"tls_cert_path"`
	TLSCAPath               *string           `env:"TLS_CA_PATH,expand" json:"tls_ca_path"`
	TLSClientCAPath         *string           `env:"TLS_CLIENT_CA_PATH,expand" json:"tls_client_ca_path"`
	TLSRootCAPath           *string           `env:"TLS_ROOT_CA_PATH,expand" json:"tls_root_ca_path"`
	TLSMutualAuth           bool              `env:"TLS_MUTUAL_AUTH,expand" json:"tls_mutual_auth"`
	OtelTraceExporter       *string           `env:"OTEL_TRACE_EXPORTER" json:"otel_trace_exporter" `
	OtelLogExporter         *string           `env:"OTEL_LOG_EXPORTER" json:"otel_log_exporter" `
	LogLevel                *string           `env:"LOG_LEVEL" json:"log_level"`
	LogExporter             map[string]string `env:"LOG_EXPORTER" json:"log_exporter"`

	Version string `json:"version"`
	tlscfg  *tls.Config

	LoggerE              func() (log.Logger, error)
	TLSWrapE             func() (*tlswrap.TLSWrap, error)
	AEADDerivableKeysetE func() (*tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], error)
	MacDerivableKeysetE  func() (*tinkx.DerivableKeyset[tinkx.PrimitiveMAC], error)
	BIDXDerivableKeysetE func() (*tinkx.DerivableKeyset[tinkx.PrimitiveBIDX], error)
	HTTPClientE          func() (httpclient.HTTPClient, error)

	closers []func(context.Context) error
}

func New(ctx context.Context, opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		Version: version.Version(),
		tlscfg:  &tls.Config{},
		closers: []func(context.Context) error{},
	}
	defer func() {
		if err == nil {
			return
		}
		c.Close(ctx)
	}()

	for _, opt := range opts {
		if err = opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply options: %w", err)
		}
	}

	c.initOtel(ctx)
	c.initLogger()
	c.initTLSWrap()
	c.initAEADDerivableKeySet()
	c.initMACDerivableKeySet()
	c.initBIDXDerivableKeyset()
	c.initHTTPClient()
	return
}

func (c *CMD) initOtel(ctx context.Context) {
	l := log.Global()

	traceProvider := ""
	if c.OtelTraceExporter != nil {
		traceProvider = *c.OtelTraceExporter
	}
	traceCloser := oteloader.WithTraceProvider(ctx, traceProvider, l.WithAttrs(log.String("logger-name", "otel-loader")))

	logProvider := ""
	if c.OtelLogExporter != nil {
		logProvider = *c.OtelLogExporter
	}
	logCloser := oteloader.WithLogProvider(ctx, logProvider, l.WithAttrs(log.String("logger-name", "otel-loader")))

	c.closers = append(c.closers, func(ctx context.Context) error {
		traceCloser()
		logCloser()
		return nil
	})
}

func (c *CMD) initLogger() {
	lvl := log.Level("info")
	if c.LogLevel != nil {
		lvl = log.Level(*c.LogLevel)
	}

	exporters := map[string]string{}
	for k, v := range c.LogExporter {
		if k != "console" && k != "otel" {
			continue
		}
		if k == "console" && (v != "keyval" && v != "json") {
			continue
		}
		exporters[k] = v
	}
	if len(exporters) == 0 {
		exporters["console"] = "keyval"
	}

	handlers := []slog.Handler{}
	for k, v := range exporters {
		switch {
		default:
			handlers = append(handlers, log.NewTraceableHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
		case k == "console" && v == "json":
			handlers = append(handlers, log.NewTraceableHandler(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
		case k == "otel":
			handlers = append(handlers, otelslog.NewHandler("cmd"))
		}
	}

	l := log.NewLogger(log.WithHandlers(handlers...))
	log.SetGlobal(l)
	c.LoggerE = func() (log.Logger, error) { return l, nil }
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
	opts = append(opts, tlswrap.WithLogger(l.WithAttrs(log.String("logger-name", "tlswrap"))))

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

	a, err := tinkx.NewInsecureCleartextDerivableKeyset(
		*c.AEADDerivableKeysetPath, tinkx.NewPrimitiveAEAD, tinkx.DerivableKeysetWithCapCache[tinkx.PrimitiveAEAD](100),
	)
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

	m, err := tinkx.NewInsecureCleartextDerivableKeyset(
		*c.MACDerivableKeysetPath, tinkx.NewPrimitiveMAC, tinkx.DerivableKeysetWithCapCache[tinkx.PrimitiveMAC](100),
	)
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

	m, err := tinkx.NewInsecureCleartextDerivableKeyset(
		*c.MACDerivableKeysetPath, tinkx.NewPrimitiveBIDXWithLen(*c.BIDXLength), tinkx.DerivableKeysetWithCapCache[tinkx.PrimitiveBIDX](100),
	)
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

func (c CMD) CancelOnExit(ctx context.Context) context.Context {
	return ctxutil.WithExitSignal(ctx)
}

func (c CMD) Close(ctx context.Context) (err error) {
	for _, fn := range c.closers {
		err = errors.Join(err, fn(ctx))
	}
	return nil
}
