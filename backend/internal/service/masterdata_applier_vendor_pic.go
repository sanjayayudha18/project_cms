package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPicApplier is the Applier for entity_type="vendor_pic" (T3.3).
type VendorPicApplier struct{}

// Apply implements Applier for entity_type="vendor_pic".
func (VendorPicApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p VendorPicPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor pic create payload: %w", err)
		}
		created, err := q.CreateVendorPicAdmin(ctx, db.CreateVendorPicAdminParams{
			VendorID: p.VendorID, VendorBranchID: p.VendorBranchID, Name: p.Name, Position: p.Position,
			Phone: p.Phone, Email: p.Email, IsNotificationRecipient: p.IsNotificationRecipient,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("create vendor pic: %w", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p VendorPicUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor pic update payload: %w", err)
		}
		updated, err := q.UpdateVendorPicAdmin(ctx, db.UpdateVendorPicAdminParams{
			ID: *change.EntityID, VendorBranchID: p.VendorBranchID, Name: p.Name, Position: p.Position,
			Phone: p.Phone, Email: p.Email, IsNotificationRecipient: p.IsNotificationRecipient,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("update vendor pic: %w", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableVendorPic(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor pic: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableVendorPic(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable vendor pic: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_pic", change.Op)
	}
}

// CurrentState implements Applier: same db.GetVendorPicAdminByIDRow shape
// VendorPicAdminService uses for SubmitRequest.Before.
func (VendorPicApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorPicAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor pic state: %w", err)
	}
	return row, nil
}
