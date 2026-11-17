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

// errCashCountAnomalyCategoryInactive: at apply time the row is gone or disabled
// (the guarded UPDATE matched nothing).
var errCashCountAnomalyCategoryInactive = errors.New("kategori ATM Anomaly tidak aktif saat apply")

// CashCountAnomalyCategoryApplier is the Applier for entity_type="cash_count_anomaly_category".
type CashCountAnomalyCategoryApplier struct{}

// Apply implements Applier.
func (CashCountAnomalyCategoryApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)
	if change.Op != "create" && change.EntityID == nil {
		return 0, nil, fmt.Errorf("%s requires entity_id", change.Op)
	}

	switch change.Op {
	case "create":
		var p CashCountAnomalyCategoryPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal anomaly category create payload: %w", err)
		}
		row, err := q.CreateCashCountAnomalyCategory(ctx, db.CreateCashCountAnomalyCategoryParams{Code: p.Code, Name: p.Name, Description: p.Description})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, nil, ErrCashCountAnomalyCategoryConflict
		}
		if err != nil {
			return 0, nil, fmt.Errorf("create anomaly category: %w", err)
		}
		return row.ID, row, nil

	case "update":
		var p CashCountAnomalyCategoryUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal anomaly category update payload: %w", err)
		}
		row, err := q.UpdateCashCountAnomalyCategory(ctx, db.UpdateCashCountAnomalyCategoryParams{ID: *change.EntityID, Name: p.Name, Description: p.Description})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, errCashCountAnomalyCategoryInactive
		}
		if err != nil {
			return 0, nil, fmt.Errorf("update anomaly category: %w", err)
		}
		return row.ID, row, nil

	case "disable":
		if err := q.DisableCashCountAnomalyCategory(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable anomaly category: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if err := q.EnableCashCountAnomalyCategory(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable anomaly category: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=%s", change.Op, cashCountAnomalyCategoryEntity)
	}
}

// CurrentState implements Applier: same row shape the service stores as Before.
func (CashCountAnomalyCategoryApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetCashCountAnomalyCategoryAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current anomaly category state: %w", err)
	}
	return row, nil
}
