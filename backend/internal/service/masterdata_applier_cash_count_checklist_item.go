package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// errCashCountChecklistItemInactive: the item to update is disabled at apply time.
var errCashCountChecklistItemInactive = errors.New("item checklist sudah nonaktif saat apply")

// CashCountChecklistItemApplier is the Applier for entity_type="cash_count_checklist_item".
type CashCountChecklistItemApplier struct{}

// Apply implements Applier.
func (CashCountChecklistItemApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		var p CashCountChecklistItemPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal checklist item create payload: %w", err)
		}
		row, err := q.CreateCashCountChecklistItem(ctx, db.CreateCashCountChecklistItemParams{
			Category: p.Category, Day: p.Day, PointNo: p.PointNo, ItemText: p.ItemText,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("create checklist item: %w", err)
		}
		return row.ID, row, nil

	case "update":
		var p CashCountChecklistItemUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal checklist item update payload: %w", err)
		}
		row, err := q.UpdateCashCountChecklistItem(ctx, db.UpdateCashCountChecklistItemParams{
			ID: *change.EntityID, Day: p.Day, PointNo: p.PointNo, ItemText: p.ItemText,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, errCashCountChecklistItemInactive
		}
		if err != nil {
			return 0, nil, fmt.Errorf("update checklist item: %w", err)
		}
		return row.ID, row, nil

	case "disable":
		if err := q.DisableCashCountChecklistItem(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable checklist item: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if err := q.EnableCashCountChecklistItem(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable checklist item: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=cash_count_checklist_item", change.Op)
	}
}

// CurrentState implements Applier: same row shape the service stores as Before.
func (CashCountChecklistItemApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetCashCountChecklistItemAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current checklist item state: %w", err)
	}
	return row, nil
}
