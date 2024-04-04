package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
	"github.com/tink-crypto/tink-go/v2/tink"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type OutboxSender func(context.Context, []*Outbox) error

type Outbox struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	Event       string    `json:"event"`
	Content     any       `json:"content"`
	IsEncrypted bool      `json:"is_encrypted"`

	aead        tink.AEAD
	contentByte []byte
}

func newOutbox(tid uuid.UUID, event string, ctype string, content any) (o *Outbox, err error) {
	o = &Outbox{
		TenantID:    tid,
		Event:       event,
		ContentType: ctype,
		CreatedAt:   time.Now(),
		Content:     content,
	}
	o.ID, err = uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("fail to create new id for outbox: %w", err)
	}
	return
}

func (p *Postgres) newOutbox(tid uuid.UUID, event string, ctype string, content any) (o *Outbox, err error) {
	o, err = newOutbox(tid, ctype, event, content)
	if err != nil {
		return
	}

	o.aead, err = p.aead.GetPrimitive(o.TenantID[:])
	if err != nil {
		return
	}

	return
}

func (p *Postgres) newOutboxEncrypted(tid uuid.UUID, event string, ctype string, content any) (o *Outbox, err error) {
	o, err = p.newOutbox(tid, ctype, event, content)
	if err != nil {
		return
	}

	*o, err = o.AsEncrypted()
	return
}

func (ob Outbox) AsEncrypted() (o Outbox, err error) {
	if ob.IsEncrypted {
		return ob, nil
	}

	if ob.aead == nil {
		return o, fmt.Errorf("can't encrypt due to nil encryptor")
	}

	b, err := json.Marshal(ob.Content)
	if err != nil {
		return o, fmt.Errorf("fail to marshal content")
	}

	ob.Content, err = ob.aead.Encrypt(b, ob.ID[:])
	if err != nil {
		return o, fmt.Errorf("fail to encrypt outbox: %w", err)
	}

	ob.IsEncrypted = true
	return ob, nil
}

func (ob Outbox) AsUnEncrypted() (o Outbox, err error) {
	if !ob.IsEncrypted {
		return ob, nil
	}

	if ob.aead == nil {
		return o, fmt.Errorf("can't decrypt due to nil decryptor")
	}

	content, ok := ob.Content.([]byte)
	if !ok {
		return o, fmt.Errorf("not a byte string of chipertext")
	}
	ob.contentByte, err = ob.aead.Decrypt(content, ob.ID[:])
	if err != nil {
		return o, fmt.Errorf("fail to decrypt encrypted outbox: %w", err)
	}
	err = json.Unmarshal(ob.contentByte, &ob.Content)
	if err != nil {
		return o, fmt.Errorf("fail to unmarshal encrypted outbox: %w", err)
	}

	ob.IsEncrypted = false
	return ob, nil
}

func (o Outbox) ContentByte() []byte {
	return o.contentByte
}

var outboxChannel = "outbox"
var outboxLock = keyNameAsHash64("outbox")

func (p *Postgres) storeOutbox(ctx context.Context, tx *sql.Tx, ob *Outbox) (err error) {
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

	_, err = tx.QueryContext(ctx, "SELECT pg_notify($1, $2)", outboxChannel, ob.CreatedAt.UnixNano())
	if err != nil {
		p.logger.Warn("fail to send notify", log.Error("error", err), log.TraceContext("trace-id", ctx))
	}

	return
}

func (p *Postgres) watchOutboxesLoop(ctx context.Context) (err error) {
	for {
		if err := p.watchOuboxes(ctx); err != nil {
			p.logger.Warn("got outbox watcher error", log.Error("error", err), log.TraceContext("trace-id", ctx))
		}

		select {
		case <-ctx.Done():
			return

		case <-time.After(time.Minute):
		}
	}
}

func (p *Postgres) watchOuboxes(ctx context.Context) (err error) {
	obtain := false
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("fail to obtain connection for lock: %w", err)
	}
	defer conn.Close()
	err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, outboxLock).Scan(&obtain)
	if !obtain {
		return fmt.Errorf("lock has been obtained by other process")
	}
	if err != nil {
		return fmt.Errorf("fail to obtain lock: %w", err)
	}

	l := pq.NewListener(p.dbUrl, time.Second, time.Minute, func(event pq.ListenerEventType, err error) { return })
	if err = l.Listen(outboxChannel); err != nil {
		return fmt.Errorf("fail to listen for outbox notification :%w", err)
	}
	defer l.Close()

	var last *Outbox
	for {
		timer := time.NewTimer(time.Minute)
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
		case event := <-l.NotificationChannel():
			i, _ := strconv.ParseInt(event.Extra, 10, 64)
			if last != nil && i < last.CreatedAt.UnixNano() {
				continue
			}
		}

		last, err = p.sendOutbox(ctx, 100)
		if err != nil {
			p.logger.Error("fail to send outboxes", log.Error("error", err), log.TraceContext("trace-id", ctx))
		}

		if !timer.Stop() {
			<-timer.C
		}
	}
}

func (p *Postgres) sendOutbox(ctx context.Context, limit int) (last *Outbox, err error) {
	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return nil, fmt.Errorf("fail to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	q := `
		WITH cte AS (
			SELECT id FROM outbox
			WHERE is_delivered = false ORDER BY created_at
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
		return nil, fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	outboxes := []*Outbox{}
	for rows.Next() {
		o := &Outbox{}
		err = rows.Scan(&o.ID, &o.TenantID, &o.ContentType, &o.contentByte, &o.Event, &o.IsEncrypted, &o.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("fail to scan row: %w", err)
		}
		switch o.IsEncrypted {
		case false:
			o.Content = map[string]interface{}{}
			err = json.Unmarshal(o.contentByte, &o.Content)

		case true:
			var content []byte
			err = json.Unmarshal(o.contentByte, &content)
			o.Content = content
		}
		if err != nil {
			return nil, fmt.Errorf("fail to unmarshall content")
		}
		o.aead, err = p.aead.GetPrimitive(o.TenantID[:])
		if err != nil {
			return nil, fmt.Errorf("fail to load encryption primitive: %w", err)
		}

		outboxes = append(outboxes, o)
	}

	if len(outboxes) == 0 {
		return
	}

	if err = p.obSender(ctx, outboxes); err != nil {
		return nil, fmt.Errorf("fail to send outboxes: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("fail to commit: %w", err)
	}

	return outboxes[len(outboxes)-1], err
}
