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

// Apply-time guard failures (state changed between submit and approve) → 409.
var (
	errCashCountAreaRegionInvalid = errors.New("region tidak dipakai cabang vendor aktif, area nonaktif, atau region sudah dimiliki area lain saat apply")
	errCashCountAreaInactive      = errors.New("area cash count nonaktif saat apply")
)

// CashCountAreaApplier is the Applier for entity_type="cash_count_area" (ScDD 5.1, AC-5.1-08).
type CashCountAreaApplier struct{}

// Apply implements Applier.
func (CashCountAreaApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		var p CashCountAreaPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal cash count area create payload: %w", err)
		}
		row, err := q.CreateCashCountArea(ctx, db.CreateCashCountAreaParams{Code: p.Code, Name: p.Name})
		if cashCountAreaUniqueViolation(err) {
			return 0, nil, ErrCashCountAreaConflict
		}
		if err != nil {
			return 0, nil, fmt.Errorf("create cash count area: %w", err)
		}
		return writeCashCountAreaRegions(ctx, q, row.ID, change.MakerID, p.RegionCodes)

	case "update":
		var p CashCountAreaUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal cash count area update payload: %w", err)
		}
		id := *change.EntityID
		if _, err := q.UpdateCashCountArea(ctx, db.UpdateCashCountAreaParams{ID: id, Name: p.Name}); errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, errCashCountAreaInactive
		} else if err != nil {
			return 0, nil, fmt.Errorf("update cash count area: %w", err)
		}
		if err := q.DeleteCashCountAreaRegions(ctx, id); err != nil {
			return 0, nil, fmt.Errorf("clear cash count area regions: %w", err)
		}
		return writeCashCountAreaRegions(ctx, q, id, change.MakerID, p.RegionCodes)

	case "disable":
		id := *change.EntityID
		n, err := q.DisableCashCountArea(ctx, id)
		if err != nil {
			return 0, nil, fmt.Errorf("disable cash count area: %w", err)
		}
		if n == 0 {
			return 0, nil, ErrCashCountAreaHasExecutors
		}
		// PK(region_code) would otherwise keep the regions locked to a disabled area.
		if err := q.DeleteCashCountAreaRegions(ctx, id); err != nil {
			return 0, nil, fmt.Errorf("clear cash count area regions: %w", err)
		}
		return id, map[string]bool{"is_active": false}, nil

	case "enable":
		if err := q.EnableCashCountArea(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable cash count area: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=cash_count_area", change.Op)
	}
}

// writeCashCountAreaRegions inserts the area's links through the guarded INSERT and returns the
// reloaded row (same shape as CurrentState) as the "after" state.
func writeCashCountAreaRegions(ctx context.Context, q *db.Queries, areaID, makerID int64, regionCodes []string) (int64, any, error) {
	for _, rc := range regionCodes {
		n, err := q.InsertCashCountAreaRegion(ctx, db.InsertCashCountAreaRegionParams{RegionCode: rc, CreatedBy: makerID, AreaID: areaID})
		if cashCountAreaUniqueViolation(err) {
			return 0, nil, errCashCountAreaRegionInvalid
		}
		if err != nil {
			return 0, nil, fmt.Errorf("insert cash count area region %s: %w", rc, err)
		}
		if n == 0 {
			return 0, nil, errCashCountAreaRegionInvalid
		}
	}
	after, err := q.GetCashCountAreaAdminByID(ctx, areaID)
	if err != nil {
		return 0, nil, fmt.Errorf("reload cash count area: %w", err)
	}
	return areaID, after, nil
}

func cashCountAreaUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// CurrentState implements Applier: same row shape the service stores as Before.
func (CashCountAreaApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetCashCountAreaAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current cash count area state: %w", err)
	}
	return row, nil
}
