package cert

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

type CAWatcher struct {
	logger logger.Logger
	fw     util.FileContentWatcher

	capath     string
	loadsystem bool

	pool *x509.CertPool
	cfg  *tls.Config
}

func NewCAWatcher(cfg *tls.Config, capath string, loadsystem bool, l logger.Logger) (cw *CAWatcher, err error) {
	cw = &CAWatcher{
		logger:     l,
		capath:     capath,
		cfg:        cfg,
		loadsystem: loadsystem,
	}

	if cw.loadsystem {
		cw.pool, err = x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("fail to load system ca pool: %w", err)
		}
	} else {
		cw.pool = &x509.CertPool{}
	}

	if err = cw.loadCA(); err != nil {
		return nil, fmt.Errorf("fail to load ca: %w", err)
	}

	fwcb := func(_ string, err error) {
		if err != nil {
			cw.logger.Error("cert-watcher-error", logger.Any("error", err))
			return
		}
		if err := cw.loadCA(); err != nil {
			cw.logger.Error("load-cert-error", logger.Any("error", err))
		}
	}
	if cw.fw, err = util.NewFileContentWatcher(capath, fwcb); err != nil {
		return
	}

	return
}

func (cw CAWatcher) loadCA() (err error) {
	pool := cw.pool.Clone()

	certs, err := os.ReadFile(cw.capath)
	if err != nil {
		return fmt.Errorf("failt to load systems ca cert: %w", err)
	}
	if ok := pool.AppendCertsFromPEM(certs); !ok {
		return fmt.Errorf("fail to append x509 cert pool: %w", err)
	}

	cw.cfg.RootCAs = pool
	return nil
}

func (cw *CAWatcher) Close(ctx context.Context) (err error) {
	if cw == nil {
		return
	}

	return cw.fw.Close(ctx)
}
