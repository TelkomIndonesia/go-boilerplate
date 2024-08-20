package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres/internal/outbox"
	"github.com/telkomindonesia/go-boilerplate/pkg/postgres/internal/sqlc"
	"github.com/telkomindonesia/go-boilerplate/pkg/profile"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/crypt/sqlval"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/outboxce"
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
		return fmt.Errorf("failed to open transaction: %w", err)
	}
	defer txRollbackDeferer(tx, &err)()

	query := p.q.WithTx(tx)
	err = query.StoreProfile(ctx, sqlc.StoreProfileParams{
		ID:        pr.ID,
		TenantID:  pr.TenantID,
		Nin:       sqlval.AEADString(p.aeadFunc(&pr.TenantID), pr.NIN, pr.ID[:]),
		NinBidx:   sqlval.BIDXString(p.bidxFullFunc(&pr.TenantID), pr.NIN),
		Name:      sqlval.AEADString(p.aeadFunc(&pr.TenantID), pr.Name, pr.ID[:]),
		NameBidx:  sqlval.BIDXString(p.bidxFunc(&pr.TenantID), pr.Name),
		Phone:     sqlval.AEADString(p.aeadFunc(&pr.TenantID), pr.Phone, pr.ID[:]),
		PhoneBidx: sqlval.BIDXString(p.bidxFunc(&pr.TenantID), pr.Phone),
		Email:     sqlval.AEADString(p.aeadFunc(&pr.TenantID), pr.Email, pr.ID[:]),
		EmailBidx: sqlval.BIDXString(p.bidxFunc(&pr.TenantID), pr.Email),
		Dob:       sqlval.AEADTime(p.aeadFunc(&pr.TenantID), pr.DOB, pr.ID[:]),
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
		New(outboxSource, eventProfileStored, pr.TenantID, outbox.FromProfile(pr)).
		WithEncryptor(outboxce.TenantAEAD(p.aead))
	if err = p.outboxManager.Store(ctx, tx, ob); err != nil {
		return fmt.Errorf("failed to store profile to outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return
}

func (p *Postgres) FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (pr *profile.Profile, err error) {
	_, span := p.tracer.Start(ctx, "fetchProfile", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
		attribute.Stringer("id", id),
	))
	defer span.End()

	spr, err := p.q.FetchProfile(ctx,
		sqlc.FetchProfileParams{TenantID: tenantID, ID: id},
		func(fpr *sqlc.FetchProfileRow) {
			// initiate so that we can decrypt
			fpr.Nin = sqlval.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Name = sqlval.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Phone = sqlval.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Email = sqlval.AEADString(p.aeadFunc(&tenantID), "", id[:])
			fpr.Dob = sqlval.AEADTime(p.aeadFunc(&tenantID), time.Time{}, id[:])
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
		NIN:      spr.Nin.To(),
		Name:     spr.Name.To(),
		Phone:    spr.Phone.To(),
		Email:    spr.Email.To(),
		DOB:      spr.Dob.To(),
	}
	return
}

func (p *Postgres) FindProfileNames(ctx context.Context, tenantID uuid.UUID, qname string) (names []string, err error) {
	_, span := p.tracer.Start(ctx, "findProfileNames", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
	))
	defer span.End()

	return p.q.FindTextHeap(ctx, sqlc.FindTextHeapParams{
		TenantID: tenantID,
		Type:     textHeapTypeProfileName,
		Content:  sql.NullString{String: qname, Valid: true},
	},
		nil, nil, // no need for initiation or additional filter
	)
}

func (p *Postgres) FindProfilesByName(ctx context.Context, tenantID uuid.UUID, qname string) (prs []*profile.Profile, err error) {
	_, span := p.tracer.Start(ctx, "findProfilesByName", trace.WithAttributes(
		attribute.Stringer("tenantID", tenantID),
	))
	defer span.End()

	// we don't need the return value since we are using the Filter func to efficiently convert the item
	_, err = p.q.FindProfilesByName(ctx,
		sqlc.FindProfilesByNameParams{
			TenantID: tenantID,
			NameBidx: sqlval.BIDXString(p.bidxFunc(&tenantID), qname).ForRead(pqByteArray),
		},
		func(fpbnr *sqlc.FindProfilesByNameRow) {
			// initiate so that we can decrypt
			fpbnr.Nin = sqlval.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Name = sqlval.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Phone = sqlval.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Email = sqlval.AEADString(p.aeadFunc(&fpbnr.TenantID), "", fpbnr.ID[:])
			fpbnr.Dob = sqlval.AEADTime(p.aeadFunc(&fpbnr.TenantID), time.Time{}, fpbnr.ID[:])
		},
		func(fpbnr sqlc.FindProfilesByNameRow) (bool, error) {
			// due to bloom filter, we need to verify if the name match
			if fpbnr.Name.To() != qname {
				return false, nil
			}

			prs = append(prs,
				&profile.Profile{
					ID:       fpbnr.ID,
					TenantID: tenantID,
					NIN:      fpbnr.Nin.To(),
					Name:     fpbnr.Name.To(),
					Email:    fpbnr.Email.To(),
					Phone:    fpbnr.Phone.To(),
					DOB:      fpbnr.Dob.To(),
				})
			return false, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query profile by name: %w", err)
	}
	return
}
