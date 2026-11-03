// Package service: VendorPicRoleService backs PUT
// /api/v1/admin/vendors/{vendorID}/pics/{id}/roles (vendor-party-scope Req
// 5.3-5.5). Like RegionAdminService it is immediate-apply-with-audit -- a
// documented deviation from Golden Rule #3: no maker-checker, but the change
// and its audit_logs row share one transaction, so a failed audit write rolls
// back the roles. Other PIC fields stay maker-checker (VendorPicAdminService).
package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// picRoleCodes are the allowed vendor_pics.roles values (migration 032 CHECK).
var picRoleCodes = []string{"cashcount", "replenish", "vault"}

// AllPicRoles is the default role set of a PIC created without roles (Req 5.6).
func AllPicRoles() []string { return slices.Clone(picRoleCodes) }

// normalizePicRoles trims each code, rejects unknown or duplicate codes, and
// returns them sorted. Empty is allowed (PIC is a contact only, Req 5.1).
func normalizePicRoles(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		code := strings.TrimSpace(r)
		if !slices.Contains(picRoleCodes, code) {
			return nil, &ValidationError{Field: "roles", Message: fmt.Sprintf("peran %q tidak dikenal (replenish, vault, cashcount)", code)}
		}
		if slices.Contains(out, code) {
			return nil, &ValidationError{Field: "roles", Message: fmt.Sprintf("peran %q duplikat", code)}
		}
		out = append(out, code)
	}
	slices.Sort(out)
	return out, nil
}

// VendorPicRolePool is the transaction-opening surface VendorPicRoleService
// needs. *pgxpool.Pool satisfies it.
type VendorPicRolePool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// VendorPicRoleService sets a PIC's roles immediately with an audit entry in
// the same transaction.
type VendorPicRoleService struct {
	pool VendorPicRolePool
}

// NewVendorPicRoleService creates a VendorPicRoleService.
func NewVendorPicRoleService(pool VendorPicRolePool) *VendorPicRoleService {
	return &VendorPicRoleService{pool: pool}
}

// SetRoles replaces the roles of vendorID's active PIC picID. The caller in
// ctx must be actorID holding ADMIN/ADMIN_PARAM (ErrMasterDataForbidden);
// a missing, disabled, or foreign PIC is ErrVendorPicNotFound. An unchanged
// set succeeds without an audit entry.
func (s *VendorPicRoleService) SetRoles(ctx context.Context, actorID, vendorID, picID int64, roles []string, actorIP string) ([]string, error) {
	if err := authorizeMasterDataMaker(ctx, actorID); err != nil {
		return nil, err
	}
	after, err := normalizePicRoles(roles)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	before, err := q.GetVendorPicRolesForUpdate(ctx, db.GetVendorPicRolesForUpdateParams{ID: picID, VendorID: vendorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVendorPicNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading vendor pic roles: %w", err)
	}
	if slices.Equal(before, after) {
		return after, nil
	}

	if _, err := q.SetVendorPicRoles(ctx, db.SetVendorPicRolesParams{Roles: after, ID: picID, VendorID: vendorID}); err != nil {
		return nil, fmt.Errorf("setting vendor pic roles: %w", err)
	}
	if err := audit.NewWriter(tx).Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "vendor_pic_roles_updated",
		EntityType: "vendor_pic",
		EntityID:   picID,
		Before:     map[string]any{"roles": before},
		After:      map[string]any{"roles": after},
		IP:         actorIP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit vendor pic roles: %w", err)
	}
	return after, nil
}
