package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPackageApplier is the Applier for entity_type="vendor_package" (T3.4).
type VendorPackageApplier struct{}

// Apply implements Applier for entity_type="vendor_package".
func (VendorPackageApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p VendorPackagePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor package create payload: %w", err)
		}
		price, err := numericFromDecimalPtr(&p.Price)
		if err != nil {
			return 0, nil, err
		}
		created, err := q.CreateVendorPackageAdmin(ctx, db.CreateVendorPackageAdminParams{
			VendorBranchID: p.VendorBranchID, Code: p.Code, PriorityClass: p.PriorityClass, Price: price,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("create vendor package: %w", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p VendorPackageUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor package update payload: %w", err)
		}
		price, err := numericFromDecimalPtr(&p.Price)
		if err != nil {
			return 0, nil, err
		}
		updated, err := q.UpdateVendorPackageAdmin(ctx, db.UpdateVendorPackageAdminParams{
			ID: *change.EntityID, PriorityClass: p.PriorityClass, Price: price,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("update vendor package: %w", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableVendorPackage(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor package: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableVendorPackage(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable vendor package: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

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
