package opostgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"fmt"
	"strconv"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var _ outboxce.Manager = &postgres{}

type OptFunc func(*postgres) error

func WithDB(db *sql.DB, url string) OptFunc {
	return func(p *postgres) error {
		p.db = db
		p.dbUrl = url
		return nil
	}
}

func WithMaxWaitNotif(d time.Duration) OptFunc {
	return func(p *postgres) error {
		p.maxWaitNotif = d
		return nil
	}
}

func WithMaxRelaySize(d int) OptFunc {
	return func(p *postgres) error {
		p.maxRelaySize = d
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

	maxWaitNotif time.Duration
	maxRelaySize int

	channelName string
	lockID      int64
	tracer      trace.Tracer
	logger      log.Logger
}

func NewManager(opts ...OptFunc) (outboxce.Manager, error) {
	p := &postgres{
		maxWaitNotif: time.Minute,
		maxRelaySize: 100,
		channelName:  "outboxce",
		lockID:       keyNameAsHash64("outboxce"),
		logger:       log.Global(),
		tracer:       otel.Tracer("postgres-outboxce"),
	}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("failed to apply options: %w", err)
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

func (p *postgres) Store(ctx context.Context, tx *sql.Tx, ob outboxce.OutboxCE) (err error) {
	ce, err := ob.Build()
	if err != nil {
		return fmt.Errorf("failed to build cloudevent :%w", err)
	}

	cejson, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("failed to marshal cloudevent from outbox: %w", err)
	}

	outboxQ := `
		INSERT INTO outboxce 
		(id, tenant_id, cloud_event, created_at)
		VALUES
		($1, $2, $3, $4)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.ID, ob.TenantID, cejson, ob.Time,
	)
	if err != nil {
		return fmt.Errorf("failed to insert to outbox: %w", err)
	}

	_, err = tx.ExecContext(ctx, "SELECT pg_notify($1, $2)", p.channelName, ob.Time.UnixNano())
	if err != nil {
		p.logger.Warn(ctx, "failed to send notify", log.WithTrace(log.Error("error", err))...)
	}

	return
}

func (p *postgres) RelayLoop(ctx context.Context, relayFunc outboxce.RelayFunc) (err error) {
	if relayFunc == nil {
		p.logger.Warn(ctx, "No outbox sender, will do nothing.")
		<-ctx.Done()
		return
	}

	unlocker, err := p.lock(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain lock: %w", err)
	}
	defer unlocker()
	p.logger.Warn(ctx, "Got lock for observing outbox")

	l := pq.NewListener(p.dbUrl, time.Second, time.Minute, func(event pq.ListenerEventType, err error) { return })
	if err = l.Listen(p.channelName); err != nil {
		return fmt.Errorf("failed to listen for outbox notification :%w", err)
	}
	defer l.Close()

	var last outboxce.OutboxCE
	for {
		timer := time.NewTimer(p.maxWaitNotif)
		stopTimer := func() {
			if timer.Stop() {
				return
			}
			select {
			case <-timer.C:
			default:
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
			if i < last.Time.UnixNano() {
				stopTimer()
				continue
			}
		}

		last, err = p.relayOutboxes(ctx, relayFunc)
		if err != nil {
			p.logger.Error(ctx, "failed to relay outboxes", log.WithTrace(log.Error("error", err))...)
		}

		stopTimer()
	}
}

func (p *postgres) lock(ctx context.Context) (unlocker func(), err error) {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain connection for lock: %w", err)
	}
	defer func() {
		if conn != nil && err != nil {
			conn.Close()
		}
	}()

	obtain := false
	err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, p.lockID).Scan(&obtain)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain lock: %w", err)
	}
	if !obtain {
		return nil, fmt.Errorf("lock has been obtained by other process")
	}

	return func() {
		conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", p.lockID)
		conn.Close()
	}, nil
}

func (p *postgres) relayOutboxes(ctx context.Context, relayFunc outboxce.RelayFunc) (last outboxce.OutboxCE, err error) {
	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return last, fmt.Errorf("failed to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	q := `
		WITH cte AS (
			SELECT id FROM outboxce
			WHERE is_delivered = false 
			ORDER BY created_at
			LIMIT $1
		)
		UPDATE outboxce o 
		SET is_delivered = true 
		FROM cte
		WHERE o.id = cte.id
		RETURNING o.id, o.tenant_id, o.cloud_event, o.created_at
	`
	rows, err := tx.QueryContext(ctx, q, p.maxRelaySize)
	if err != nil {
		return last, fmt.Errorf("failed to query outboxes: %w", err)
	}
	defer rows.Close()

	events := []event.Event{}
	for rows.Next() {
		var o outboxce.OutboxCE
		var data []byte
		err = rows.Scan(&o.ID, &o.TenantID, &data, &o.Time)
		if err != nil {
			return last, fmt.Errorf("failed to scan row: %w", err)
		}

		var e event.Event
		if err = json.Unmarshal(data, &e); err != nil {
			return last, fmt.Errorf("failed to unmarshal cloud event: %w", err)
		}

		last = o
		events = append(events, e)
	}

	if len(events) == 0 {
		return last, tx.Rollback()
	}

	if err = p.relayWithRelayErrorsHandler(ctx, tx, relayFunc, events); err != nil {
		return last, fmt.Errorf("failed to relay outboxes with handler: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return last, fmt.Errorf("failed to commit: %w", err)
	}

	return
}

func (p *postgres) relayWithRelayErrorsHandler(ctx context.Context, tx *sql.Tx, relayFunc outboxce.RelayFunc, events []event.Event) (err error) {
	err = relayFunc(ctx, events)

	var errRelay = &outboxce.RelayErrors{}
	if !errors.As(err, &errRelay) {
		return err
	}
	if len(*errRelay) == 0 {
		return nil
	}

	p.logger.Warn(ctx, "got partial relay error", log.WithTrace(log.Error("error", err))...)

	ids := []string{}
	for _, e := range *errRelay {
		ids = append(ids, e.Event.ID())
	}
	q := `
		UPDATE outboxce
		SET is_delivered = false 
		WHERE id = ANY($1)
	`
	_, err = tx.ExecContext(ctx, q, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("failed to unset delivery status: %w", err)
	}

	return
}
