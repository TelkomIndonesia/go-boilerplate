package httpserver

import (
	"crypto/tls"
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

type certWatcher struct {
	cert     *tls.Certificate
	certPath string
	keyPath  string
	logger   logger.Logger

	fw util.FileWatcher
}

func newCertWatcher(keyPath string, certPath string, l logger.Logger) (cw *certWatcher, err error) {

	cw = &certWatcher{certPath: certPath, keyPath: keyPath, logger: l}
	if err = cw.loadCert(); err != nil {
		return
	}

	fwcb := func(err error) {
		if err != nil {
			cw.logger.Error("cert-watcher-error", logger.Any("error", err))
		}
		if err := cw.loadCert(); err != nil {
			cw.logger.Error("load-cert-error", logger.Any("error", err))
		}
	}
	if cw.fw, err = util.NewFileWatcher(certPath, fwcb); err != nil {
		return
	}

	return
}

func (cw *certWatcher) loadCert() error {
	cert, err := tls.LoadX509KeyPair(cw.certPath, cw.keyPath)
	if err != nil {
		return fmt.Errorf("fail to load x509 key pair: %w", err)
	}

	cw.cert = &cert
	return nil
}

func (cw *certWatcher) Close() (err error) {
	if cw == nil {
		return
	}

	return cw.fw.Close()
}

func (cw *certWatcher) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return cw.cert, nil
	}
}
