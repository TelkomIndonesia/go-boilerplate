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

	"github.com/telkomindonesia/go-boilerplate/pkg/util/filewatcher"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

type Dialer interface {
	Dial(net string, addr string) (net.Conn, error)
	DialContext(ctx context.Context, net string, addr string) (net.Conn, error)
}

type TLSWrapper interface {
	WrapListener(net.Listener) net.Listener
	WrapDialer(*net.Dialer) Dialer
	Close(context.Context) error
}

var _ TLSWrapper = &wrapper{}

type OptFunc func(*wrapper) error

func WithTLSConfig(cfg *tls.Config) OptFunc {
	return func(c *wrapper) (err error) {
		c.cfg = cfg
		return
	}
}

func WithLeafCert(key, cert string) OptFunc {
	return func(c *wrapper) (err error) {
		c.keyPath, c.certPath = key, cert
		if err = c.loadLeaf(); err != nil {
			return fmt.Errorf("failed to load leaf cert: %w", err)
		}

		cw, err := filewatcher.New(cert, func(s string, err error) {
			if err != nil {
				c.logger.Error("leaf-cert-file-watcher", log.Error("error", err))
				return
			}
			if err = c.loadLeaf(); err != nil {
				c.logger.Error("leaf-cert-file-watcher", log.Error("error", err))
				return
			}
			c.logger.Info("leaf-cert-file-watcher", log.String("info", "leaf cert file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate leaf cert content watcher")
		}

		c.closers = append(c.closers, cw.Close)
		return
	}
}

func WithCA(path string) OptFunc {
	return func(c *wrapper) (err error) {
		c.caPath = path
		if err = c.loadCA(); err != nil {
			return fmt.Errorf("failed to load CA: %w", err)
		}

		cw, err := filewatcher.New(path, func(s string, err error) {
			if err != nil {
				c.logger.Error("ca-file-watcher", log.Error("error", err))
				return
			}
			if err = c.loadCA(); err != nil {
				c.logger.Error("ca-file-watcher", log.Error("error", err))
				return
			}
			c.logger.Info("ca-file-watcher", log.String("info", "ca file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate ca content watcher")
		}

		c.closers = append(c.closers, cw.Close)
		return
	}
}

func WithConfigReloadListener(f func(s, c *tls.Config)) OptFunc {
	return func(w *wrapper) (err error) {
		w.listenerFunc = f
		return
	}
}

func WithLogger(l log.Logger) OptFunc {
	return func(c *wrapper) (err error) {
		c.logger = l
		return
	}
}

type wrapper struct {
	keyPath      string
	certPath     string
	caPath       string
	logger       log.Logger
	cfg          *tls.Config
	mux          sync.Mutex
	listenerFunc func(c, s *tls.Config)
	closers      []func(context.Context) error

	cert     *tls.Certificate
	clientCa *x509.CertPool
	rootCA   *x509.CertPool
	cfgs     *tls.Config
	cfgc     *tls.Config
}

func New(opts ...OptFunc) (c TLSWrapper, err error) {
	cr := &wrapper{
		logger:       log.Global(),
		cfg:          &tls.Config{},
		listenerFunc: func(c, s *tls.Config) {},
	}
	for _, opt := range opts {
		if err = opt(cr); err != nil {
			return nil, fmt.Errorf("failed to instantiate connector: %w", err)
		}
	}
	if cr.logger == nil {
		return nil, fmt.Errorf("missing logger")
	}

	return cr, err
}

func (c *wrapper) loadLeaf() (err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	cert, err := tls.LoadX509KeyPair(c.certPath, c.keyPath)
	if err != nil {
		return fmt.Errorf("failed to load x509 key pair: %w", err)
	}

	c.cert = &cert
	c.cfgs = c.serverConfig()
	c.cfgc = c.clientConfig()
	c.listenerFunc(c.cfgs, c.cfgc)
	return
}

func (c *wrapper) loadCA() (err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	clientCa := x509.NewCertPool()
	rootCA, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert file: %w", err)
	}

	certs, err := os.ReadFile(c.caPath)
	if err != nil {
		return fmt.Errorf("failed to open ca file: %w", err)
	}
	if ok := rootCA.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("failed to append x509 cert pool: %w", err)
	}
	if ok := clientCa.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("failed to append x509 cert pool: %w", err)
	}

	c.clientCa = clientCa
	c.rootCA = rootCA
	c.cfgs = c.serverConfig()
	c.cfgc = c.clientConfig()
	c.listenerFunc(c.cfgs, c.cfgc)
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

func (c *wrapper) WrapDialer(d *net.Dialer) Dialer {
	return dialer{c: c, d: d}
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

type dialer struct {
	c *wrapper
	d *net.Dialer
}

func (d dialer) Dial(net string, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), net, addr)
}

func (d dialer) DialContext(ctx context.Context, net string, addr string) (net.Conn, error) {
	return (&tls.Dialer{NetDialer: d.d, Config: d.c.cfgc}).
		DialContext(ctx, net, addr)
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

	return tls.Server(conn, c.c.cfgs), nil
}

func (c listener) Close() error {
	return c.l.Close()
}

func (c listener) Addr() net.Addr {
	return c.l.Addr()
}
