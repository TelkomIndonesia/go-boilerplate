package tlswrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/telkomindonesia/go-boilerplate/pkg/filewatch"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

type Dialer interface {
	Dial(net string, addr string) (net.Conn, error)
	DialContext(ctx context.Context, net string, addr string) (net.Conn, error)
}

type OptFunc func(*TLSWrap) error

func WithTLSConfig(cfg *tls.Config) OptFunc {
	return func(c *TLSWrap) (err error) {
		c.cfg = cfg
		return
	}
}

func WithLeafCert(key, cert string) OptFunc {
	return func(c *TLSWrap) (err error) {
		c.keyPath, c.certPath = key, cert
		if err = c.loadLeaf(); err != nil {
			return fmt.Errorf("failed to load leaf cert: %w", err)
		}
		return
	}
}

func WithCA(path string) OptFunc {
	return func(c *TLSWrap) (err error) {
		c.clientCAPath = path
		if err = c.loadClientCA(); err != nil {
			return fmt.Errorf("failed to load CA: %w", err)
		}

		c.rootCAPath = path
		if err = c.loadRootCA(); err != nil {
			return fmt.Errorf("failed to load CA: %w", err)
		}
		return
	}
}

func WithClientCA(path string) OptFunc {
	return func(c *TLSWrap) (err error) {
		c.clientCAPath = path
		if err = c.loadClientCA(); err != nil {
			return fmt.Errorf("failed to load CA: %w", err)
		}
		return
	}
}

func WithRootCA(path string) OptFunc {
	return func(c *TLSWrap) (err error) {
		c.rootCAPath = path
		if err = c.loadRootCA(); err != nil {
			return fmt.Errorf("failed to load CA: %w", err)
		}
		return
	}
}

func WithConfigReloadListener(f func(s, c *tls.Config)) OptFunc {
	return func(w *TLSWrap) (err error) {
		w.listenerFunc = f
		return
	}
}

func WithLogger(l log.Logger) OptFunc {
	return func(c *TLSWrap) (err error) {
		c.logger = l
		return
	}
}

type TLSWrap struct {
	keyPath      string
	certPath     string
	clientCAPath string
	rootCAPath   string
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

func New(opts ...OptFunc) (c *TLSWrap, err error) {
	cr := &TLSWrap{
		logger:       log.Global(),
		cfg:          &tls.Config{},
		listenerFunc: func(c, s *tls.Config) {},
	}
	for _, opt := range opts {
		if err = opt(cr); err != nil {
			return nil, fmt.Errorf("failed to instantiate connector: %w", err)
		}
	}
	if err := cr.initWatcher(); err != nil {
		return nil, err
	}
	if cr.logger == nil {
		return nil, fmt.Errorf("missing logger")
	}

	return cr, err
}

func (c *TLSWrap) initWatcher() (err error) {
	if c.certPath != "" {
		cw, err := filewatch.New(c.certPath, func(s string, err error) {
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
	}

	if c.clientCAPath != "" {
		cw, err := filewatch.New(c.clientCAPath, func(s string, err error) {
			if err != nil {
				c.logger.Error("client-ca-file-watcher", log.Error("error", err))
				return
			}
			if err = c.loadClientCA(); err != nil {
				c.logger.Error("client-ca-file-watcher", log.Error("error", err))
				return
			}
			c.logger.Info("client-ca-file-watcher", log.String("info", "ca file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate ca content watcher")
		}

		c.closers = append(c.closers, cw.Close)
	}

	if c.rootCAPath != "" {
		cw, err := filewatch.New(c.rootCAPath, func(s string, err error) {
			if err != nil {
				c.logger.Error("root-ca-file-watcher", log.Error("error", err))
				return
			}
			if err = c.loadRootCA(); err != nil {
				c.logger.Error("root-ca-file-watcher", log.Error("error", err))
				return
			}
			c.logger.Info("root-ca-file-watcher", log.String("info", "ca file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate root ca content watcher")
		}

		c.closers = append(c.closers, cw.Close)
	}

	return
}

func (c *TLSWrap) loadLeaf() (err error) {
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

func (c *TLSWrap) loadClientCA() (err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	clientCa := x509.NewCertPool()

	certs, err := os.ReadFile(c.clientCAPath)
	if err != nil {
		return fmt.Errorf("failed to open ca file: %w", err)
	}
	if ok := clientCa.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("failed to append x509 cert pool: %w", err)
	}

	c.clientCa = clientCa
	c.cfgs = c.serverConfig()
	c.listenerFunc(c.cfgs, c.cfgc)
	return
}

func (c *TLSWrap) loadRootCA() (err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	rootCA, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert file: %w", err)
	}

	certs, err := os.ReadFile(c.rootCAPath)
	if err != nil {
		return fmt.Errorf("failed to open ca file: %w", err)
	}
	if ok := rootCA.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("failed to append x509 cert pool: %w", err)
	}

	c.rootCA = rootCA
	c.cfgc = c.clientConfig()
	c.listenerFunc(c.cfgs, c.cfgc)
	return
}

func (c *TLSWrap) clientConfig() *tls.Config {
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

func (c *TLSWrap) serverConfig() *tls.Config {
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

func (c *TLSWrap) getCertificate() (*tls.Certificate, error) {
	if c.cert == nil {
		return nil, fmt.Errorf("no certificate found")
	}
	return c.cert, nil
}

func (c *TLSWrap) Dialer(d *net.Dialer) Dialer {
	return dialer{c: c, d: d}
}

func (c *TLSWrap) Listener(l net.Listener) net.Listener {
	return listener{c: c, l: l}
}

func (c *TLSWrap) Close(ctx context.Context) (err error) {
	for _, closer := range c.closers {
		err = errors.Join(closer(ctx))
	}
	return
}

type dialer struct {
	c *TLSWrap
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
	c *TLSWrap
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
