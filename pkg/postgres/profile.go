package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt/sqlval"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	outboxTypeProfile        = "profile"
	outboxEventProfileStored = "profile_stored"
)

func (p *Postgres) StoreProfile(ctx context.Context, pr *profile.Profile) (err error) {
	_, span := p.tracer.Start(ctx, "storeProfile", trace.WithAttributes(
		attribute.Stringer("tenantID", pr.TenantID),
		attribute.Stringer("id", pr.ID),
	))
	defer span.RecordError(err)
	defer span.End()

	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return fmt.Errorf("fail to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	// profile
	insertProfile := `
		INSERT INTO profile
		(id, tenant_id, nin, nin_bidx, name, name_bidx, phone, phone_bidx, email, email_bidx, dob)
		VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id)
		DO UPDATE SET updated_at = NOW()
	`
	_, err = tx.ExecContext(ctx, insertProfile,
		pr.ID,
		pr.TenantID,
		sqlval.AEADString(p.aeadFunc(pr.TenantID), pr.NIN, pr.ID[:]),
		sqlval.BIDXString(p.bidxFunc(pr.TenantID), pr.NIN),
		sqlval.AEADString(p.aeadFunc(pr.TenantID), pr.Name, pr.ID[:]),
		sqlval.BIDXString(p.bidxFunc(pr.TenantID), pr.Name),
		sqlval.AEADString(p.aeadFunc(pr.TenantID), pr.Phone, pr.ID[:]),
		sqlval.BIDXString(p.bidxFunc(pr.TenantID), pr.Phone),
		sqlval.AEADString(p.aeadFunc(pr.TenantID), pr.Email, pr.ID[:]),
		sqlval.BIDXString(p.bidxFunc(pr.TenantID), pr.Email),
		sqlval.AEADTime(p.aeadFunc(pr.TenantID), pr.DOB, pr.ID[:]),
	)
	if err != nil {
		return fmt.Errorf("fail to insert to profile: %w", err)
	}

	// text heap
	if err = p.storeTextHeap(ctx, tx, textHeap{
		tenantID:    pr.TenantID,
		contentType: "profile_name",
		content:     pr.Name,
	}); err != nil {
		return fmt.Errorf("fail to store profile name to text_heap: %w", err)
	}

	// outbox
	ob, err := p.newOutboxEncrypted(pr.TenantID, outboxEventProfileStored, outboxTypeProfile, pr)
	if err != nil {
		return fmt.Errorf("fail to create outbox: %w", err)
	}
	if err = p.storeOutbox(ctx, tx, ob); err != nil {
		return fmt.Errorf("fail to store profile to outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("fail to commit: %w", err)
	}

	return
}

func (p *Postgres) FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *profile.Profile, err error) {
	_, span := p.tracer.Start(ctx, "fetchProfile", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
		attribute.Stringer("id", id),
	))
	defer span.End()

	nin := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
	name := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
	phone := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
	email := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
	dob := sqlval.AEADTime(p.aeadFunc(tenantID), time.Time{}, id[:])
	q := `SELECT nin, name, phone, email, dob FROM profile WHERE id = $1 AND tenant_id = $2`
	err = p.db.QueryRowContext(ctx, q,
		id, tenantID).
		Scan(&nin, &name, &phone, &email, &dob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail to select profile: %w", err)
	}

	pr = &profile.Profile{
		ID:       id,
		TenantID: tenantID,
		NIN:      nin.To(),
		Name:     name.To(),
		Phone:    phone.To(),
		Email:    email.To(),
		DOB:      dob.To(),
	}

	return
}

func (p *Postgres) FindProfileNames(ctx context.Context, tenantID uuid.UUID, qname string) (names []string, err error) {
	_, span := p.tracer.Start(ctx, "findProfileNames", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
	))
	defer span.End()

	return p.findTextHeap(ctx, tenantID, "profile_name", qname)
}

func (p *Postgres) FindProfilesByName(ctx context.Context, tenantID uuid.UUID, qname string) (prs []*profile.Profile, err error) {
	_, span := p.tracer.Start(ctx, "findProfilesByName", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
	))
	defer span.End()

	q := `SELECT id, nin, name, phone, email, dob FROM profile WHERE tenant_id = $1 and name_bidx = ANY($2)`
	rows, err := p.db.QueryContext(ctx, q,
		tenantID,
		sqlval.BIDXString(p.bidx.GetPrimitiveFunc(tenantID[:]), qname).ForRead(pqByteArray))
	if err != nil {
		return nil, fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		nin := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
		name := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
		phone := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
		email := sqlval.AEADString(p.aeadFunc(tenantID), "", id[:])
		dob := sqlval.AEADTime(p.aeadFunc(tenantID), time.Time{}, id[:])
		err = rows.Scan(&id, &nin, &name, &phone, &email, &dob)
		if err != nil {
			return nil, fmt.Errorf("fail to scan row: %w", err)
		}
		if qname != name.To() {
			continue
		}

		pr := &profile.Profile{
			ID:       id,
			TenantID: tenantID,
			NIN:      nin.To(),
			Name:     name.To(),
			Phone:    phone.To(),
			Email:    email.To(),
			DOB:      dob.To(),
		}
		prs = append(prs, pr)
	}
	return
}
