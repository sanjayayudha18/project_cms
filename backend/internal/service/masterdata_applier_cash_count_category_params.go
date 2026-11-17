package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// errCashCountCategoryParamsSuperseded: another set became active between
// submit and approve; applying would silently override it.
var errCashCountCategoryParamsSuperseded = errors.New("set parameter aktif sudah berubah sejak pengajuan; ajukan ulang")

// CashCountCategoryParamsApplier is the Applier for
// entity_type="cash_count_category_params" (ScDD 5.1, AC-5.1-04): op=create
// closes the active set and inserts the new one in the approve tx, so exactly
// one set is active and old sets stay as history.
type CashCountCategoryParamsApplier struct{}

// Apply implements Applier.
func (CashCountCategoryParamsApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	if change.Op != "create" {
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=%s", change.Op, cashCountCategoryParamsEntity)
	}
	var p CashCountCategoryParamsPayload
	if err := json.Unmarshal(change.Payload, &p); err != nil {
		return 0, nil, fmt.Errorf("unmarshal cash count category params payload: %w", err)
	}
	q := db.New(tx)

	active, err := q.GetActiveCashCountCategoryParams(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if p.SupersedesID != 0 {
			return 0, nil, errCashCountCategoryParamsSuperseded
		}
	case err != nil:
		return 0, nil, fmt.Errorf("load active cash count category params: %w", err)
	case active.ID != p.SupersedesID:
		return 0, nil, errCashCountCategoryParamsSuperseded
	}

	arg, err := cashCountCategoryParamsCreateArg(p, change.MakerID)
	if err != nil {
		return 0, nil, err
	}
	if err := q.SupersedeActiveCashCountCategoryParams(ctx); err != nil {
		return 0, nil, fmt.Errorf("supersede active cash count category params: %w", err)
	}
	row, err := q.CreateCashCountCategoryParams(ctx, arg)
	if isUniqueViolation(err) {
		// A concurrent approve inserted a set after our supersede (active-unique index).
		return 0, nil, errCashCountCategoryParamsSuperseded
	}
	if err != nil {
		return 0, nil, fmt.Errorf("create cash count category params: %w", err)
	}
	return row.ID, row, nil
}

// CurrentState implements Applier. Only op=create exists (the engine never
// calls it for create); returns the active set for completeness.
func (CashCountCategoryParamsApplier) CurrentState(ctx context.Context, tx pgx.Tx, _ int64) (any, error) {
	row, err := db.New(tx).GetActiveCashCountCategoryParams(ctx)
	if err != nil {
		return nil, fmt.Errorf("load current cash count category params: %w", err)
	}
	return row, nil
}

func cashCountCategoryParamsCreateArg(p CashCountCategoryParamsPayload, makerID int64) (db.CreateCashCountCategoryParamsParams, error) {
	var minN, maxN pgtype.Numeric
	if err := minN.Scan(p.MediumMinAmount); err != nil {
		return db.CreateCashCountCategoryParamsParams{}, fmt.Errorf("parse medium_min_amount %q: %w", p.MediumMinAmount, err)
	}
	if err := maxN.Scan(p.MediumMaxAmount); err != nil {
		return db.CreateCashCountCategoryParamsParams{}, fmt.Errorf("parse medium_max_amount %q: %w", p.MediumMaxAmount, err)
	}
	return db.CreateCashCountCategoryParamsParams{
		MediumMinAmount: minN,
		MediumMaxAmount: maxN,
		OnsiteHigh:      p.OnsiteHigh,
		OnsiteMedium:    p.OnsiteMedium,
		OnsiteLow:       p.OnsiteLow,
		CreatedBy:       makerID,
	}, nil
}
