package httpserver

import (
	"crypto/tls"
	"fmt"

	"github.com/telkomindonesia/go-boilerplate/pkg/util"
)

type certWatcher struct {
	cert     *tls.Certificate
	certPath string
	keyPath  string

	fw util.FileWatcher
}

func newCertWatcher(keyPath string, certPath string, logger func(error)) (cw *certWatcher, err error) {
	if logger == nil {
		logger = func(err error) {}
	}

	cw = &certWatcher{certPath: certPath, keyPath: keyPath}
	if err = cw.loadCert(); err != nil {
		return
	}

	fwcb := func() {
		if err := cw.loadCert(); err != nil {
			logger(err)
		}
	}
	if cw.fw, err = util.NewFileWatcher(certPath, fwcb, logger); err != nil {
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
