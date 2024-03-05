package httpserver

import (
	"crypto/tls"
	"fmt"
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type certWatcher struct {
	certMu   sync.RWMutex
	cert     *tls.Certificate
	certPath string
	keyPath  string
	done     chan struct{}
}

func newCertWatcher(keyPath, certPath string) (cw *certWatcher, err error) {
	cw = &certWatcher{
		certPath: certPath,
		keyPath:  keyPath,
		done:     make(chan struct{}),
	}
	if err = cw.loadCert(); err != nil {
		return
	}

	go func() {
		err = cw.watch()
		if err != nil {
			log.Println(err)
		}
	}()
	return
}

func (cw *certWatcher) loadCert() error {
	cert, err := tls.LoadX509KeyPair(cw.certPath, cw.keyPath)
	if err != nil {
		return fmt.Errorf("fail to load x509 key pair: %w", err)
	}

	cw.certMu.Lock()
	defer cw.certMu.Unlock()
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
		case _, ok := <-watcher.Events:
			if !ok {
				return
			}
			if err = cw.loadCert(); err != nil {
				return err
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
		cw.certMu.RLock()
		defer cw.certMu.RUnlock()
		return cw.cert, nil
	}
}
