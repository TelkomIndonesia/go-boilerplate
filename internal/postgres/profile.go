package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/internal/postgres/internal/outbox"
	"github.com/telkomindonesia/go-boilerplate/internal/postgres/internal/sqlc"
	"github.com/telkomindonesia/go-boilerplate/internal/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/outboxce"
	"github.com/telkomindonesia/go-boilerplate/pkg/tinkx/tinksql"
)

func (p *Postgres) StoreProfile(ctx context.Context, pr *profile.Profile) (err error) {
	tx, errtx := p.db.BeginTx(ctx, &sql.TxOptions{})
	if errtx != nil {
		return fmt.Errorf("failed to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	query := p.q.WithTx(tx)
	err = query.StoreProfile(ctx, sqlc.StoreProfileParams{
		ID:        pr.ID,
		TenantID:  pr.TenantID,
		Nin:       tinksql.AEADString(p.aeadFunc(&pr.TenantID), pr.NIN, pr.ID[:]),
		NinBidx:   tinksql.BIDXString(p.bidxFullFunc(&pr.TenantID), pr.NIN),
		Name:      tinksql.AEADString(p.aeadFunc(&pr.TenantID), pr.Name, pr.ID[:]),
		NameBidx:  tinksql.BIDXString(p.bidxFunc(&pr.TenantID), pr.Name),
		Phone:     tinksql.AEADString(p.aeadFunc(&pr.TenantID), pr.Phone, pr.ID[:]),
		PhoneBidx: tinksql.BIDXString(p.bidxFunc(&pr.TenantID), pr.Phone),
		Email:     tinksql.AEADString(p.aeadFunc(&pr.TenantID), pr.Email, pr.ID[:]),
		EmailBidx: tinksql.BIDXString(p.bidxFunc(&pr.TenantID), pr.Email),
		Dob:       tinksql.AEADTime(p.aeadFunc(&pr.TenantID), pr.DOB, pr.ID[:]),
	})
	if err != nil {
		return fmt.Errorf("failed to insert to profile: %w", err)
	}

	// text heap
	if err = query.StoreTextHeap(ctx, sqlc.StoreTextHeapParams{
		TenantID: pr.TenantID,
		Type:     textHeapTypeProfileName,
		Content:  pr.Name,
	}); err != nil {
		return fmt.Errorf("failed to store profile name to text_heap: %w", err)
	}

	// outbox
	ob := outboxce.
		New(outboxceSource, outboxceEventProfileStored, pr.TenantID, outbox.FromProfile(pr)).
		WithEncryptor(outboxce.TenantAEAD(p.aead))
	if err = p.obceManager.Store(ctx, tx, ob); err != nil {
		return fmt.Errorf("failed to store profile to outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return
}

func (p *Postgres) FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *profile.Profile, err error) {
	spr, err := p.q.FetchProfile(ctx,
		sqlc.FetchProfileParams{TenantID: tenantID, ID: id},
		func(fpr *sqlc.FetchProfileRow) {
			// initiate so that we can decrypt
			fpr.Nin = tinksql.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Name = tinksql.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Phone = tinksql.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Email = tinksql.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Dob = tinksql.AEADTime(p.aeadFunc(&tenantID), time.Time{}, id[:])
		},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to select profile: %w", err)
	}

	pr = &profile.Profile{
		ID:       id,
		TenantID: tenantID,
		NIN:      spr.Nin.Plain(),
		Name:     spr.Name.Plain(),
		Phone:    spr.Phone.Plain(),
		Email:    spr.Email.Plain(),
		DOB:      spr.Dob.Plain(),
	}
	return
}

func (p *Postgres) FindProfileNames(ctx context.Context, tenantID uuid.UUID, qname string) (names []string, err error) {
	return p.q.FindTextHeap(ctx, sqlc.FindTextHeapParams{
		TenantID: tenantID,
		Type:     textHeapTypeProfileName,
		Content:  sql.NullString{String: qname, Valid: true},
	},
		nil, nil, // no need for initiation or additional filter
	)
}

func (p *Postgres) FindProfilesByName(ctx context.Context, tenantID uuid.UUID, qname string) (prs []*profile.Profile, err error) {
	// we don't need the return value since we are using the Filter func to efficiently convert the item
	_, err = p.q.FindProfilesByName(ctx,
		sqlc.FindProfilesByNameParams{
			TenantID: tenantID,
			NameBidx: tinksql.BIDXString(p.bidxFunc(&tenantID), qname).ForRead(pqByteArray),
		},
		func(fpbnr *sqlc.FindProfilesByNameRow) {
			// initiate so that we can decrypt
			fpbnr.Nin = tinksql.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Name = tinksql.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Phone = tinksql.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Email = tinksql.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Dob = tinksql.AEADTime(p.aeadFunc(&fpbnr.TenantID), time.Time{}, fpbnr.ID[:])
		},
		func(fpbnr sqlc.FindProfilesByNameRow) (bool, error) {
			// due to bloom filter, we need to verify if the name match
			if fpbnr.Name.Plain() != qname {
				return false, nil
			}

			prs = append(prs,
				&profile.Profile{
					ID:       fpbnr.ID,
					TenantID: tenantID,
					NIN:      fpbnr.Nin.Plain(),
					Name:     fpbnr.Name.Plain(),
					Email:    fpbnr.Email.Plain(),
					Phone:    fpbnr.Phone.Plain(),
					DOB:      fpbnr.Dob.Plain(),
				})
			return false, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query profile by name: %w", err)
	}
	return
}
