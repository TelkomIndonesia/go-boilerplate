package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

var _ profile.ProfileRepository = &Postgres{}

func WithTracer(name string) OptFunc {
	return func(p *Postgres) (err error) {
		p.tracer = otel.Tracer(name)
		return
	}
}
func WithLogger(l logger.Logger) OptFunc {
	return func(p *Postgres) (err error) {
		p.logger = l
		return
	}
}

func WithInsecureKeysetFiles(aeadPath string, macPath string) OptFunc {
	return func(p *Postgres) error {
		f, err := os.Open(aeadPath)
		if err != nil {
			return fmt.Errorf("fail to open aead keyset file: %w", err)
		}
		aead, err := insecurecleartextkeyset.Read(keyset.NewJSONReader(f))
		if err != nil {
			return fmt.Errorf("fail to load aead keyset: %w", err)
		}

		f, err = os.Open(macPath)
		if err != nil {
			return fmt.Errorf("fail to open mac keyset file: %w", err)
		}
		mac, err := insecurecleartextkeyset.Read(keyset.NewJSONReader(f))
		if err != nil {
			return fmt.Errorf("fail to load mac keyset: %w", err)
		}

		return WithKeysets(aead, mac)(p)
	}
}

func WithKeysets(aeadKey *keyset.Handle, macKey *keyset.Handle) OptFunc {
	return func(p *Postgres) (err error) {
		p.aead = crypt.NewDerivableKeySet(aeadKey, crypt.NewPrimitiveAEAD)
		if _, err = p.aead.GetPrimitive(nil); err != nil {
			return fmt.Errorf("fail to verify aead derivable-keyset: %w", err)
		}

		p.mac = crypt.NewDerivableKeySet(macKey, crypt.NewPrimitiveMAC)
		if _, err = p.mac.GetPrimitive(nil); err != nil {
			return fmt.Errorf("fail to create mac derivable-keyset: %w", err)
		}
		return nil
	}
}

func WithConnString(connStr string) OptFunc {
	return func(p *Postgres) (err error) {
		p.dbUrl = connStr
		p.db, err = sql.Open("postgres", connStr)
		return
	}
}

func WithBlindIdxLen(n int) OptFunc {
	return func(p *Postgres) error {
		if n < 8 {
			return fmt.Errorf("length must be greater than equal 8")
		}
		p.bidxLen = n
		return nil
	}
}

func WithOutboxSender(f OutboxSender) OptFunc {
	return func(p *Postgres) error {
		p.obSender = f
		return nil
	}
}

type OptFunc func(*Postgres) error

type Postgres struct {
	dbUrl    string
	db       *sql.DB
	aead     *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
	mac      *crypt.DerivableKeyset[crypt.PrimitiveMAC]
	bidxLen  int
	obSender OutboxSender

	tracer trace.Tracer
	logger logger.Logger

	closers []func(context.Context) error
}

func New(opts ...OptFunc) (p *Postgres, err error) {
	p = &Postgres{
		logger:  logger.Global(),
		bidxLen: 16,
		tracer:  otel.Tracer("postgres"),
	}
	for _, opt := range opts {
		if err = opt(p); err != nil {
			return p, err
		}
	}

	if p.obSender != nil {
		ctx, cancel := context.WithCancel(context.Background())
		p.closers = append(p.closers, func(ctx context.Context) error { cancel(); return nil })
		go p.watchOutboxesLoop(ctx)
	}

	return p, nil
}

func (p *Postgres) GetBlindIdxKeys(tenantID uuid.UUID, key []byte) (idxs [][]byte, err error) {
	h, err := p.mac.GetHandle(tenantID[:])
	if err != nil {
		return nil, fmt.Errorf("fail to get keyset handle for tenant %s: %w", tenantID, err)
	}
	return crypt.GetBlindIdxs(h, key, p.bidxLen)
}

func (p *Postgres) Close(ctx context.Context) (err error) {
	for _, f := range p.closers {
		err = errors.Join(err, f(ctx))
	}
	return errors.Join(p.db.Close())
}
