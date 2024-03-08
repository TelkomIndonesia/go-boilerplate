package cert

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

type CertWatcher struct {
	fw     util.FileContentWatcher
	logger logger.Logger

	certPath string
	keyPath  string
	cert     *tls.Certificate
}

func NewCertWatcher(keyPath string, certPath string, l logger.Logger) (cw *CertWatcher, err error) {
	cw = &CertWatcher{certPath: certPath, keyPath: keyPath, logger: l}
	if err = cw.loadCert(); err != nil {
		return
	}

	fwcb := func(_ string, err error) {
		if err != nil {
			cw.logger.Error("cert-watcher-error", logger.Any("error", err))
			return
		}
		if err := cw.loadCert(); err != nil {
			cw.logger.Error("load-cert-error", logger.Any("error", err))
		}
	}
	if cw.fw, err = util.NewFileContentWatcher(certPath, fwcb); err != nil {
		return
	}

	return
}

func (cw *CertWatcher) loadCert() error {
	cert, err := tls.LoadX509KeyPair(cw.certPath, cw.keyPath)
	if err != nil {
		return fmt.Errorf("fail to load x509 key pair: %w", err)
	}

	cw.cert = &cert
	return nil
}

func (cw *CertWatcher) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if cw.cert == nil {
			return nil, fmt.Errorf("no certificate was found")
		}
		return cw.cert, nil
	}
}

func (cw *CertWatcher) Close(ctx context.Context) (err error) {
	if cw == nil {
		return
	}

	return cw.fw.Close(ctx)
}
