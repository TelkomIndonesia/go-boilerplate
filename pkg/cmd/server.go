package cmd

import "context"

type ServerOptFunc func(*Server) error
type Server struct {
}

func NewServer(opts ...ServerOptFunc) (c *Server, err error) {
	c = &Server{}
	for _, opt := range opts {
		if err = opt(c); err != nil {
			return
		}
	}
	return
}

func (s Server) Start(ctx context.Context) error {
	return ctx.Err()
}
