package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Outbox struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ContentType    string
	CreatedAt      time.Time
	Content        []byte
	storeEncrypted bool
}

func newOutbox(tid uuid.UUID, ctype string, content any) (o *Outbox, err error) {
	o = &Outbox{TenantID: tid, ContentType: ctype, CreatedAt: time.Now()}
	o.ID, err = uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("fail to create new id for outbox: %w", err)
	}
	o.Content, err = json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal content as json")
	}
	return
}

func newOutboxEncrypted(tid uuid.UUID, ctype string, content any) (o *Outbox, err error) {
	o, err = newOutbox(tid, ctype, content)
	if err != nil {
		return
	}
	o.storeEncrypted = true
	return
}

type OutboxSender func(context.Context, []*Outbox) error

func (p *Postgres) storeOutbox(ctx context.Context, tx *sql.Tx, ob *Outbox) (err error) {
	_, span := p.tracer.Start(ctx, "storeOutbox", trace.WithAttributes(
		attribute.Stringer("tenantID", ob.TenantID),
		attribute.Stringer("id", ob.ID),
		attribute.String("contentType", ob.ContentType),
	))
	defer span.End()

	content := ob.Content
	if ob.storeEncrypted {
		paead, err := p.aead.GetPrimitive(ob.TenantID)
		if err != nil {
			return err
		}
		content, err = paead.Encrypt(content, ob.ID[:])
		if err != nil {
			return fmt.Errorf("fail to encrypt outbox: %w", err)
		}
		content, err = json.Marshal(content)
		if err != nil {
			return fmt.Errorf("fail to marshal encrypted outbox: %w", err)
		}
	}

	outboxQ := `
		INSERT INTO outbox 
		(id, tenant_id, type, content, is_encrypted, created_at)
		VALUES
		($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.ExecContext(ctx, outboxQ,
		ob.ID, ob.TenantID, ob.ContentType, content, ob.storeEncrypted, ob.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("fail to insert to outbox: %w", err)
	}

	_, err = tx.QueryContext(ctx, "SELECT pg_notify($1, $2)", outboxChannel, ob.CreatedAt.UnixNano())
	if err != nil {
		return fmt.Errorf("fail to send notification: %w", err)
	}

	return
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
		RETURNING o.id, o.tenant_id, o.type, o.content, o.is_encrypted, o.created_at
	`
	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	outboxes := []*Outbox{}
	for rows.Next() {
		o := &Outbox{}

		err = rows.Scan(&o.ID, &o.TenantID, &o.ContentType, &o.Content, &o.storeEncrypted, &o.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("fail to scan row: %w", err)
		}

		if o.storeEncrypted {
			var content []byte
			err = json.Unmarshal(o.Content, &content)
			if err != nil {
				return nil, fmt.Errorf("fail to unmarshal encrypted outbox: %w", err)
			}
			paead, err := p.aead.GetPrimitive(o.TenantID)
			if err != nil {
				return nil, err
			}
			o.Content, err = paead.Decrypt(content, o.ID[:])
			if err != nil {
				return nil, fmt.Errorf("fail to decrypt encrypted outbox: %w", err)
			}
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

func keyNameAsHash64(keyName string) uint64 {
	hash := fnv.New64()
	_, err := hash.Write([]byte(keyName))
	if err != nil {
		panic(err)
	}
	return hash.Sum64()
}

var outboxChannel = "outbox"
var outboxLock = keyNameAsHash64("outbox")

func (p *Postgres) watchOutboxes(ctx context.Context) (err error) {
	for {
		nextwait := time.Minute
		if err = ctx.Err(); err != nil {
			return
		}

		func() {
			cango := false
			conn, err := p.db.Conn(ctx)
			if err != nil {
				p.logger.Error("fail to obtain db connection for lock; %w", logger.Any("error", err))
				return
			}
			defer conn.Close()

			err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, outboxLock).Scan(&cango)
			if !cango || err != nil {
				p.logger.Warn("Fail to obtain lock. Retrying in next 1 minute", logger.Any("error", err))
				return
			}

			l := pq.NewListener(p.dbUrl, time.Second, time.Minute, func(event pq.ListenerEventType, err error) { return })
			err = l.Listen(outboxChannel)
			if err != nil {
				p.logger.Error("fail to listen for outbox notification", logger.Any("error", err))
				return
			}
			defer l.Close()

			nextwait = 0
			var last *Outbox
			for {
				var event *pq.Notification
				select {
				case <-ctx.Done():
					return

				case event = <-l.NotificationChannel():
				}

				i, _ := strconv.ParseInt(event.Extra, 10, 64)
				if last != nil && i < last.CreatedAt.UnixNano() {
					continue
				}

				last, err = p.sendOutbox(ctx, 100)
				if err != nil {
					p.logger.Error("fail to send outboxes", logger.Any("error", err))
				}
			}
		}()

		<-time.After(nextwait)
	}
}
