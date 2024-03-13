package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
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
		p.aead = multiTenantKeyset[primitiveAEAD]{master: aeadKey, constructur: newPrimitiveAEAD}
		if _, err = p.aead.GetPrimitive(uuid.UUID{}); err != nil {
			return fmt.Errorf("fail to verify aead derivable-keyset: %w", err)
		}

		p.mac = multiTenantKeyset[primitiveMAC]{master: macKey, constructur: newPrimitiveMAC}
		if _, err = p.mac.GetPrimitive(uuid.UUID{}); err != nil {
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
		if n < 0 {
			return fmt.Errorf("invalid blind index length")
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
	aead     multiTenantKeyset[primitiveAEAD]
	mac      multiTenantKeyset[primitiveMAC]
	bidxLen  int
	obSender OutboxSender

	tracer trace.Tracer
	logger logger.Logger
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
	return p, nil
}

func (p *Postgres) Close(ctx context.Context) error {
	return p.db.Close()
}
