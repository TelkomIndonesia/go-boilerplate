package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/rucciva/go-boilerplate/pkg/profile"
)

func (p *Postgres) StoreProfile(ctx context.Context, pr *profile.Profile) (err error) {
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
		p.argMacStr(pr.TenantID, pr.NIN),
		p.argEncStr(pr.TenantID, pr.Name, pr.ID[:]),
		p.argMacStr(pr.TenantID, pr.Name),
		p.argEncStr(pr.TenantID, pr.Phone, pr.ID[:]),
		p.argMacStr(pr.TenantID, pr.Phone),
		p.argEncStr(pr.TenantID, pr.Email, pr.ID[:]),
		p.argMacStr(pr.TenantID, pr.Email),
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
	if err = p.storeTextHeap(ctx, tx, pr.TenantID, "profile_name", pr.Name); err != nil {
		return fmt.Errorf("fail to store profile name to text_heap: %w", err)
	}

	// outbox
	msgFunc := argAsB64(p.argEncJSON(pr.TenantID, pr, pr.ID[:]))
	if err = p.storeOutbox(ctx, tx, pr.TenantID, "profile_saved", msgFunc); err != nil {
		return fmt.Errorf("fail to store profile to outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("fail to commit: %w", err)
	}
	return
}

func (p *Postgres) FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *profile.Profile, err error) {
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

	paead, err := p.aead.GetPrimitive(tenantID)
	if err != nil {
		return nil, err
	}
	nin, err = paead.Decrypt(nin, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt nin : %w", err)
	}
	name, err = paead.Decrypt(name, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt nin : %w", err)
	}
	phone, err = paead.Decrypt(phone, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt nin : %w", err)
	}
	email, err = paead.Decrypt(email, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt nin : %w", err)
	}
	dob, err = paead.Decrypt(dob, id[:])
	if err != nil {
		return nil, fmt.Errorf("fail to decrypt nin : %w", err)
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
		return nil, fmt.Errorf("fail to unmarshal dat of birth: %w", err)
	}

	return
}
