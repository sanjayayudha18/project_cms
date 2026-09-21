package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ATMApplier is the Applier for entity_type="atm" (T4.2): the only place an
// atms row is created/updated/disabled/enabled from the admin API. terminal_id
// is written on create only -- UpdateATMAdmin never touches it, and the update
// payload has no such field, so the immutability holds at the write layer too,
// not just in ATMAdminService.
type ATMApplier struct{}

// atmMoney parses the three optional money strings of an update payload to
// numeric (nil/blank => SQL NULL). Never goes through float.
func atmMoney(p atmUpdatePayload) (capacity, low, critical pgtype.Numeric, err error) {
	for _, f := range []struct {
		dst *pgtype.Numeric
		src *string
	}{{&capacity, p.CapacityAmount}, {&low, p.LowThresholdAmount}, {&critical, p.CriticalThresholdAmount}} {
		if *f.dst, err = numericFromDecimalPtr(f.src); err != nil {
			return
		}
	}
	return
}

// Apply implements Applier for entity_type="atm".
func (ATMApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p atmCreatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal atm create payload: %w", err)
		}
		capacity, low, critical, err := atmMoney(p.atmUpdatePayload)
		if err != nil {
			return 0, nil, err
		}
		created, err := q.CreateATMAdmin(ctx, db.CreateATMAdminParams{
			TerminalID: p.TerminalID, LocationID: p.LocationID, MachineType: p.MachineType, Brand: p.Brand, Model: p.Model,
			OperationHours: p.OperationHours, DeploymentType: p.DeploymentType,
			CapacityAmount: capacity, LowThresholdAmount: low, CriticalThresholdAmount: critical,
			Blacklisted: p.Blacklisted, EscrowAccount: p.EscrowAccount, PriorityClass: p.PriorityClass,
		})
		if err != nil {
			if isUniqueViolation(err) {
				// Two creates for the same terminal_id can both be pending; the DB
				// constraint decides at apply time. Clean error, not the raw one.
				return 0, nil, fmt.Errorf("create atm: %w", ErrATMTerminalIDConflict)
			}
			return 0, nil, fmt.Errorf("create atm: %w", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p atmUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal atm update payload: %w", err)
		}
		capacity, low, critical, err := atmMoney(p)
		if err != nil {
			return 0, nil, err
		}
		updated, err := q.UpdateATMAdmin(ctx, db.UpdateATMAdminParams{
			ID: *change.EntityID, LocationID: p.LocationID, MachineType: p.MachineType, Brand: p.Brand, Model: p.Model,
			OperationHours: p.OperationHours, DeploymentType: p.DeploymentType,
			CapacityAmount: capacity, LowThresholdAmount: low, CriticalThresholdAmount: critical,
			Blacklisted: p.Blacklisted, EscrowAccount: p.EscrowAccount, PriorityClass: p.PriorityClass,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("update atm: %w", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableATM(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable atm: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableATM(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable atm: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=atm", change.Op)
	}
}

// CurrentState implements Applier: same db.GetATMAdminByIDRow shape
// ATMAdminService uses for SubmitRequest.Before on update/disable/enable.
func (ATMApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetATMAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current atm state: %w", err)
	}
	return row, nil
}
