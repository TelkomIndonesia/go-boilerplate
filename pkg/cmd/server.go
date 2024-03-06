package cmd

import (
	"context"
	"fmt"

	env "github.com/caarlos0/env/v10"
	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres"
)

type ServerOptFunc func(*Server) error

func ServerWithEnvPrefix(p string) ServerOptFunc {
	return func(s *Server) (err error) {
		s.envPrefix = p
		return
	}
}

type Server struct {
	envPrefix string

	HTTPAddr     string  `env:"HTTP_LISTEN_ADDRESS,expand"`
	HTTPKeyPath  *string `env:"HTTP_TLS_KEY_PATH"`
	HTTPCertPath *string `env:"HTTP_TLS_CERT_PATH"`

	PostgresUrl      string `env:"POSTGRES_URL,required,notEmpty,expand"`
	PostgresAEADPath string `env:"POSTGRES_AEAD_KEY_PATH,required,notEmpty"`
	PostgresMACPath  string `env:"POSTGRES_MAC_KEY_PATH,required,notEmpty"`

	h *httpserver.HTTPServer
	p *postgres.Postgres
}

func NewServer(opts ...ServerOptFunc) (s *Server, err error) {
	s = &Server{envPrefix: "PROFILE_"}
	for _, opt := range opts {
		if err = opt(s); err != nil {
			return
		}
	}

	opt := env.Options{
		Prefix: s.envPrefix,
	}
	if err = env.ParseWithOptions(s, opt); err != nil {
		return nil, fmt.Errorf("fail to parse options: %w", err)
	}

	if err = s.initPostgres(); err != nil {
		return
	}
	if err = s.initHTTPServer(); err != nil {
		return
	}

	return
}

func (s *Server) initPostgres() (err error) {
	s.p, err = postgres.New(
		postgres.WithConnString(s.PostgresUrl),
		postgres.WithInsecureKeysetFiles(s.PostgresAEADPath, s.PostgresMACPath),
	)
	if err != nil {
		return fmt.Errorf("fail to instantiate postges: %w", err)
	}
	return
}

func (s *Server) initHTTPServer() (err error) {
	opts := []httpserver.OptFunc{
		httpserver.WithAddr(s.HTTPAddr),
		httpserver.WithProfileRepository(s.p),
	}
	if s.HTTPKeyPath != nil && s.HTTPCertPath != nil {
		opts = append(opts, httpserver.WithTLS(*s.HTTPKeyPath, *s.HTTPCertPath))
	}

	s.h, err = httpserver.New(opts...)
	if err != nil {
		return fmt.Errorf("fail to instantiate http server: %w", err)
	}
	return
}

func (s *Server) Run(ctx context.Context) (err error) {
	return s.h.Start(ctx)
}
