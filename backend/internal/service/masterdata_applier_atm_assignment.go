package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// pgExclusionViolation is atm_vendor_packages_no_overlap; the unique case
// ((atm_id, vendor_package_id, effective_start_date)) reuses pgUniqueViolation
// from vendor_request_actions.go.
const pgExclusionViolation = "23P01"

// ATMAssignmentApplier is the Applier for entity_type="atm_assignment" (T3.5).
// It is where the authoritative overlap guard bites: the exclusion constraint
// is re-checked inside the apply transaction, so two overlapping requests that
// were both pending cannot both land. A violation is returned as
// ErrATMAssignmentOverlap (never the raw DB error); the approval handler maps
// it to a 409.
type ATMAssignmentApplier struct{}

func assignmentDates(p ATMAssignmentUpdatePayload) (start, end pgtype.Date, err error) {
	s, err := time.Parse(assignmentDateLayout, p.EffectiveStartDate)
	if err != nil {
		return start, end, fmt.Errorf("parse effective_start_date %q: %w", p.EffectiveStartDate, err)
	}
	start = pgtype.Date{Time: s, Valid: true}
	if p.EffectiveEndDate != nil {
		e, err := time.Parse(assignmentDateLayout, *p.EffectiveEndDate)
		if err != nil {
			return start, end, fmt.Errorf("parse effective_end_date %q: %w", *p.EffectiveEndDate, err)
		}
		end = pgtype.Date{Time: e, Valid: true}
	}
	return start, end, nil
}

// mapAssignmentDBError translates the overlap/duplicate SQLSTATEs; other
// errors pass through wrapped with the operation name.
func mapAssignmentDBError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgExclusionViolation:
			return fmt.Errorf("%s atm assignment: %w", op, ErrATMAssignmentOverlap)
		case pgUniqueViolation:
			return fmt.Errorf("%s atm assignment: %w", op, ErrATMAssignmentDuplicate)
		}
	}
	return fmt.Errorf("%s atm assignment: %w", op, err)
}

// Apply implements Applier for entity_type="atm_assignment".
func (ATMAssignmentApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p ATMAssignmentPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal atm assignment create payload: %w", err)
		}
		start, end, err := assignmentDates(p.ATMAssignmentUpdatePayload)
		if err != nil {
			return 0, nil, err
		}
		created, err := q.CreateATMAssignmentAdmin(ctx, db.CreateATMAssignmentAdminParams{
			AtmID: p.ATMID, VendorPackageID: p.VendorPackageID, EffectiveStartDate: start, EffectiveEndDate: end,
		})
		if err != nil {
			return 0, nil, mapAssignmentDBError("create", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p ATMAssignmentUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal atm assignment update payload: %w", err)
		}
		start, end, err := assignmentDates(p)
		if err != nil {
			return 0, nil, err
		}
		updated, err := q.UpdateATMAssignmentAdmin(ctx, db.UpdateATMAssignmentAdminParams{
			ID: *change.EntityID, VendorPackageID: p.VendorPackageID, EffectiveStartDate: start, EffectiveEndDate: end,
		})
		if err != nil {
			return 0, nil, mapAssignmentDBError("update", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableATMAssignment(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable atm assignment: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableATMAssignment(ctx, *change.EntityID); err != nil {
			return 0, nil, mapAssignmentDBError("enable", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=atm_assignment", change.Op)
	}
}

// CurrentState implements Applier: same db.GetATMAssignmentAdminByIDRow shape
// ATMAssignmentAdminService uses for SubmitRequest.Before.
func (ATMAssignmentApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetATMAssignmentAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current atm assignment state: %w", err)
	}
	return row, nil
}
