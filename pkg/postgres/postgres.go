package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"github.com/tink-crypto/tink-go/v2/tink"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var _ profile.ProfileRepository = &Postgres{}

func WithTracer(name string) OptFunc {
	return func(p *Postgres) (err error) {
		p.tracer = otel.Tracer(name)
		return
	}
}
func WithLogger(l log.Logger) OptFunc {
	return func(p *Postgres) (err error) {
		p.logger = l
		return
	}
}

func WithDerivableKeysets(aead *crypt.DerivableKeyset[crypt.PrimitiveAEAD], mac *crypt.DerivableKeyset[crypt.PrimitiveMAC]) OptFunc {
	return func(p *Postgres) (err error) {
		p.aead = aead
		p.mac = mac
		return
	}
}

func WithConnString(connStr string) OptFunc {
	return func(p *Postgres) (err error) {
		p.dbUrl = connStr
		p.db, err = otelsql.Open("postgres", connStr)
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
	logger log.Logger

	closers []func(context.Context) error
}

func New(opts ...OptFunc) (p *Postgres, err error) {
	p = &Postgres{
		logger:  log.Global(),
		bidxLen: 16,
		tracer:  otel.Tracer("postgres"),
	}
	for _, opt := range opts {
		if err = opt(p); err != nil {
			return p, err
		}
	}

	if p.db == nil {
		return nil, fmt.Errorf("missing db connection")
	}
	if p.aead == nil || p.mac == nil {
		return nil, fmt.Errorf("missing aead or mac keyset")
	}
	if p.logger == nil {
		return nil, fmt.Errorf("missing logger")
	}

	if p.obSender != nil {
		ctx, cancel := context.WithCancel(context.Background())
		p.closers = append(p.closers, func(ctx context.Context) error { cancel(); return nil })
		go p.watchOutboxesLoop(ctx)
	}

	return p, nil
}

func (p *Postgres) getAEAD(tenantID uuid.UUID) (tink.AEAD, error) {
	return p.aead.GetPrimitive(tenantID[:])
}

func (p *Postgres) getMac(tenantID uuid.UUID) (tink.MAC, error) {
	return p.mac.GetPrimitive(tenantID[:])
}

func (p *Postgres) getBlindIdxs(tenantID uuid.UUID, key []byte) (idxs [][]byte, err error) {
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
