package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	args, err := argList(
		argLiteral(pr.ID),
		argLiteral(pr.TenantID),
		p.argEncStr(pr.TenantID, pr.NIN, pr.ID[:]),
		p.argBlindIdx(pr.TenantID, pr.NIN),
		p.argEncStr(pr.TenantID, pr.Name, pr.ID[:]),
		p.argBlindIdx(pr.TenantID, pr.Name),
		p.argEncStr(pr.TenantID, pr.Phone, pr.ID[:]),
		p.argBlindIdx(pr.TenantID, pr.Phone),
		p.argEncStr(pr.TenantID, pr.Email, pr.ID[:]),
		p.argBlindIdx(pr.TenantID, pr.Email),
		p.argEncTime(pr.TenantID, pr.DOB, pr.ID[:]),
	)
	if err != nil {
		return fmt.Errorf("fail to prepare arguments for insert profile: %w", err)
	}
	_, err = tx.ExecContext(ctx, insertProfile, args...)
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
	ob, err := p.newOutboxEncrypted(pr.TenantID, "profile_stored", "profile", pr)
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

	var nin, name, phone, email, dob []byte
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

	paead, err := p.getAEAD(tenantID)
	if err != nil {
		return nil, err
	}
	nin, err = paead.Decrypt(nin, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt nin : %w", err)
	}
	name, err = paead.Decrypt(name, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt name : %w", err)
	}
	phone, err = paead.Decrypt(phone, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt phone : %w", err)
	}
	email, err = paead.Decrypt(email, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt email : %w", err)
	}
	dob, err = paead.Decrypt(dob, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt dob : %w", err)
	}

	pr = &profile.Profile{
		ID:       id,
		TenantID: tenantID,
		NIN:      string(nin),
		Name:     string(name),
		Phone:    string(phone),
		Email:    string(email),
	}
	if err = pr.DOB.UnmarshalBinary(dob); err != nil {
		return nil, fmt.Errorf("fail to unmarshal dob: %w", err)
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

	nameIdxs, err := p.getBlindIdxs(tenantID, []byte(qname))
	if err != nil {
		return nil, fmt.Errorf("fail to compute blind indexes from profile name: %w", err)
	}

	q := `SELECT id, nin, name, phone, email, dob FROM profile WHERE tenant_id = $1 and name_bidx = ANY($2)`
	rows, err := p.db.QueryContext(ctx, q, tenantID, pq.Array(nameIdxs))
	if err != nil {
		return nil, fmt.Errorf("fail to query profile by name: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var nin, name, phone, email, dob []byte
		err = rows.Scan(&id, &nin, &name, &phone, &email, &dob)
		if err != nil {
			return nil, fmt.Errorf("fail to scan row: %w", err)
		}

		paead, err := p.getAEAD(tenantID)
		if err != nil {
			return nil, err
		}
		name, err = paead.Decrypt(name, id[:])
		if err != nil {
			return nil, fmt.Errorf("fail to decrypt name : %w", err)
		}
		if string(name) != qname {
			continue
		}

		nin, err = paead.Decrypt(nin, id[:])
		if err != nil {
			return nil, fmt.Errorf("fail to decrypt nin : %w", err)
		}
		phone, err = paead.Decrypt(phone, id[:])
		if err != nil {
			return nil, fmt.Errorf("fail to decrypt phone : %w", err)
		}
		email, err = paead.Decrypt(email, id[:])
		if err != nil {
			return nil, fmt.Errorf("fail to decrypt email : %w", err)
		}
		dob, err = paead.Decrypt(dob, id[:])
		if err != nil {
			return nil, fmt.Errorf("fail to decrypt dob : %w", err)
		}

		pr := &profile.Profile{
			ID:       id,
			TenantID: tenantID,
			NIN:      string(nin),
			Name:     string(name),
			Phone:    string(phone),
			Email:    string(email),
		}
		if err = pr.DOB.UnmarshalBinary(dob); err != nil {
			return nil, fmt.Errorf("fail to unmarshal dob: %w", err)
		}
		prs = append(prs, pr)
	}
	return
}
