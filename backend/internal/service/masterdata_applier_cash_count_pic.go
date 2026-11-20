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

// errCashCountPicInvalid: at apply time the user is no longer eligible (inactive / not ldap /
// role changed) or the region/area is no longer valid (the guarded statement matched no row).
var errCashCountPicInvalid = errors.New("user atau target PIC tidak valid lagi saat apply")

// CashCountPicApplier is the Applier for entity_type="cash_count_pic" (ScDD 5.1, AC-5.1-09/10).
type CashCountPicApplier struct{}

// Apply implements Applier.
func (CashCountPicApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		var p CashCountPicPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal cash count pic create payload: %w", err)
		}
		row, err := q.CreateCashCountPic(ctx, db.CreateCashCountPicParams{
			PicRole: p.PicRole, UserID: p.UserID, RegionCode: p.RegionCode, AreaID: p.AreaID,
		})
		if err != nil {
			return 0, nil, mapCashCountPicErr("create", err)
		}
		return row.ID, row, nil

	case "update":
		var p CashCountPicUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal cash count pic update payload: %w", err)
		}
		row, err := q.UpdateCashCountPic(ctx, db.UpdateCashCountPicParams{
			ID: *change.EntityID, UserID: p.UserID, RegionCode: p.RegionCode, AreaID: p.AreaID,
		})
		if err != nil {
			return 0, nil, mapCashCountPicErr("update", err)
		}
		return row.ID, row, nil

	case "disable":
		if err := q.DisableCashCountPic(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable cash count pic: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		n, err := q.EnableCashCountPic(ctx, *change.EntityID)
		if err != nil {
			return 0, nil, mapCashCountPicErr("enable", err)
		}
		if n == 0 {
			return 0, nil, errCashCountPicInvalid
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=cash_count_pic", change.Op)
	}
}

// CurrentState implements Applier: same row shape the service stores as Before.
func (CashCountPicApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetCashCountPicAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current cash count pic state: %w", err)
	}
	return row, nil
}

// mapCashCountPicErr turns guard misses (no row) and the active-unique violation into the
// clear errors the approval flow reports as 409.
func mapCashCountPicErr(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errCashCountPicInvalid
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrCashCountPicConflict
	}
	return fmt.Errorf("%s cash count pic: %w", op, err)
}
