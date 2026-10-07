package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPackageApplier is the Applier for entity_type="vendor_package"
// (migration 016). The vpb_no_overlap exclusion constraint is the
// authoritative overlap guard -- re-checked here inside the apply
// transaction, so two overlapping requests that were both pending cannot both
// land. A violation surfaces as ErrVendorPackageOverlap (never the raw DB
// error); the approval handler maps it to a 409.
type VendorPackageApplier struct{}

// mapPackageDBError translates the exclusion-violation SQLSTATE; other errors
// pass through wrapped with the operation name.
func mapPackageDBError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
		return fmt.Errorf("%s vendor package: %w", op, ErrVendorPackageOverlap)
	}
	return fmt.Errorf("%s vendor package: %w", op, err)
}

// Apply implements Applier for entity_type="vendor_package".
func (VendorPackageApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p VendorPackagePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor package create payload: %w", err)
		}
		base, err := numericFromDecimalPtr(p.BasePrice)
		if err != nil {
			return 0, nil, err
		}
		start, err := priceDate(p.EffectiveStartDate)
		if err != nil {
			return 0, nil, err
		}
		end, err := priceDatePtr(p.EffectiveEndDate)
		if err != nil {
			return 0, nil, err
		}
		created, err := q.CreateVendorPackageAdmin(ctx, db.CreateVendorPackageAdminParams{
			VendorBranchID: &p.VendorBranchID, PackageCode: p.PackageCode, MachineGroup: p.MachineGroup, PriceClass: p.PriceClass,
			TierMin: int32(p.TierMin), TierMax: tierMaxInt32(p.TierMax),
			BasePrice: base, AtmID: p.AtmID, SlaNote: p.SlaNote, Currency: p.Currency,
			EffectiveStartDate: start, EffectiveEndDate: end,
		})
		if err != nil {
			return 0, nil, mapPackageDBError("create", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p VendorPackageContentPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor package update payload: %w", err)
		}
		base, err := numericFromDecimalPtr(p.BasePrice)
		if err != nil {
			return 0, nil, err
		}
		end, err := priceDatePtr(p.EffectiveEndDate)
		if err != nil {
			return 0, nil, err
		}
		updated, err := q.UpdateVendorPackageAdmin(ctx, db.UpdateVendorPackageAdminParams{
			ID: *change.EntityID, BasePrice: base, SlaNote: p.SlaNote, EffectiveEndDate: end,
		})
		if err != nil {
			return 0, nil, mapPackageDBError("update", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableVendorPackage(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor package: %w", err)
		}
		return *change.EntityID, map[string]string{"effective_end_date": "closed"}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_package", change.Op)
	}
}

// CurrentState implements Applier: same db.GetVendorPackageAdminByIDRow shape
// VendorPackageAdminService uses for SubmitRequest.Before.
func (VendorPackageApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorPackageAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor package state: %w", err)
	}
	return row, nil
}
