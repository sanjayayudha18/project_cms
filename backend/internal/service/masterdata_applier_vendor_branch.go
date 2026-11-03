package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func ensureBranchHasNoActiveChildren(ctx context.Context, q *db.Queries, branchID int64) error {
	vaults, err := q.CountActiveVendorVaultsByBranch(ctx, branchID)
	if err != nil {
		return fmt.Errorf("checking active vaults: %w", err)
	}
	pics, err := q.CountActiveVendorPicsByBranch(ctx, &branchID)
	if err != nil {
		return fmt.Errorf("checking active pics: %w", err)
	}
	pkgs, err := q.CountActiveVendorPackagesByBranch(ctx, &branchID)
	if err != nil {
		return fmt.Errorf("checking active packages: %w", err)
	}
	if vaults+pics+pkgs > 0 {
		return fmt.Errorf("disable vendor branch: %w", ErrVendorBranchHasActiveChildren)
	}
	return nil
}

// VendorBranchApplier is the Applier for entity_type="vendor_branch" (T3.1).
type VendorBranchApplier struct{}

// Apply implements Applier for entity_type="vendor_branch".
func (VendorBranchApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p VendorBranchPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor branch create payload: %w", err)
		}
		created, err := q.CreateVendorBranchAdmin(ctx, db.CreateVendorBranchAdminParams{
			VendorID:   p.VendorID,
			BranchCode: p.BranchCode,
			BranchName: p.BranchName,
			LocationID: p.LocationID,
			Region:     p.Region,
			RegionCode: p.RegionCode,
			Category:   p.Category,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("create vendor branch: %w", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p VendorBranchUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor branch update payload: %w", err)
		}
		updated, err := q.UpdateVendorBranchAdmin(ctx, db.UpdateVendorBranchAdminParams{
			ID:         *change.EntityID,
			BranchName: p.BranchName,
			LocationID: p.LocationID,
			Region:     p.Region,
			RegionCode: p.RegionCode,
			Category:   p.Category,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("update vendor branch: %w", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		// Re-check at apply time: a vault/PIC/package may have been approved
		// between this disable's submit and its approval (decision 2026-09-25).
		if err := ensureBranchHasNoActiveChildren(ctx, q, *change.EntityID); err != nil {
			return 0, nil, err
		}
		if err := q.DisableVendorBranch(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor branch: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableVendorBranch(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable vendor branch: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_branch", change.Op)
	}
}

// CurrentState implements Applier for entity_type="vendor_branch". Returns
// the same db.GetVendorBranchAdminByIDRow shape VendorBranchAdminService
// uses for SubmitRequest.Before on update/disable/enable submissions.
func (VendorBranchApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorBranchAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor branch state: %w", err)
	}
	return row, nil
}
