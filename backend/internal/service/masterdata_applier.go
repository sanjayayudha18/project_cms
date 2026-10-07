package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Applier performs the real create/update/disable/enable for one
// entity_type, inside the transaction the apply-on-approve hook (T2.4)
// opened. It receives the change request's raw Payload (already validated
// by whoever called MasterDataChangeService.Submit -- appliers do not
// re-run business validation, only the mechanical write) and must unmarshal
// it into whatever shape it expects for change.Op. Returns the affected
// entity's id (used for the audit entry -- change.EntityID is nil for
// op=create, since the entity doesn't exist until Apply runs) and its
// resulting state, for the audit "after" field.
type Applier interface {
	Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (entityID int64, after any, err error)

	// CurrentState returns entityID's present state, in the exact same JSON
	// shape a caller of MasterDataChangeService.Submit is expected to have
	// captured as SubmitRequest.Before for this entity_type. Used by the
	// T2.5 staleness check: if this differs from the change request's
	// Before snapshot, the entity moved since submit and the change is
	// marked stale rather than applied. Not called for op=create (nothing
	// to compare against yet).
	CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error)
}

// ApplierRegistry maps entity_type -> Applier. Each entity_type gets its
// applier registered as its own CRUD support lands (Fase 3/4) -- there is
// no requirement that every entity_type in Sec 3's DB map has one from day
// one; a missing entry surfaces as a clear "no applier registered" apply
// error rather than a silent no-op or a panic.
type ApplierRegistry map[string]Applier

// vendorCreatePayload/vendorUpdatePayload are the JSON shapes
// MasterDataChangeService.Submit callers are expected to send for
// entity_type="vendor" (mirrors VendorAdminService's CreateVendorRequest/
// UpdateVendorRequest field set; VendorAdminService builds these and stages
// them via Submit since T4.1).
//
// LegalName/NPWP (T4.3) are already resolved by VendorAdminService: NPWP is
// digits only (validated 15/16), "" means NULL. An update staged before these
// fields existed would unmarshal them as "" and write NULLs over real data --
// it is never applied, because its "before" snapshot lacks the keys and so
// fails the T2.5 staleness check (statesEqual compares whole maps); pinned by
// TestVendorSnapshotPreT43_IsStaleNotApplied.
type vendorCreatePayload struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	LegalName    string `json:"legal_name"`
	NPWP         string `json:"npwp"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	HqAddress    string `json:"hq_address"`
}

type vendorUpdatePayload struct {
	Name         string `json:"name"`
	LegalName    string `json:"legal_name"`
	NPWP         string `json:"npwp"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	HqAddress    string `json:"hq_address"`
}

// VendorApplier is the Applier for entity_type="vendor". It performs the
// only place a vendor row is created/updated/disabled/enabled from the admin
// API (tx-scoped, so T2.4 commits the entity write, the change-request status
// flip and the audit entry atomically).
type VendorApplier struct{}

// Apply implements Applier for entity_type="vendor".
func (VendorApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p vendorCreatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor create payload: %w", err)
		}
		created, err := q.CreateVendorAdmin(ctx, db.CreateVendorAdminParams{
			Code:         p.Code,
			Name:         p.Name,
			LegalName:    nilIfEmpty(p.LegalName),
			Npwp:         nilIfEmpty(p.NPWP),
			ContactEmail: nilIfEmpty(p.ContactEmail),
			ContactPhone: nilIfEmpty(p.ContactPhone),
			HqAddress:    nilIfEmpty(p.HqAddress),
		})
		if err != nil {
			if isUniqueViolation(err) {
				// Two creates for the same code can both be pending; the DB
				// constraint decides at apply time. Clean error, not the raw one.
				return 0, nil, fmt.Errorf("create vendor: %w", ErrVendorCodeConflict)
			}
			return 0, nil, fmt.Errorf("create vendor: %w", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p vendorUpdatePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor update payload: %w", err)
		}
		updated, err := q.UpdateVendorAdmin(ctx, db.UpdateVendorAdminParams{
			ID:           *change.EntityID,
			Name:         p.Name,
			LegalName:    nilIfEmpty(p.LegalName),
			Npwp:         nilIfEmpty(p.NPWP),
			ContactEmail: nilIfEmpty(p.ContactEmail),
			ContactPhone: nilIfEmpty(p.ContactPhone),
			HqAddress:    nilIfEmpty(p.HqAddress),
		})
		if err != nil {
			return 0, nil, fmt.Errorf("update vendor: %w", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableVendor(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": false}, nil

	case "enable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("enable requires entity_id")
		}
		if err := q.EnableVendor(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("enable vendor: %w", err)
		}
		return *change.EntityID, map[string]bool{"is_active": true}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor", change.Op)
	}
}

// CurrentState implements Applier for entity_type="vendor". Returns the
// same db.GetVendorAdminByIDRow shape callers of Submit are expected to use
// for SubmitRequest.Before on vendor update/disable/enable submissions.
func (VendorApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor state: %w", err)
	}
	return row, nil
}
