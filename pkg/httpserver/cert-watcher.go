package httpserver

import (
	"crypto/tls"
	"fmt"

	"github.com/fsnotify/fsnotify"
)

type certWatcher struct {
	cert     *tls.Certificate
	certPath string
	keyPath  string

	log  func(error)
	done chan struct{}
}

func newCertWatcher(keyPath string, certPath string, logger func(error)) (cw *certWatcher, err error) {
	if logger == nil {
		logger = func(err error) {}
	}

	cw = &certWatcher{
		certPath: certPath,
		keyPath:  keyPath,
		log:      logger,
		done:     make(chan struct{}),
	}
	if err = cw.loadCert(); err != nil {
		return
	}

	go func() {
		err = cw.watch()
		if err != nil {
			cw.log(err)
		}
	}()
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

func (cw *certWatcher) watch() (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fail to instantiate watcher: %w", err)
	}
	defer watcher.Close()

	err = watcher.Add(cw.certPath)
	if err != nil {
		return fmt.Errorf("fail to watch %s: %w", cw.certPath, err)
	}

	for {
		select {
		case e, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !e.Has(fsnotify.Write) {
				return
			}

			err = cw.loadCert()
			if err != nil {
				cw.log(fmt.Errorf("fail to reload certificate: %w", err))
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return err
			}

		case <-cw.done:
			return
		}
	}
}

func (cw *certWatcher) close() (err error) {
	if cw == nil {
		return
	}

	select {
	case cw.done <- struct{}{}:
	default:
	}

	return
}

func (cw *certWatcher) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return cw.cert, nil
	}
}
