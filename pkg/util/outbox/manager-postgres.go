package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"github.com/tink-crypto/tink-go/v2/tink"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var outboxChannel = "outbox"
var outboxLock = keyNameAsHash64("outbox")

func keyNameAsHash64(keyName string) uint64 {
	hash := fnv.New64()
	if _, err := hash.Write([]byte(keyName)); err != nil {
		panic(err)
	}
	return hash.Sum64()
}

var _ Manager = &postgres{}

type ManagerPostgresOptFunc func(*postgres) error

func ManagerPostgresWithDB(db *sql.DB, url string) ManagerPostgresOptFunc {
	return func(p *postgres) error {
		p.db = db
		p.dbUrl = url
		return nil
	}
}

func ManagerPostgresWithSender(sender Sender) ManagerPostgresOptFunc {
	return func(p *postgres) error {
		p.sender = sender
		return nil
	}
}

func ManagerPostgresWithTenantAEAD(aead *crypt.DerivableKeyset[crypt.PrimitiveAEAD]) ManagerPostgresOptFunc {
	return func(p *postgres) error {
		p.aeadFunc = func(ob Outbox) (tink.AEAD, error) {
			return aead.GetPrimitive(ob.TenantID[:])
		}
		return nil
	}
}

func ManagerPostgresWithAEADFunc(aeadFunc AEADFunc) ManagerPostgresOptFunc {
	return func(p *postgres) error {
		p.aeadFunc = aeadFunc
		return nil
	}
}

func ManagerPostgresWithLogger(l log.Logger) ManagerPostgresOptFunc {
	return func(p *postgres) error {
		p.logger = l
		return nil
	}
}

type postgres struct {
	dbUrl string
	db    *sql.DB

	sender   Sender
	aeadFunc AEADFunc

	maxIdle time.Duration
	limit   int

	channelName string
	lockID      uint64
	tracer      trace.Tracer
	logger      log.Logger
}

func NewManagerPostgres(opts ...ManagerPostgresOptFunc) (Manager, error) {
	p := &postgres{
		maxIdle:     time.Minute,
		limit:       100,
		channelName: outboxChannel,
		lockID:      outboxLock,
		logger:      log.Global(),
		tracer:      otel.Tracer("postgres-outbox"),
		aeadFunc: func(ob Outbox) (tink.AEAD, error) {
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

func (p *postgres) StoreOutboxEncrypted(ctx context.Context, tx *sql.Tx, ob Outbox) (err error) {
	aead, err := p.aeadFunc(ob)
	if err != nil {
		return fmt.Errorf("fail to load encryption primitive: %w", err)
	}

	o, err := ob.AsEncrypted(aead)
	if err != nil {
		return fmt.Errorf("fail to encrypt outbox")
	}

	return p.StoreOutbox(ctx, tx, o)
}

func (p *postgres) StoreOutbox(ctx context.Context, tx *sql.Tx, ob Outbox) (err error) {
	_, span := p.tracer.Start(ctx, "storeOutbox", trace.WithAttributes(
		attribute.Stringer("tenantID", ob.TenantID),
		attribute.Stringer("id", ob.ID),
		attribute.String("contentType", ob.ContentType),
	))
	defer span.End()

	content, err := json.Marshal(ob.Content)
	if err != nil {
		return fmt.Errorf("fail to marshal content")
	}
	outboxQ := `
		INSERT INTO outbox 
		(id, tenant_id, type, content, event, is_encrypted, created_at)
		VALUES
		($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.ID, ob.TenantID, ob.ContentType, content, ob.Event, ob.IsEncrypted, ob.CreatedAt,
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

func (p *postgres) WatchOuboxes(ctx context.Context) (err error) {
	if p.sender == nil {
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

	var last Outbox
	for {
		timer := time.NewTimer(p.maxIdle)
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
				continue
			}
		}

		last, err = p.sendOutboxes(ctx, p.limit)
		if err != nil {
			p.logger.Error("fail to send outboxes", log.Error("error", err), log.TraceContext("trace-id", ctx))
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
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

func (p *postgres) sendOutboxes(ctx context.Context, limit int) (last Outbox, err error) {
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
		RETURNING o.id, o.tenant_id, o.type, o.content, o.event, o.is_encrypted, o.created_at
	`
	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return last, fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	outboxes := []Outbox{}
	for rows.Next() {
		o := Outbox{}
		err = rows.Scan(&o.ID, &o.TenantID, &o.ContentType, &o.contentByte, &o.Event, &o.IsEncrypted, &o.CreatedAt)
		if err != nil {
			return last, fmt.Errorf("fail to scan row: %w", err)
		}

		switch o.IsEncrypted {
		case true:
			o.aead, err = p.aeadFunc(o)
			if err != nil {
				return last, fmt.Errorf("fail to load encryption primitive: %w", err)
			}

			var content []byte
			err = json.Unmarshal(o.contentByte, &content)
			if err != nil {
				return last, fmt.Errorf("fail to unmarshall content")
			}
			o.Content = content

		case false:
			o.Content = map[string]interface{}{}
			err = json.Unmarshal(o.contentByte, &o.Content)
			if err != nil {
				return last, fmt.Errorf("fail to unmarshall content")
			}
		}

		outboxes = append(outboxes, o)
	}

	if len(outboxes) == 0 {
		return last, tx.Rollback()
	}

	if err = p.sender(ctx, outboxes); err != nil {
		return last, fmt.Errorf("fail to send outboxes: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return last, fmt.Errorf("fail to commit: %w", err)
	}

	return outboxes[len(outboxes)-1], err
}
