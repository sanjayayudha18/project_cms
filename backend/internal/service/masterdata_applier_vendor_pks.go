package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPksApplier is the Applier for entity_type="vendor_pks" (spec
// vendor-pks, Req 3.7–3.10, 5.3, 5.5). create = new PKS replacing the active
// one; disable = terminate; enable = batalkan terminate. It never touches
// vendors.is_active. A change that no longer fits the DB returns
// ErrMasterDataChangeStale, which applyOne marks stale (task 7.3).
type VendorPksApplier struct{}

// Apply implements Applier.
func (VendorPksApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		return applyVendorPksCreate(ctx, q, change)

	case "disable":
		var p VendorPksReasonPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor pks terminate payload: %w", err)
		}
		row, err := q.TerminateVendorPks(ctx, db.TerminateVendorPksParams{
			ID: *change.EntityID, TerminatedBy: &change.MakerID, TerminationReason: &p.Reason,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, fmt.Errorf("terminate vendor pks %d: replaced or already terminated: %w", *change.EntityID, ErrMasterDataChangeStale)
		}
		if err != nil {
			return 0, nil, fmt.Errorf("terminate vendor pks %d: %w", *change.EntityID, err)
		}
		return row.ID, row, nil

	case "enable":
		row, err := q.UnterminateVendorPks(ctx, *change.EntityID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, fmt.Errorf("unterminate vendor pks %d: replaced or not terminated: %w", *change.EntityID, ErrMasterDataChangeStale)
		}
		if err != nil {
			return 0, nil, fmt.Errorf("unterminate vendor pks %d: %w", *change.EntityID, err)
		}
		return row.ID, row, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_pks", change.Op)
	}
}

// applyVendorPksCreate: lock the vendor (serialises every PKS apply of that
// vendor), re-check it is an active FLM vendor and that the active PKS is still
// the one the maker saw, then replace it. The old row keeps its terminate data
// (Req 3.10).
func applyVendorPksCreate(ctx context.Context, q *db.Queries, change db.MasterDataChangeRequest) (int64, any, error) {
	var p VendorPksCreatePayload
	if err := json.Unmarshal(change.Payload, &p); err != nil {
		return 0, nil, fmt.Errorf("unmarshal vendor pks create payload: %w", err)
	}
	eff, err := time.Parse(vendorPksDateLayout, p.EffectiveDate)
	if err != nil {
		return 0, nil, fmt.Errorf("vendor pks effective_date: %w", err)
	}
	exp, err := time.Parse(vendorPksDateLayout, p.ExpiryDate)
	if err != nil {
		return 0, nil, fmt.Errorf("vendor pks expiry_date: %w", err)
	}

	v, err := q.LockVendorForPks(ctx, p.VendorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, fmt.Errorf("vendor %d gone: %w", p.VendorID, ErrMasterDataChangeStale)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("lock vendor %d for pks: %w", p.VendorID, err)
	}
	if v.Kind == vendorKindInternal || v.DeletedAt.Valid {
		return 0, nil, fmt.Errorf("vendor %d is no longer an active FLM vendor: %w", p.VendorID, ErrMasterDataChangeStale)
	}

	var activeID *int64
	active, err := q.GetActiveVendorPks(ctx, p.VendorID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return 0, nil, fmt.Errorf("load active pks of vendor %d: %w", p.VendorID, err)
	default:
		activeID = &active.ID
	}
	if !sameInt64Ptr(activeID, p.ReplacesPksID) {
		return 0, nil, fmt.Errorf("active pks of vendor %d changed since submit: %w", p.VendorID, ErrMasterDataChangeStale)
	}

	if activeID != nil {
		err := q.MarkVendorPksReplaced(ctx, db.MarkVendorPksReplacedParams{
			ID: *activeID, ReplacedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return 0, nil, fmt.Errorf("mark vendor pks %d replaced: %w", *activeID, err)
		}
	}
	row, err := q.InsertVendorPks(ctx, db.InsertVendorPksParams{
		VendorID:      p.VendorID,
		PksName:       p.PksName,
		EffectiveDate: pgtype.Date{Time: eff, Valid: true},
		ExpiryDate:    pgtype.Date{Time: exp, Valid: true},
		CreatedBy:     change.MakerID,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("insert vendor pks for vendor %d: %w", p.VendorID, err)
	}
	if activeID != nil {
		err := q.SetVendorPksReplacedBy(ctx, db.SetVendorPksReplacedByParams{ID: *activeID, ReplacedByPksID: &row.ID})
		if err != nil {
			return 0, nil, fmt.Errorf("link vendor pks %d to %d: %w", *activeID, row.ID, err)
		}
	}
	return row.ID, row, nil
}

func sameInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// CurrentState implements Applier: the same db.VendorPk shape the service
// stores as Before for terminate / batalkan terminate.
func (VendorPksApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorPksByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor pks state: %w", err)
	}
	return row, nil
}
