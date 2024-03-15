package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver"
	"github.com/telkomindonesia/go-boilerplate/pkg/kafka"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres"
	"github.com/telkomindonesia/go-boilerplate/pkg/tenantservice"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger/zap"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/otel"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/tlswrapper"
)

type OptFunc func(*CMD) error

func WithEnvPrefix(p string) OptFunc {
	return func(s *CMD) (err error) {
		s.envPrefix = p
		return
	}
}

func WithoutDotEnv(p string) OptFunc {
	return func(s *CMD) (err error) {
		s.dotenv = false
		return
	}
}

func WithCanceler(f func(context.Context) context.Context) OptFunc {
	return func(s *CMD) (err error) {
		s.canceler = f
		return
	}
}

func WithOtelLoader(f func(ctx context.Context, l logger.Logger) func()) OptFunc {
	return func(s *CMD) (err error) {
		s.otelLoader = f
		return
	}
}

type CMD struct {
	envPrefix  string
	dotenv     bool
	canceler   func(ctx context.Context) context.Context
	otelLoader func(ctx context.Context, l logger.Logger) func()

	HTTPAddr     string  `env:"HTTP_LISTEN_ADDRESS,expand" envDefault:":8080" json:"http_listen_addr"`
	HTTPKeyPath  *string `env:"HTTP_TLS_KEY_PATH" json:"http_tls_key_path"`
	HTTPCertPath *string `env:"HTTP_TLS_CERT_PATH" json:"http_tls_cert_path"`
	HTTPCA       *string `env:"HTTP_CA_CERTS_PATHS" json:"http_ca_certs_paths"`
	HTTPMTLS     bool    `env:"HTTP_MTLS" json:"http_mtls"`

	PostgresUrl      string `env:"POSTGRES_URL,required,notEmpty,expand" json:"postgres_url"`
	PostgresAEADPath string `env:"POSTGRES_AEAD_KEY_PATH,required,notEmpty" json:"postgres_aead_key_path"`
	PostgresMACPath  string `env:"POSTGRES_MAC_KEY_PATH,required,notEmpty" json:"postgres_mac_key_path"`

	KafkaBrokers     []string `env:"KAFKA_BROKERS,required,notEmpty,expand" json:"kafka_url"`
	KafkaTopicOutbox string   `env:"KAFKA_TOPIC_OUTBOX,required,notEmpty,expand" json:"kafka_topic_outbox"`

	TenantServiceBaseUrl string `env:"TENANT_SERVICE_BASE_URL,required,notEmpty,expand" json:"tenant_service_base_url"`

	l  logger.Logger
	h  *httpserver.HTTPServer
	p  *postgres.Postgres
	k  *kafka.Kafka
	ok postgres.OutboxSender
	ts *tenantservice.TenantService
	hc httpclient.HTTPClient
	t  tlswrapper.TLSWrapper

	closers []func(context.Context) error
}

func New(opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		envPrefix:  "PROFILE_",
		dotenv:     true,
		canceler:   util.CancelOnExitSignal,
		otelLoader: otel.FromEnv,
	}
	for _, opt := range opts {
		if err = opt(c); err != nil {
			return
		}
	}

	err = util.LoadFromEnv(c, util.LoadEnvOptions{
		Prefix: c.envPrefix,
		DotEnv: c.dotenv,
	})
	if err != nil {
		return nil, err
	}

	if err = c.initLogger(); err != nil {
		return
	}
	if err = c.initKafka(); err != nil {
		return
	}
	if err = c.initPostgres(); err != nil {
		return
	}
	if err = c.initTLSWrapper(); err != nil {
		return
	}
	if err = c.initHTTPClient(); err != nil {
		return
	}
	if err = c.initTenantService(); err != nil {
		return
	}
	if err = c.initHTTPServer(); err != nil {
		return
	}

	return
}

func (c *CMD) initLogger() (err error) {
	c.l, err = zap.New()
	if err != nil {
		return fmt.Errorf("fail to instantiate logger: %w", err)
	}
	return
}

