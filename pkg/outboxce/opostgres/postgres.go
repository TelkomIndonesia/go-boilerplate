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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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

func WithoutDeliveryTracking() OptFunc {
	return func(p *postgres) error {
		p.trackDelivery = false
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

	trackDelivery bool
}

func NewManager(opts ...OptFunc) (outboxce.Manager, error) {
	p := &postgres{
		maxWaitNotif:  time.Minute,
		maxRelaySize:  100,
		channelName:   "outboxce",
		lockID:        keyNameAsHash64("outboxce"),
		logger:        log.Global(),
		tracer:        otel.Tracer("postgres-outboxce"),
		trackDelivery: true,
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

	data := ce.Data()

	ce.SetData(ce.DataContentType(), []byte{})
	cejson, err := ce.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal cloudevent from outbox: %w", err)
	}

	outboxQ := `
		INSERT INTO outboxce 
		(id, attributes, data, created_at, is_delivered)
		VALUES
		($1, $2, $3, $4, $5)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.ID, cejson, data, ob.Time, p.deliveryStatus(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert to outbox: %w", err)
	}

	_, err = tx.ExecContext(ctx, "SELECT pg_notify($1, $2)", p.channelName, strconv.FormatInt(ob.Time.UnixNano(), 10))
	if err != nil {
		p.logger.Warn(ctx, "failed to send notify", log.WithTrace(log.Error("error", err))...)
	}

	return
}

func (p *postgres) deliveryStatus() (status *bool) {
	if !p.trackDelivery {
		return
	}

	t := false
	return &t
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

	notifs, err := p.listen(ctx, p.dbUrl, p.channelName)
	if err != nil {
		return fmt.Errorf("failed to listen to outbox channel: %w", err)
	}
	p.logger.Warn(ctx, "Got channel for outbox notifications")

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
		case event := <-notifs:
			var istr string
			if event != nil {
				istr = event.Payload
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

func (p *postgres) listen(ctx context.Context, dbURL, channelName string) (<-chan *pgconn.Notification, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(ctx, "LISTEN "+channelName)
	if err != nil {
		conn.Close(ctx)
		return nil, err
	}

	ch := make(chan *pgconn.Notification)
	go func() {
		defer close(ch)
		defer conn.Close(ctx)

		for {
			n, err := conn.WaitForNotification(ctx)
			if err != nil {
				return
			}

			select {
			case ch <- n:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (p *postgres) relayOutboxes(ctx context.Context, relayFunc outboxce.RelayFunc) (last outboxce.OutboxCE, err error) {
	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return last, fmt.Errorf("failed to open transaction: %w", errtx)
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
		RETURNING o.id, o.attributes, o.data, o.created_at
	`
	rows, err := tx.QueryContext(ctx, q, p.maxRelaySize)
	if err != nil {
		return last, fmt.Errorf("failed to query outboxes: %w", err)
	}
	defer rows.Close()

	events, outboxes := []event.Event{}, map[string]outboxce.OutboxCE{}
	for rows.Next() {
		var o outboxce.OutboxCE
		var attributes, data []byte
		err = rows.Scan(&o.ID, &attributes, &data, &o.Time)
		if err != nil {
			return last, fmt.Errorf("failed to scan row: %w", err)
		}

		var e event.Event
		if err = json.Unmarshal(attributes, &e); err != nil {
			return last, fmt.Errorf("failed to unmarshal cloud event: %w", err)
		}
		e.SetData(e.DataContentType(), data)

		last = o
		events = append(events, e)
		outboxes[e.ID()] = o
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

	ids := []uuid.UUID{}
	for _, e := range *errRelay {
		id, _ := uuid.Parse(e.Event.ID())
		ids = append(ids, id)
	}
	q := `
		UPDATE outboxce
		SET is_delivered = false 
		WHERE id = ANY($1)
	`
	_, err = tx.ExecContext(ctx, q, pgtype.Array[uuid.UUID]{
		Elements: ids,
		Dims:     []pgtype.ArrayDimension{{Length: int32(len(ids))}},
		Valid:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to unset delivery status: %w", err)
	}

	return
}
