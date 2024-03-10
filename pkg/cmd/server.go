package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/telkomindonesia/go-boilerplate/pkg/httpclient"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/logger/zap"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres"
	"github.com/telkomindonesia/go-boilerplate/pkg/tenantservice"
	"github.com/telkomindonesia/go-boilerplate/pkg/tlswrapper"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

type ServerOptFunc func(*Server) error

func ServerWithEnvPrefix(p string) ServerOptFunc {
	return func(s *Server) (err error) {
		s.envPrefix = p
		return
	}
}

func ServerWithOutDotEnv(p string) ServerOptFunc {
	return func(s *Server) (err error) {
		s.dotenv = false
		return
	}
}

type Server struct {
	envPrefix string
	dotenv    bool

	HTTPAddr     string  `env:"HTTP_LISTEN_ADDRESS,expand" envDefault:":8080" json:"http_listen_addr"`
	HTTPKeyPath  *string `env:"HTTP_TLS_KEY_PATH" json:"http_tls_key_path"`
	HTTPCertPath *string `env:"HTTP_TLS_CERT_PATH" json:"http_tls_cert_path"`
	HTTPCA       *string `env:"HTTP_CA_CERTS_PATHS" json:"http_ca_certs_paths"`
	HTTPMTLS     bool    `env:"HTTP_MTLS" json:"http_mtls"`

	PostgresUrl      string `env:"POSTGRES_URL,required,notEmpty,expand" json:"postgres_url"`
	PostgresAEADPath string `env:"POSTGRES_AEAD_KEY_PATH,required,notEmpty" json:"postgres_aead_key_path"`
	PostgresMACPath  string `env:"POSTGRES_MAC_KEY_PATH,required,notEmpty" json:"postgres_mac_key_path"`

	TenantServiceBaseUrl string `env:"TENANT_SERVICE_BASE_URL,required,notEmpty,expand" json:"tenant_service_base_url"`

	l  logger.Logger
	h  *httpserver.HTTPServer
	p  *postgres.Postgres
	ts *tenantservice.TenantService
	hc httpclient.HTTPClient
	t  tlswrapper.TLSWrapper

	closers []func(context.Context) error
}

func NewServer(opts ...ServerOptFunc) (s *Server, err error) {
	s = &Server{envPrefix: "PROFILE_", dotenv: true}
	for _, opt := range opts {
		if err = opt(s); err != nil {
			return
		}
	}

	err = util.LoadFromEnv(s, util.LoadEnvOptions{
		Prefix: s.envPrefix,
		DotEnv: s.dotenv,
	})
	if err != nil {
		return nil, err
	}

	if err = s.initLogger(); err != nil {
		return
	}
	if err = s.initPostgres(); err != nil {
		return
	}
	if err = s.initTLSWrapper(); err != nil {
		return
	}
	if err = s.initHTTPServer(); err != nil {
		return
	}
	if err = s.initHTTPClient(); err != nil {
		return
	}
	if err = s.initTenantService(); err != nil {
		return
	}

	return
}

func (s *Server) initLogger() (err error) {
	s.l, err = zap.New()
	if err != nil {
		return fmt.Errorf("fail to instantiate logger: %w", err)
	}
	return
}

func (s *Server) initPostgres() (err error) {
	s.p, err = postgres.New(
		postgres.WithConnString(s.PostgresUrl),
		postgres.WithInsecureKeysetFiles(s.PostgresAEADPath, s.PostgresMACPath),
		postgres.WithLogger(s.l),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate postges: %w", err)
	}

	s.closers = append(s.closers, s.p.Close)
	return
}

func (s *Server) initTLSWrapper() (err error) {
	t := &tls.Config{}
	if s.HTTPMTLS {
		t.ClientAuth = tls.RequireAndVerifyClientCert
	}

	opts := []tlswrapper.OptFunc{
		tlswrapper.WithTLSConfig(t),
		tlswrapper.WithLogger(s.l),
	}
	if s.HTTPCA != nil {
		opts = append(opts, tlswrapper.WithCA(*s.HTTPCA))
	}
	if s.HTTPKeyPath != nil && s.HTTPCertPath != nil {
		opts = append(opts, tlswrapper.WithLeafCert(*s.HTTPKeyPath, *s.HTTPCertPath))
	}

	s.t, err = tlswrapper.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate TLS Connector: %w", err)
	}

	s.closers = append(s.closers, s.t.Close)
	return
}

func (s *Server) initHTTPServer() (err error) {
	l, err := net.Listen("tcp", s.HTTPAddr)
	if err != nil {
		return fmt.Errorf("fail to start listener: %w", err)
	}
	opts := []httpserver.OptFunc{
		httpserver.WithListener(s.t.WrapListener(l)),
		httpserver.WithProfileRepository(s.p),
		httpserver.WithTenantRepository(s.ts),
		httpserver.WithLogger(s.l),
	}

	s.h, err = httpserver.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate http server: %w", err)
	}
	s.closers = append(s.closers, s.h.Close)
	return
}

func (s *Server) initHTTPClient() (err error) {
	d := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	s.hc, err = httpclient.New(
		httpclient.WithDial(d.DialContext),
		httpclient.WithDialTLS(s.t.WrapDialer(d).DialContext),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate http client: %w", err)
	}
	s.closers = append(s.closers, s.hc.Close)
	return
}

func (s *Server) initTenantService() (err error) {
	s.ts, err = tenantservice.New(
		tenantservice.WithBaseUrl(s.TenantServiceBaseUrl),
		tenantservice.WithHTTPClient(s.hc.Client),
		tenantservice.WithLogger(s.l),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate tenant service: %w", err)
	}
	return
}

func (s *Server) Run(ctx context.Context) (err error) {
	s.l.Info("server_starting", logger.Any("server", s))
	err = s.h.Start(ctx)
	defer func() {
		for _, fn := range s.closers {
			err = errors.Join(err, fn(ctx))
		}
	}()
	return
}
