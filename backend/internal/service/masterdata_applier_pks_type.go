package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// PksTypeApplier is the Applier for entity_type="vendor_pks_type" (vendor-pks-type task 3.2).
// Audit is written by applyOne in the same tx.
type PksTypeApplier struct{}

// Apply implements Applier.
func (PksTypeApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		var p PksTypeCreatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal pks type create payload: %w", err)
		}
		row, err := q.CreatePksType(ctx, db.CreatePksTypeParams{Code: p.Code, Name: p.Name})
		if isUniqueViolation(err) {
			return 0, nil, fmt.Errorf("create pks type %q: %w", p.Code, ErrPksTypeConflict)
		}
		if err != nil {
			return 0, nil, fmt.Errorf("create pks type: %w", err)
		}
		return row.ID, row, nil

	case "update":
		var p PksTypeUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal pks type update payload: %w", err)
		}
		row, err := q.UpdatePksTypeName(ctx, db.UpdatePksTypeNameParams{ID: *change.EntityID, Name: p.Name})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, fmt.Errorf("update pks type %d: inactive: %w", *change.EntityID, ErrMasterDataChangeStale)
		}
		if err != nil {
			return 0, nil, fmt.Errorf("update pks type: %w", err)
		}
		return row.ID, row, nil

	case "disable":
		return disablePksType(ctx, q, *change.EntityID)

	case "enable":
		row, err := q.EnablePksType(ctx, *change.EntityID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, fmt.Errorf("enable pks type %d: already active: %w", *change.EntityID, ErrMasterDataChangeStale)
		}
		if err != nil {
			return 0, nil, fmt.Errorf("enable pks type: %w", err)
		}
		return row.ID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_pks_type", change.Op)
	}
}

// disablePksType locks the type row FOR UPDATE before the in-use check, so a
// concurrent vendor_pks create (FOR SHARE on the same row) cannot slip in between
// (design DD6). In use → ErrPksTypeInUse: apply fails (409), change not applied.
func disablePksType(ctx context.Context, q *db.Queries, id int64) (int64, any, error) {
	if _, err := q.LockPksType(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, fmt.Errorf("disable pks type %d: gone: %w", id, ErrMasterDataChangeStale)
		}
		return 0, nil, fmt.Errorf("lock pks type: %w", err)
	}
	inUse, err := q.PksTypeInUse(ctx, id)
	if err != nil {
		return 0, nil, fmt.Errorf("check pks type in use: %w", err)
	}
	if inUse {
		return 0, nil, fmt.Errorf("disable pks type %d: %w", id, ErrPksTypeInUse)
	}
	row, err := q.DisablePksType(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, fmt.Errorf("disable pks type %d: already inactive: %w", id, ErrMasterDataChangeStale)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("disable pks type: %w", err)
	}
	return row.ID, map[string]bool{"is_active": false}, nil
}

// CurrentState implements Applier: same row shape the service stores as Before (GetPksType).
func (PksTypeApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetPksType(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current pks type state: %w", err)
	}
	return row, nil
}
