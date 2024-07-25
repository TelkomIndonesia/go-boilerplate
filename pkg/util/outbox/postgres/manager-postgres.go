package postgres

import (
	"context"
	"database/sql"

	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outbox"
	"github.com/tink-crypto/tink-go/v2/tink"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var outboxChannel = "outbox"
var outboxLock = keyNameAsHash64("outbox")

var _ outbox.Manager = &postgres{}

type OptFunc func(*postgres) error

func WithDB(db *sql.DB, url string) OptFunc {
	return func(p *postgres) error {
		p.db = db
		p.dbUrl = url
		return nil
	}
}

func WithTenantAEAD(aead *crypt.DerivableKeyset[crypt.PrimitiveAEAD]) OptFunc {
	return func(p *postgres) error {
		p.aeadFunc = func(ob outbox.Outbox[any]) (tink.AEAD, error) {
			return aead.GetPrimitive(ob.TenantID[:])
		}
		return nil
	}
}

func WithAEADFunc(aeadFunc outbox.AEADFunc) OptFunc {
	return func(p *postgres) error {
		p.aeadFunc = aeadFunc
		return nil
	}
}

func WithMaxIdle(d time.Duration) OptFunc {
	return func(p *postgres) error {
		p.maxIdle = d
		return nil
	}
}

func WithLogger(l log.Logger) OptFunc {
	return func(p *postgres) error {
		p.logger = l
		return nil
	}
}

type postgres struct {
	dbUrl string
	db    *sql.DB

	aeadFunc outbox.AEADFunc

	maxIdle time.Duration
	limit   int

	channelName string
	lockID      uint64
	tracer      trace.Tracer
	logger      log.Logger
}

func New(opts ...OptFunc) (outbox.Manager, error) {
	p := &postgres{
		maxIdle:     time.Minute,
		limit:       100,
		channelName: "outbox",
		lockID:      keyNameAsHash64("outbox"),
		logger:      log.Global(),
		tracer:      otel.Tracer("postgres-outbox"),
		aeadFunc: func(ob outbox.Outbox[any]) (tink.AEAD, error) {
			return nil, fmt.Errorf("nil aead primitive")
		},
	}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("fail to apply options: %w", err)
		}
	}

	if p.dbUrl == "" {
		return nil, fmt.Errorf("db url is required")
	}
	if p.db == nil {
		return nil, fmt.Errorf("db connection is required")
	}

	return p, nil
}

func (p *postgres) StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob outbox.Outbox[any]) (err error) {
	aead, err := p.aeadFunc(ob)
	if err != nil {
		return fmt.Errorf("fail to load encryption primitive: %w", err)
	}

	b, err := msgpack.Marshal(ob.Content)
	if err != nil {
		return fmt.Errorf("fail to marshal content")
	}

	ob.Content, err = aead.Encrypt(b, ob.ID[:])
	if err != nil {
		return fmt.Errorf("fail to encrypt outbox: %w", err)
	}

	ob.IsEncrypted = true
	return p.storeOutbox(ctx, tx, ob)
}

func (p *postgres) StoreOutbox(ctx context.Context, tx *sql.Tx, ob outbox.Outbox[any]) (err error) {
	ob.Content, err = msgpack.Marshal(ob.Content)
	if err != nil {
		return fmt.Errorf("fail to marshal content")
	}

	ob.IsEncrypted = false
	return p.storeOutbox(ctx, tx, ob)
}

