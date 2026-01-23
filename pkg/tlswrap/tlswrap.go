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
	"sync/atomic"

	"github.com/telkomindonesia/go-boilerplate/pkg/filewatch"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

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

	cert     atomic.Pointer[tls.Certificate]
	clientCa *x509.CertPool
	rootCA   *x509.CertPool
	cfgs     atomic.Pointer[tls.Config]
	cfgc     atomic.Pointer[tls.Config]
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

func (tw *TLSWrap) initWatcher() (err error) {
	ctx := context.Background()
	if tw.certPath != "" {
		cw, err := filewatch.New(tw.certPath, func(s string, err error) {
			if err != nil {
				tw.logger.Error(ctx, "leaf-cert-file-watcher", log.Error("error", err))
				return
			}
			if err = tw.loadLeaf(); err != nil {
				tw.logger.Error(ctx, "leaf-cert-file-watcher", log.Error("error", err))
				return
			}
			tw.logger.Info(ctx, "leaf-cert-file-watcher", log.String("info", "leaf cert file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate leaf cert content watcher")
		}

		tw.closers = append(tw.closers, cw.Close)
	}

	if tw.clientCAPath != "" {
		cw, err := filewatch.New(tw.clientCAPath, func(s string, err error) {
			if err != nil {
				tw.logger.Error(ctx, "client-ca-file-watcher", log.Error("error", err))
				return
			}
			if err = tw.loadClientCA(); err != nil {
				tw.logger.Error(ctx, "client-ca-file-watcher", log.Error("error", err))
				return
			}
			tw.logger.Info(ctx, "client-ca-file-watcher", log.String("info", "ca file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate ca content watcher")
		}

		tw.closers = append(tw.closers, cw.Close)
	}

	if tw.rootCAPath != "" {
		cw, err := filewatch.New(tw.rootCAPath, func(s string, err error) {
			if err != nil {
				tw.logger.Error(ctx, "root-ca-file-watcher", log.Error("error", err))
				return
			}
			if err = tw.loadRootCA(); err != nil {
				tw.logger.Error(ctx, "root-ca-file-watcher", log.Error("error", err))
				return
			}
			tw.logger.Info(ctx, "root-ca-file-watcher", log.String("info", "ca file updated"))
		})
		if err != nil {
			return fmt.Errorf("failed to instantiate root ca content watcher")
		}

		tw.closers = append(tw.closers, cw.Close)
	}

	return
}

func (tw *TLSWrap) loadLeaf() (err error) {
	cert, err := tls.LoadX509KeyPair(tw.certPath, tw.keyPath)
	if err != nil {
		return fmt.Errorf("failed to load x509 key pair: %w", err)
	}

	tw.mux.Lock()
	defer tw.mux.Unlock()
	tw.cert.Store(&cert)
	tw.cfgs.Store(tw.createServerConfig())
	tw.cfgc.Store(tw.createClientConfig())
	tw.listenerFunc(tw.cfgs.Load(), tw.cfgc.Load())
	return
}

func (tw *TLSWrap) loadClientCA() (err error) {
	certs, err := os.ReadFile(tw.clientCAPath)
	if err != nil {
		return fmt.Errorf("failed to open ca file: %w", err)
	}

	clientCa := x509.NewCertPool()
	if ok := clientCa.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("failed to append x509 cert pool: %w", err)
	}

	tw.mux.Lock()
	defer tw.mux.Unlock()
	tw.clientCa = clientCa
	tw.cfgs.Store(tw.createServerConfig())
	tw.listenerFunc(tw.cfgs.Load(), tw.cfgc.Load())
	return
}

func (tw *TLSWrap) loadRootCA() (err error) {
	certs, err := os.ReadFile(tw.rootCAPath)
	if err != nil {
		return fmt.Errorf("failed to open ca file: %w", err)
	}

	rootCA, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert file: %w", err)
	}
	if ok := rootCA.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("failed to append x509 cert pool: %w", err)
	}

	tw.mux.Lock()
	defer tw.mux.Unlock()
	tw.rootCA = rootCA
	tw.cfgc.Store(tw.createClientConfig())
	tw.listenerFunc(tw.cfgs.Load(), tw.cfgc.Load())
	return
}

func (tw *TLSWrap) createClientConfig() *tls.Config {
	cfg := tw.cfg.Clone()
	cfg.RootCAs = tw.rootCA.Clone()

	cert := tw.cert.Load()
	if cert == nil {
		return cfg
	}

	cfg.GetClientCertificate = func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return cert, nil
	}
	return cfg
}

func (tw *TLSWrap) createServerConfig() *tls.Config {
	cfg := tw.cfg.Clone()

	cert := tw.cert.Load()
	if cert == nil {
		return cfg
	}

	cfg.ClientCAs = tw.clientCa.Clone()
	cfg.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return cert, nil
	}
	return cfg
}

func (tw *TLSWrap) Dialer(d *net.Dialer) Dialer {
	return dialer{tw: tw, d: d}
}

func (tw *TLSWrap) Listener(l net.Listener) net.Listener {
	return listener{tw: tw, l: l}
}

func (tw *TLSWrap) Close(ctx context.Context) (err error) {
	for _, closer := range tw.closers {
		err = errors.Join(closer(ctx))
	}
	return
}