func (c *CMD) initKafka() (err error) {
	if len(c.KafkaBrokers) == 0 {
		return
	}

	c.k, err = kafka.New(
		kafka.WithBrokers(c.KafkaBrokers),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate kafka:%w", err)
	}

	if c.k != nil && c.KafkaTopicOutbox == "" {
		return fmt.Errorf("invalid kafka outboox topic: %s", c.KafkaTopicOutbox)
	}
	c.ok = func(ctx context.Context, o []*postgres.Outbox) error {
		msgs := make([]kafka.Message, 0, len(o))
		for _, o := range o {
			var msg kafka.Message
			if msg.Value, err = json.Marshal(o); err != nil {
				return fmt.Errorf("fail to marshal outbox: %w", err)
			}
			msgs = append(msgs, msg)
		}
		return c.k.Write(ctx, c.KafkaTopicOutbox, msgs...)
	}

	c.closers = append(c.closers, c.k.Close)
	return
}

func (c *CMD) initPostgres() (err error) {
	opts := []postgres.OptFunc{
		postgres.WithConnString(c.PostgresUrl),
		postgres.WithInsecureKeysetFiles(c.PostgresAEADPath, c.PostgresMACPath),
		postgres.WithLogger(c.l),
	}
	if c.ok != nil {
		opts = append(opts, postgres.WithOutboxSender(c.ok))
	}
	c.p, err = postgres.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate postges: %w", err)
	}

	c.closers = append(c.closers, c.p.Close)
	return
}

func (c *CMD) initTLSWrapper() (err error) {
	t := &tls.Config{}
	if c.HTTPMTLS {
		t.ClientAuth = tls.RequireAndVerifyClientCert
	}

	opts := []tlswrapper.OptFunc{
		tlswrapper.WithTLSConfig(t),
		tlswrapper.WithLogger(c.l),
	}
	if c.HTTPCA != nil {
		opts = append(opts, tlswrapper.WithCA(*c.HTTPCA))
	}
	if c.HTTPKeyPath != nil && c.HTTPCertPath != nil {
		opts = append(opts, tlswrapper.WithLeafCert(*c.HTTPKeyPath, *c.HTTPCertPath))
	}

	c.t, err = tlswrapper.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate TLS Connector: %w", err)
	}

	c.closers = append(c.closers, c.t.Close)
	return
}

func (c *CMD) initHTTPClient() (err error) {
	d := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	c.hc, err = httpclient.New(
		httpclient.WithDial(d.DialContext),
		httpclient.WithDialTLS(c.t.WrapDialer(d).DialContext),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate http client: %w", err)
	}
	c.closers = append(c.closers, c.hc.Close)
	return
}

func (c *CMD) initTenantService() (err error) {
	c.ts, err = tenantservice.New(
		tenantservice.WithBaseUrl(c.TenantServiceBaseUrl),
		tenantservice.WithHTTPClient(c.hc.Client),
		tenantservice.WithLogger(c.l),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate tenant service: %w", err)
	}
	return
}

func (c *CMD) initHTTPServer() (err error) {
	l, err := net.Listen("tcp", c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("fail to start listener: %w", err)
	}
	opts := []httpserver.OptFunc{
		httpserver.WithListener(c.t.WrapListener(l)),
		httpserver.WithProfileRepository(c.p),
		httpserver.WithTenantRepository(c.ts),
		httpserver.WithLogger(c.l),
	}

	c.h, err = httpserver.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate http server: %w", err)
	}
	c.closers = append(c.closers, c.h.Close)
	return
}

func (c *CMD) Run(ctx context.Context) (err error) {
	defer c.otelLoader(ctx, c.l)

	c.l.Info("server starting", logger.Any("server", c))
	err = c.h.Start(c.canceler(ctx))
	defer func() {
		for _, fn := range c.closers {
			err = errors.Join(err, fn(ctx))
		}
	}()
	return
}
