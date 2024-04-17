package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres/internal/sqlc"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
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

func WithDerivableKeysets(aead *crypt.DerivableKeyset[crypt.PrimitiveAEAD], bidx *crypt.DerivableKeyset[crypt.PrimitiveBIDX]) OptFunc {
	return func(p *Postgres) (err error) {
		p.aead = aead
		p.bidx = bidx
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
	q        *sqlc.Queries
	aead     *crypt.DerivableKeyset[crypt.PrimitiveAEAD]
	bidx     *crypt.DerivableKeyset[crypt.PrimitiveBIDX]
	obSender OutboxSender

	tracer trace.Tracer
	logger log.Logger

	closers []func(context.Context) error
}

func New(opts ...OptFunc) (p *Postgres, err error) {
	p = &Postgres{
		logger: log.Global(),
		tracer: otel.Tracer("postgres"),
	}
	for _, opt := range opts {
		if err = opt(p); err != nil {
			return p, err
		}
	}

	if p.db == nil {
		return nil, fmt.Errorf("missing db connection")
	}
	p.q = sqlc.New(p.db)
	if p.aead == nil || p.bidx == nil {
		return nil, fmt.Errorf("missing aead or bidx primitive")
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

func (p *Postgres) aeadFunc(tenantID uuid.UUID) func() (crypt.PrimitiveAEAD, error) {
	return p.aead.GetPrimitiveFunc(tenantID[:])
}

func (p *Postgres) bidxFunc(tenantID uuid.UUID) func() (crypt.PrimitiveBIDX, error) {
	return p.bidx.GetPrimitiveFunc(tenantID[:])
}

func (p *Postgres) Close(ctx context.Context) (err error) {
	for _, f := range p.closers {
		err = errors.Join(err, f(ctx))
	}
	return errors.Join(p.db.Close())
}
