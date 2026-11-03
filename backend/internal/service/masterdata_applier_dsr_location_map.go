package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// errDsrLocationMapVaultInvalid: at apply time the vault is gone, disabled, or
// owned by another vendor (the guarded INSERT/UPDATE matched no row).
var errDsrLocationMapVaultInvalid = errors.New("vault tidak aktif atau bukan milik vendor ini saat apply")

// DsrLocationMapApplier is the Applier for entity_type="dsr_location_map" (cit-acm-plan FR2).
type DsrLocationMapApplier struct{}

// Apply implements Applier.
func (DsrLocationMapApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		var p DsrLocationMapPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal dsr location map create payload: %w", err)
		}
		row, err := q.CreateDsrLocationMap(ctx, db.CreateDsrLocationMapParams{VendorID: p.VendorID, DsrLocation: p.DsrLocation, VendorVaultID: p.VendorVaultID})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, errDsrLocationMapVaultInvalid
		}
		if err != nil {
			return 0, nil, fmt.Errorf("create dsr location map: %w", err)
		}
		return row.ID, row, nil

	case "update":
		var p DsrLocationMapUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal dsr location map update payload: %w", err)
		}
		row, err := q.UpdateDsrLocationMap(ctx, db.UpdateDsrLocationMapParams{ID: *change.EntityID, VendorVaultID: p.VendorVaultID})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, errDsrLocationMapVaultInvalid
		}
		if err != nil {
			return 0, nil, fmt.Errorf("update dsr location map: %w", err)
		}
		return row.ID, row, nil

	case "disable":
		if err := q.DisableDsrLocationMap(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable dsr location map: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if err := q.EnableDsrLocationMap(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable dsr location map: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=dsr_location_map", change.Op)
	}
}

// CurrentState implements Applier: same row shape the service stores as Before.
func (DsrLocationMapApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetDsrLocationMapAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current dsr location map state: %w", err)
	}
	return row, nil
}
