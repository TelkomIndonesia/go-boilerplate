package httpserver

import (
	"crypto/tls"
	"fmt"
	"time"

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

	go cw.watchLoop()
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

func (cw *certWatcher) watchLoop() {
	for {
		select {
		case <-cw.done:
			return

		default:
		}

		if err := cw.watch(); err != nil {
			cw.log(fmt.Errorf("cert watcher stopped due to error: %w", err))
			<-time.After(time.Minute)
		}
	}

}

func (cw *certWatcher) watch() (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fail to instantiate fsnotify watcher: %w", err)
	}
	defer watcher.Close()

	err = watcher.Add(cw.certPath)
	if err != nil {
		return fmt.Errorf("fail to watch %s: %w", cw.certPath, err)
	}

	for {
		var event fsnotify.Event

		select {
		case <-cw.done:
			return

		case err, ok := <-watcher.Errors:
			if !ok {
				return err
			}
			cw.log(fmt.Errorf("cert-watcher receives error event: %w", err))
			continue

		case e, ok := <-watcher.Events:
			if !ok {
				return
			}

			event = e
		}

		switch {
		default:
			continue

		case event.Has(fsnotify.Write):
		case event.Has(fsnotify.Remove) || event.Has(fsnotify.Chmod):
			if err := watcher.Remove(cw.certPath); err != nil {
				return fmt.Errorf("fail to remove watched file: %w", err)
			}
			if err := watcher.Add(cw.certPath); err != nil {
				return fmt.Errorf("fail to re-add watched file: %w", err)
			}
		}

		err = cw.loadCert()
		if err != nil {
			cw.log(fmt.Errorf("fail to reload certificate: %w", err))
		}
	}
}

func (cw *certWatcher) close() (err error) {
	if cw == nil {
		return
	}

	close(cw.done)
	return
}

func (cw *certWatcher) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return cw.cert, nil
	}
}
