package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorVaultApplier is the Applier for entity_type="vendor_vault" (T3.2).
type VendorVaultApplier struct{}

// numericFromDecimalPtr parses an optional decimal string into pgtype.Numeric
// (nil/blank => SQL NULL). Never goes through float.
func numericFromDecimalPtr(s *string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if s == nil || strings.TrimSpace(*s) == "" {
		return n, nil
	}
	if err := n.Scan(strings.TrimSpace(*s)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("parse decimal %q: %w", *s, err)
	}
	return n, nil
}

// vaultNumerics converts the four numeric fields of an update payload.
func vaultNumerics(p VendorVaultUpdatePayload) (minC, maxC, lat, lon pgtype.Numeric, err error) {
	for _, f := range []struct {
		dst *pgtype.Numeric
		src *string
	}{{&minC, p.MinCapacityAmount}, {&maxC, p.MaxCapacityAmount}, {&lat, p.Latitude}, {&lon, p.Longitude}} {
		if *f.dst, err = numericFromDecimalPtr(f.src); err != nil {
			return
		}
	}
	return
}

// Apply implements Applier for entity_type="vendor_vault".
func (VendorVaultApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p VendorVaultPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor vault create payload: %w", err)
		}
		minC, maxC, lat, lon, err := vaultNumerics(p.VendorVaultUpdatePayload)
		if err != nil {
			return 0, nil, err
		}
		created, err := q.CreateVendorVaultAdmin(ctx, db.CreateVendorVaultAdminParams{
			VendorBranchID: p.VendorBranchID, VaultCode: p.VaultCode, Category: p.Category, CurrencyCode: p.CurrencyCode,
			MinCapacityAmount: minC, MaxCapacityAmount: maxC, Latitude: lat, Longitude: lon,
			OperatingHours: p.OperatingHours, LocationID: p.LocationID,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("create vendor vault: %w", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p VendorVaultUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor vault update payload: %w", err)
		}
		minC, maxC, lat, lon, err := vaultNumerics(p)
		if err != nil {
			return 0, nil, err
		}
		updated, err := q.UpdateVendorVaultAdmin(ctx, db.UpdateVendorVaultAdminParams{
			ID: *change.EntityID, Category: p.Category, CurrencyCode: p.CurrencyCode,
			MinCapacityAmount: minC, MaxCapacityAmount: maxC, Latitude: lat, Longitude: lon,
			OperatingHours: p.OperatingHours, LocationID: p.LocationID,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("update vendor vault: %w", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableVendorVault(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor vault: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableVendorVault(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable vendor vault: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_vault", change.Op)
	}
}

// CurrentState implements Applier: same db.GetVendorVaultAdminByIDRow shape
// VendorVaultAdminService uses for SubmitRequest.Before.
func (VendorVaultApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorVaultAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor vault state: %w", err)
	}
	return row, nil
}
