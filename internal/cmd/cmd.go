package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/telkomindonesia/go-boilerplate/internal/httpserver"
	"github.com/telkomindonesia/go-boilerplate/internal/kafka"
	"github.com/telkomindonesia/go-boilerplate/internal/otelwrap"
	"github.com/telkomindonesia/go-boilerplate/internal/postgres"
	"github.com/telkomindonesia/go-boilerplate/internal/tenantservice"
	"github.com/telkomindonesia/go-boilerplate/pkg/cmd"
	"github.com/telkomindonesia/go-boilerplate/pkg/cmd/env"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/log/logvaluer"
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

var _ log.Valuer = CMD{}

type CMD struct {
	envPrefix string
	dotenv    bool

	HTTPAddr             string                        `env:"HTTP_LISTEN_ADDRESS,expand" envDefault:":8080" json:"http_listen_addr"`
	PostgresUrl          logvaluer.MaskedStringUserURL `env:"POSTGRES_URL,required,notEmpty,expand" json:"postgres_url"`
	KafkaBrokers         []string                      `env:"KAFKA_BROKERS,expand" json:"kafka_brokers"`
	KafkaTopicOutbox     string                        `env:"KAFKA_TOPIC_OUTBOX,expand" json:"kafka_topic_outbox"`
	TenantServiceBaseUrl logvaluer.MaskedStringUserURL `env:"TENANT_SERVICE_BASE_URL,required,notEmpty,expand" json:"tenant_service_base_url"`

	CMD *cmd.CMD `env:"-" json:"cmd"`

	h  *httpserver.HTTPServer
	p  *postgres.Postgres
	k  *kafka.Kafka
	ts *tenantservice.TenantService

	closers []func(context.Context) error
}

func New(ctx context.Context, opts ...OptFunc) (c *CMD, err error) {
	c = &CMD{
		envPrefix: "PROFILE_",
		dotenv:    true,
	}
	defer func() {
		if err == nil {
			return
		}
		c.close(context.Background(), err)
	}()

	for _, opt := range opts {
		if err = opt(c); err != nil {
			return
		}
	}
	err = env.Load(c, env.Options{
		Prefix: c.envPrefix,
		DotEnv: c.dotenv,
	})
	if err != nil {
		return nil, err
	}

	if err = c.initCMD(ctx); err != nil {
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

func (c CMD) AsLog() any {
	return logvaluer.AsLog(c)
}

func (c *CMD) initCMD(ctx context.Context) (err error) {
	c.CMD, err = cmd.New(ctx, cmd.WithEnv(c.envPrefix, c.dotenv))
	if err != nil {
		return fmt.Errorf("failed to instantiate cmd: %w", err)
	}
	c.closers = append(c.closers, c.CMD.Close)

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
		return fmt.Errorf("failed to instantiate kafka: %w", err)
	}

	if c.k != nil && c.KafkaTopicOutbox == "" {
		return fmt.Errorf("invalid kafka outboox topic: %s", c.KafkaTopicOutbox)
	}

	c.closers = append(c.closers, c.k.Close)
	return
}

func (c *CMD) initPostgres() (err error) {
	opts := []postgres.OptFunc{
		postgres.WithConnString(c.PostgresUrl.String()),
		postgres.WithDerivableKeysets(c.CMD.AEADDerivableKeyset(), c.CMD.BIDXDerivableKeyset()),
		postgres.WithLogger(c.CMD.Logger().WithAttrs(log.String("logger-name", "postgres"))),
	}
	if c.k != nil {
		opts = append(opts, postgres.WithOutboxCERelayFunc(c.k.OutboxCERelayFunc()))
	}
	c.p, err = postgres.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to instantiate postges: %w", err)
	}

	c.closers = append(c.closers, c.p.Close)
	return
}

func (c *CMD) initTenantService() (err error) {
	c.ts, err = tenantservice.New(
		tenantservice.WithBaseUrl(c.TenantServiceBaseUrl.String()),
		tenantservice.WithHTTPClient(c.CMD.HTTPClient().Client),
		tenantservice.WithLogger(c.CMD.Logger().WithAttrs(log.String("logger-name", "tenant-service"))),
	)
	if err != nil {
		return fmt.Errorf("failed to instantiate tenant service: %w", err)
	}
	return
}

func (c *CMD) initHTTPServer() (err error) {
	l, err := net.Listen("tcp", c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	c.h, err = httpserver.New(
		httpserver.WithListener(c.CMD.TLSWrap().Listener(l)),
		httpserver.WithProfileRepository(otelwrap.NewProfileRepositoryWrapper(c.p, otelwrap.Tracer, "Postgres")),
		httpserver.WithTenantRepository(otelwrap.NewTenantRepositoryWrapper(c.ts, otelwrap.Tracer, "TenantService")),
		httpserver.WithLogger(c.CMD.Logger().WithAttrs(log.String("logger-name", "http-server"))),
	)
	if err != nil {
		return fmt.Errorf("failed to instantiate http server: %w", err)
	}

	c.closers = append(c.closers, c.h.Close)
	return
}

func (c *CMD) Run(ctx context.Context) (err error) {
	defer func() { err = c.close(ctx, err) }()

	c.CMD.Logger().Info(ctx, "server starting", log.Any("server", c))
	return c.h.Start(c.CMD.CancelOnExit(ctx))
}

func (c *CMD) close(ctx context.Context, err error) error {
	for _, fn := range c.closers {
		err = errors.Join(err, fn(ctx))
	}
	return err
}
