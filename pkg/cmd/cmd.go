package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver"
	"github.com/telkomindonesia/go-boilerplate/pkg/kafka"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres"
	"github.com/telkomindonesia/go-boilerplate/pkg/tenantservice"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/tlswrapper"
)

type OptFunc func(*CMD) error

func WithEnvPrefix(p string) OptFunc {
	return func(s *CMD) (err error) {
		s.envPrefix = p
		return
	}
}

func WithoutDotEnv() OptFunc {
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

func WithOtelLoader(f func(ctx context.Context) func()) OptFunc {
	return func(s *CMD) (err error) {
		s.otelLoader = f
		return
	}
}

type CMD struct {
	envPrefix string
	dotenv    bool

	HTTPAddr             string   `env:"HTTP_LISTEN_ADDRESS,expand" envDefault:":8080" json:"http_listen_addr"`
	PostgresUrl          string   `env:"POSTGRES_URL,required,notEmpty,expand" json:"postgres_url"`
	KafkaBrokers         []string `env:"KAFKA_BROKERS,required,notEmpty,expand" json:"kafka_url"`
	KafkaTopicOutbox     string   `env:"KAFKA_TOPIC_OUTBOX,required,notEmpty,expand" json:"kafka_topic_outbox"`
	TenantServiceBaseUrl string   `env:"TENANT_SERVICE_BASE_URL,required,notEmpty,expand" json:"tenant_service_base_url"`

	CMD        *cmd.CMD `env:"-" json:"cmd"`
	l          logger.Logger
	aead       *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
	mac        *crypt.DerivableKeyset[crypt.PrimitiveMAC]
	hc         httpclient.HTTPClient
	tls        tlswrapper.TLSWrapper
	canceler   func(ctx context.Context) context.Context
	otelLoader func(ctx context.Context) func()

	h   *httpserver.HTTPServer
	p   *postgres.Postgres
	k   *kafka.Kafka
	pok postgres.OutboxSender
	ts  *tenantservice.TenantService

	closers []func(context.Context) error
}

func New(opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		envPrefix: "PROFILE_",
		dotenv:    true,
	}
	for _, opt := range opts {
		if err = opt(c); err != nil {
			return
		}
	}
	err = util.LoadEnv(c, util.LoadEnvOptions{
		Prefix: c.envPrefix,
		DotEnv: c.dotenv,
	})
	if err != nil {
		return nil, err
	}

	if err = c.initCMD(); err != nil {
		return
	}
	if err = c.initKafka(); err != nil {
		return
	}
	if err = c.initPostgres(); err != nil {
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

func (c *CMD) initCMD() (err error) {
	c.CMD, err = cmd.New(cmd.WithEnv(c.envPrefix, c.dotenv))
	if err != nil {
		return fmt.Errorf("fail to instantiate cmd: %w", err)
	}

	c.l, err = c.CMD.Logger()
	if err != nil {
		return fmt.Errorf("fail to instantiate logger: %w", err)
	}
	c.aead, err = c.CMD.AEADDerivableKeyset()
	if err != nil {
		return fmt.Errorf("fail to instantiate aead: %w", err)
	}
	c.mac, err = c.CMD.MacDerivableKeyset()
	if err != nil {
		return fmt.Errorf("fail to instantiate mac: %w", err)
	}
	c.tls, err = c.CMD.TLSWrapper()
	if err != nil {
		return fmt.Errorf("fail to instantiate tlswrapper: %w", err)
	}
	c.hc, err = c.CMD.HTTPClient()
	if err != nil {
		return fmt.Errorf("fail to instantiate httpclient: %w", err)
	}
	if c.otelLoader == nil {
		c.otelLoader = c.CMD.LoadOtelTraceProvider
	}
	if c.canceler == nil {
		c.canceler = c.CMD.CancelOnExitSignal
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
	c.pok = func(ctx context.Context, o []*postgres.Outbox) error {
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
		postgres.WithDerivableKeysets(c.aead, c.mac),
		postgres.WithLogger(c.l),
	}
	if c.pok != nil {
		opts = append(opts, postgres.WithOutboxSender(c.pok))
	}
	c.p, err = postgres.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate postges: %w", err)
	}

	c.closers = append(c.closers, c.p.Close)
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

	c.h, err = httpserver.New(
		httpserver.WithListener(c.tls.WrapListener(l)),
		httpserver.WithProfileRepository(c.p),
		httpserver.WithTenantRepository(c.ts),
		httpserver.WithLogger(c.l),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate http server: %w", err)
	}

	c.closers = append(c.closers, c.h.Close)
	return
}

func (c *CMD) Run(ctx context.Context) (err error) {
	ctx = c.canceler(ctx)
	defer c.otelLoader(ctx)
	defer func() {
		for _, fn := range c.closers {
			err = errors.Join(err, fn(ctx))
		}
	}()

	c.l.Info("server starting", logger.Any("server", c))
	err = c.h.Start(ctx)
	return
}
