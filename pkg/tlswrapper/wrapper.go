package tlswrapper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

type Dialer func(ctx context.Context, network string, addr string) (net.Conn, error)

type Wrapper interface {
	WrapListener(net.Listener) net.Listener
	WrapDialer(*net.Dialer) *tls.Dialer
	Close(context.Context) error
}

var _ Wrapper = &wrapper{}

type WrapperOptFunc func(*wrapper) error

func WrapperWithTLSConfig(cfg *tls.Config) WrapperOptFunc {
	return func(c *wrapper) (err error) {
		c.cfg = cfg
		return
	}
}

func WrapperWithLeaf(key, cert string) WrapperOptFunc {
	return func(c *wrapper) (err error) {
		c.keyPath, c.certPath = key, cert
		if err = c.loadLeaf(); err != nil {
			return fmt.Errorf("fail to load leaf cert: %w", err)
		}

		cw, err := util.NewFileContentWatcher(cert, func(s string, err error) {
			if err != nil {
				c.logger.Error("leaf-cert-file-watcher", logger.Any("error", err))
				return
			}
			if err = c.loadLeaf(); err != nil {
				c.logger.Error("leaf-cert-file-watcher", logger.Any("error", err))
				return
			}
			c.logger.Info("leaf-cert-file-watcher", logger.String("info", "leaf cert file updated"))
		})
		if err != nil {
			return fmt.Errorf("fail to instantiate leaf cert content watcher")
		}

		c.closers = append(c.closers, cw.Close)
		return
	}
}

func WrapperWithCA(path string) WrapperOptFunc {
	return func(c *wrapper) (err error) {
		c.caPath = path
		if err = c.loadCA(); err != nil {
			return fmt.Errorf("fail to load CA: %w", err)
		}

		cw, err := util.NewFileContentWatcher(path, func(s string, err error) {
			if err != nil {
				c.logger.Error("ca-file-watcher", logger.Any("error", err))
				return
			}
			if err = c.loadLeaf(); err != nil {
				c.logger.Error("ca-file-watcher", logger.Any("error", err))
				return
			}
			c.logger.Info("ca-file-watcher", logger.String("info", "ca file updated"))
		})
		if err != nil {
			return fmt.Errorf("fail to instantiate ca content watcher")
		}

		c.closers = append(c.closers, cw.Close)
		return
	}
}

func WrapperWithLogger(l logger.Logger) WrapperOptFunc {
	return func(c *wrapper) (err error) {
		c.logger = l
		return
	}
}

type wrapper struct {
	keyPath  string
	certPath string
	caPath   string

	cert     *tls.Certificate
	clientCa *x509.CertPool
	rootCA   *x509.CertPool

	logger logger.Logger
	cfg    *tls.Config
	mux    sync.Mutex

	serverCfg *tls.Config
	clientCfg *tls.Config
	closers   []func(context.Context) error
}

func New(opts ...WrapperOptFunc) (c Wrapper, err error) {
	cr := &wrapper{
		logger: logger.Global(),
		cfg:    &tls.Config{},
	}
	for _, opt := range opts {
		if err = opt(cr); err != nil {
			return nil, fmt.Errorf("fail to instantiate connector: %w", err)
		}
	}

	return cr, err
}

func (c *wrapper) loadLeaf() (err error) {
	cert, err := tls.LoadX509KeyPair(c.certPath, c.keyPath)
	if err != nil {
		return fmt.Errorf("fail to load x509 key pair: %w", err)
	}

	c.cert = &cert

	c.mux.Lock()
	defer c.mux.Unlock()
	c.serverCfg = c.serverConfig()
	c.clientCfg = c.clientConfig()
	return
}

func (c *wrapper) loadCA() (err error) {
	c.clientCa = x509.NewCertPool()
	c.rootCA, err = x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("fail to load system cert file: %w", err)
	}

	certs, err := os.ReadFile(c.caPath)
	if err != nil {
		return fmt.Errorf("fail to open ca file: %w", err)
	}
	if ok := c.rootCA.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("fail to append x509 cert pool: %w", err)
	}
	if ok := c.clientCa.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("fail to append x509 cert pool: %w", err)
	}

	c.mux.Lock()
	defer c.mux.Unlock()
	c.serverCfg = c.serverConfig()
	c.clientCfg = c.clientConfig()
	return
}

func (c *wrapper) clientConfig() *tls.Config {
	cfg := c.cfg.Clone()
	cfg.RootCAs = c.rootCA
	if c.cert == nil {
		return cfg
	}

	cfg.GetClientCertificate = func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return c.cert, nil
	}

	return cfg
}

func (c *wrapper) serverConfig() *tls.Config {
	cfg := c.cfg.Clone()
	if c.cert == nil {
		return cfg
	}

	cfg.ClientCAs = c.clientCa
	cfg.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return c.cert, nil
	}
	return cfg
}

func (c *wrapper) getCertificate() (*tls.Certificate, error) {
	if c.cert == nil {
		return nil, fmt.Errorf("no certificate found")
	}
	return c.cert, nil
}

func (c *wrapper) WrapDialer(d *net.Dialer) *tls.Dialer {
	return &tls.Dialer{
		NetDialer: d,
		Config:    c.clientCfg,
	}
}

func (c *wrapper) WrapListener(l net.Listener) net.Listener {
	return listener{c: c, l: l}
}

func (c *wrapper) Close(ctx context.Context) (err error) {
	for _, closer := range c.closers {
		err = errors.Join(closer(ctx))
	}
	return
}

type listener struct {
	c *wrapper
	l net.Listener
}

func (c listener) Accept() (net.Conn, error) {
	conn, err := c.l.Accept()
	if err != nil || c.c.cert == nil {
		return conn, err
	}

	return tls.Server(conn, c.c.serverCfg), nil
}

func (c listener) Close() error {
	return c.l.Close()
}

func (c listener) Addr() net.Addr {
	return c.l.Addr()
}
