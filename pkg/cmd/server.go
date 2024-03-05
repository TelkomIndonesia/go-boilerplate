package cmd

import (
	"context"
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/httpserver"
)

type ServerOptFunc func(*Server) error
type Server struct {
	h *httpserver.HTTPServer
}

func NewServer(opts ...ServerOptFunc) (c *Server, err error) {
	h, err := httpserver.New()
	if err != nil {
		return nil, fmt.Errorf("fail to instantiate HTTP Server: %w", err)
	}

	c = &Server{
		h: h,
	}
	return
}

func (s *Server) Exec(ctx context.Context) (err error) {
	return s.h.Start(ctx)
}
