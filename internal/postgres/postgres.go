package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/telkomindonesia/go-boilerplate/internal/postgres/internal/sqlc"
	"github.com/telkomindonesia/go-boilerplate/internal/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce/opostgres"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

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

func WithDerivableKeysets(aead *tinkx.DerivableKeyset[tinkx.PrimitiveAEAD], bidx *tinkx.DerivableKeyset[tinkx.PrimitiveBIDX]) OptFunc {
	return func(p *Postgres) (err error) {
		p.aead = aead
		p.bidx = bidx
		return
	}
}

func WithConnString(connStr string) OptFunc {
	return func(p *Postgres) (err error) {
		p.dbUrl = connStr
		p.db, err = otelsql.Open("pgx", connStr)
		return
	}
}

func WithOutboxCERelayFunc(r outboxce.RelayFunc) OptFunc {
	return func(p *Postgres) (err error) {
		p.obceRelay = r
		return
	}
}

func WithOutboxCEManager(m outboxce.Manager) OptFunc {
	return func(p *Postgres) (err error) {
		p.obceManager = m
		return
	}
}

type OptFunc func(*Postgres) error

var _ profile.ProfileRepository = &Postgres{}

type Postgres struct {
	dbUrl string
	db    *sql.DB
	q     *sqlc.Queries
	aead  *tinkx.DerivableKeyset[tinkx.PrimitiveAEAD]
	bidx  *tinkx.DerivableKeyset[tinkx.PrimitiveBIDX]

	obceManager outboxce.Manager
	obceRelay   outboxce.RelayFunc

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
	if p.obceManager == nil {
		p.obceManager, err = opostgres.NewManager(
			opostgres.WithDB(p.db, p.dbUrl),
			opostgres.WithLogger(p.logger),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate outbox manager: %w", err)
		}
	}

	go p.relayOutboxes()

	return p, nil
}

func (p *Postgres) relayOutboxes() {
	ctx, cancel := context.WithCancel(context.Background())
	p.closers = append(p.closers, func(ctx context.Context) error { cancel(); return nil })
	outboxce.RelayLoopWithRetry(ctx, p.obceManager, p.obceRelay, p.logger)
}

func (p *Postgres) aeadFunc(tenantID *uuid.UUID) func() (tinkx.PrimitiveAEAD, error) {
	if tenantID == nil {
		return func() (tinkx.PrimitiveAEAD, error) { return tinkx.PrimitiveAEAD{}, fmt.Errorf("nil Tenant ID") }
	}

	return p.aead.GetPrimitiveFunc(tenantID[:])
}

func (p *Postgres) bidxFunc(tenantID *uuid.UUID) func() (tinkx.PrimitiveBIDX, error) {
	if tenantID == nil {
		return func() (tinkx.PrimitiveBIDX, error) { return tinkx.PrimitiveBIDX{}, fmt.Errorf("nil Tenant ID") }
	}

	return p.bidx.GetPrimitiveFunc(tenantID[:])
}

func (p *Postgres) bidxFullFunc(tenantID *uuid.UUID) func() (tinkx.PrimitiveBIDX, error) {
	if tenantID == nil {
		return func() (tinkx.PrimitiveBIDX, error) { return tinkx.PrimitiveBIDX{}, fmt.Errorf("nil Tenant ID") }
	}

	pb, err := p.bidx.GetPrimitive(tenantID[:])
	if err != nil {
		return func() (tinkx.PrimitiveBIDX, error) { return tinkx.PrimitiveBIDX{}, err }
	}
	b, err := tinkx.BIDXWithLen(pb, 0)
	return func() (tinkx.PrimitiveBIDX, error) { return tinkx.PrimitiveBIDX{BIDX: b}, nil }
}

func (p *Postgres) Close(ctx context.Context) (err error) {
	for _, f := range p.closers {
		err = errors.Join(err, f(ctx))
	}
	return errors.Join(p.db.Close())
}
