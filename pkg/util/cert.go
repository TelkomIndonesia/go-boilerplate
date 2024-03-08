package util

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/telkomindonesia/go-boilerplate/pkg/logger"
)

func WithAutoReloadCA(cfg *tls.Config, ca string, l logger.Logger) (err error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failt to load systems x509 cert pool:%w", err)
	}

	appendCA := func() error {
		certs, err := os.ReadFile(ca)
		if err != nil {
			return fmt.Errorf("failt to load systems ca cert :%w", err)
		}
		if ok := pool.AppendCertsFromPEM(certs); !ok {
			return fmt.Errorf("fail to append x509 cert pool: %w", err)
		}
		cfg.RootCAs = pool
		return nil
	}
	if err = appendCA(); err != nil {
		return
	}

	NewFileContentWatcher(ca, func(_ string, err error) {
		if err != nil {
			l.Error("ca-watcher-error", logger.Any("error", err))
			return
		}

		if err = appendCA(); err != nil {
			l.Error("append-ca-error", logger.Any("error", err))
		}
	})

	return
}