func (p *postgres) storeOutbox(ctx context.Context, tx *sql.Tx, ob outbox.Outbox[any]) (err error) {
	_, span := p.tracer.Start(ctx, "storeOutbox", trace.WithAttributes(
		attribute.Stringer("tenantID", ob.TenantID),
		attribute.Stringer("id", ob.ID),
		attribute.String("eventName", ob.EventName),
		attribute.String("contentType", ob.ContentType),
	))
	defer span.End()

	outboxQ := `
		INSERT INTO outbox 
		(id, tenant_id, content_type, content, event_name, is_encrypted, created_at)
		VALUES
		($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.ID, ob.TenantID, ob.ContentType, ob.Content, ob.EventName, ob.IsEncrypted, ob.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to outbox: %w", err)
	}

	_, err = tx.ExecContext(ctx, "SELECT pg_notify($1, $2)", p.channelName, ob.CreatedAt.UnixNano())
	if err != nil {
		p.logger.Warn("fail to send notify", log.Error("error", err), log.TraceContext("trace-id", ctx))
	}

	return
}

func (p *postgres) ObserveOutboxes(ctx context.Context, relayer outbox.Relay) (err error) {
	if relayer == nil {
		p.logger.Info("not outbox sender, will do nothing.")
		<-ctx.Done()
		return
	}

	unlocker, err := p.lock(ctx)
	if err != nil {
		return fmt.Errorf("fail to obtain lock: %w", err)
	}
	defer unlocker()

	l := pq.NewListener(p.dbUrl, time.Second, time.Minute, func(event pq.ListenerEventType, err error) { return })
	if err = l.Listen(p.channelName); err != nil {
		return fmt.Errorf("fail to listen for outbox notification :%w", err)
	}
	defer l.Close()

	var last outbox.Outbox[outbox.Serialized]
	for {
		timer := time.NewTimer(p.maxIdle)
		stopper := func() {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}

		select {
		case <-ctx.Done():
			return

		case <-timer.C:
		case event := <-l.NotificationChannel():
			var istr string
			if event != nil {
				istr = event.Extra
			}
			i, _ := strconv.ParseInt(istr, 10, 64)
			if i < last.CreatedAt.UnixNano() {
				stopper()
				continue
			}
		}

		last, err = p.relayOutboxes(ctx, relayer, p.limit)
		if err != nil {
			p.logger.Error("fail to relay outboxes", log.Error("error", err), log.TraceContext("trace-id", ctx))
		}

		stopper()
	}
}

func (p *postgres) lock(ctx context.Context) (unlocker func(), err error) {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to obtain connection for lock: %w", err)
	}
	defer conn.Close()

	obtain := false
	err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, p.lockID).Scan(&obtain)
	if !obtain {
		return nil, fmt.Errorf("lock has been obtained by other process")
	}
	if err != nil {
		return nil, fmt.Errorf("fail to obtain lock: %w", err)
	}

	return func() {
		conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", p.lockID)
	}, nil
}

func (p *postgres) relayOutboxes(ctx context.Context, relay outbox.Relay, limit int) (last outbox.Outbox[outbox.Serialized], err error) {
	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return last, fmt.Errorf("fail to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	q := `
		WITH cte AS (
			SELECT id FROM outbox
			WHERE is_delivered = false 
			ORDER BY created_at
			LIMIT $1
		)
		UPDATE outbox o 
		SET is_delivered = true 
		FROM cte
		WHERE o.id = cte.id
		RETURNING o.id, o.tenant_id, o.content_type, o.content, o.event_name, o.is_encrypted, o.created_at
	`
	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return last, fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	outboxes := []outbox.Outbox[outbox.Serialized]{}
	for rows.Next() {
		var o outbox.Outbox[any]
		var content []byte
		err = rows.Scan(&o.ID, &o.TenantID, &o.ContentType, &content, &o.EventName, &o.IsEncrypted, &o.CreatedAt)
		if err != nil {
			return last, fmt.Errorf("fail to scan row: %w", err)
		}

		cb := serialized{b: content}
		if o.IsEncrypted {
			cb.ad = o.ID[:]
			cb.aead, err = p.aeadFunc(o)
			if err != nil {
				return last, fmt.Errorf("fail to load encryption primitive: %w", err)
			}
		}
		os := outbox.Outbox[outbox.Serialized]{
			ID:          o.ID,
			TenantID:    o.TenantID,
			EventName:   o.EventName,
			ContentType: o.ContentType,
			Content:     cb.Serialized(),
			IsEncrypted: o.IsEncrypted,
			CreatedAt:   o.CreatedAt,
		}

		outboxes = append(outboxes, os)
	}

	if len(outboxes) == 0 {
		return last, tx.Rollback()
	}

	if err = relay(ctx, outboxes); err != nil {
		return last, fmt.Errorf("fail to relay outboxes: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return last, fmt.Errorf("fail to commit: %w", err)
	}

	return outboxes[len(outboxes)-1], err
}
