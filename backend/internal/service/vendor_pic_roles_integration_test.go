//go:build integration

package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// createPicViaApproval stages a vendor_pic create and approves it, returning the new PIC's id.
func createPicViaApproval(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, vendorID int64, name string, roles *[]string) int64 {
	t.Helper()
	if err := approveChange(t, svc, h.insertPending("vendor_pic", "create", nil, VendorPicPayload{
		VendorID: vendorID, VendorPicUpdatePayload: VendorPicUpdatePayload{Name: name, Roles: roles},
	})); err != nil {
		t.Fatalf("approve create %s: %v", name, err)
	}
	var id int64
	if err := dbtx(svc).QueryRow(context.Background(), `SELECT id FROM vendor_pics WHERE vendor_id = $1 AND name = $2`, vendorID, name).Scan(&id); err != nil {
		t.Fatalf("load created pic %s: %v", name, err)
	}
	h.vendorPicIDs = append(h.vendorPicIDs, id)
	return id
}

func picRoles(t *testing.T, svc *MasterDataApprovalService, picID int64) []string {
	t.Helper()
	var roles []string
	if err := dbtx(svc).QueryRow(context.Background(), `SELECT roles FROM vendor_pics WHERE id = $1`, picID).Scan(&roles); err != nil {
		t.Fatalf("load pic roles: %v", err)
	}
	return roles
}

func picRolesAuditCount(t *testing.T, svc *MasterDataApprovalService, picID int64) int {
	t.Helper()
	var n int
	if err := dbtx(svc).QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_pic' AND entity_id = $1 AND action = 'vendor_pic_roles_updated'`, picID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return n
}

// Req 5.6: create via maker-checker without roles = all three; with roles = those roles.
func TestVendorPicApplier_CreateRoles_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	vendorID := h.seedVendor("ITPR-"+tag, "Vendor PIC Roles "+tag)

	if got := picRoles(t, svc, createPicViaApproval(t, svc, h, vendorID, "Default "+tag, nil)); !slices.Equal(got, AllPicRoles()) {
		t.Errorf("create without roles: roles = %q, want all three", got)
	}
	if got := picRoles(t, svc, createPicViaApproval(t, svc, h, vendorID, "Vault "+tag, &[]string{"vault"})); !slices.Equal(got, []string{"vault"}) {
		t.Errorf("create with [vault]: roles = %q", got)
	}
	if got := picRoles(t, svc, createPicViaApproval(t, svc, h, vendorID, "Contact "+tag, &[]string{})); len(got) != 0 {
		t.Errorf("create with []: roles = %q, want empty", got)
	}
}

// Req 5.7: a form update (Roles nil) keeps the roles; an update carrying roles
// (CSV import path) sets them in the same apply tx.
func TestVendorPicApplier_UpdateRoles_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	vendorID := h.seedVendor("ITPU-"+tag, "Vendor PIC Update "+tag)
	picID := createPicViaApproval(t, svc, h, vendorID, "Upd "+tag, &[]string{"vault"})

	if err := approveChange(t, svc, h.insertPending("vendor_pic", "update", &picID, VendorPicUpdatePayload{Name: "Upd Renamed " + tag})); err != nil {
		t.Fatalf("form update: %v", err)
	}
	if got := picRoles(t, svc, picID); !slices.Equal(got, []string{"vault"}) {
		t.Errorf("after form update: roles = %q, want [vault]", got)
	}

	if err := approveChange(t, svc, h.insertPending("vendor_pic", "update", &picID,
		VendorPicUpdatePayload{Name: "Upd Renamed " + tag, Roles: &[]string{"cashcount", "replenish"}})); err != nil {
		t.Fatalf("update with roles: %v", err)
	}
	if got := picRoles(t, svc, picID); !slices.Equal(got, []string{"cashcount", "replenish"}) {
		t.Errorf("after update with roles: roles = %q", got)
	}
}

// Req 5.3-5.5, 10.1 (g): immediate roles change + audit in tx, its rejections,
// and its interaction with a pending PIC change request (decision A: roles are
// part of the Before snapshot, so the pending change goes stale).
func TestVendorPicRoleService_SetRoles_Integration(t *testing.T) {
	svc, tag, h := harness(t)
	roleSvc := NewVendorPicRoleService(h.pool)
	ctx := adminCtx(h.makerID)

	vendorID := h.seedVendor("ITPS-"+tag, "Vendor PIC Set "+tag)
	otherVendor := h.seedVendor("ITPSO-"+tag, "Other Vendor "+tag)
	picID := createPicViaApproval(t, svc, h, vendorID, "Set "+tag, nil)

	t.Run("success writes one audit", func(t *testing.T) {
		got, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, []string{" vault", "replenish"}, "127.0.0.1")
		if err != nil || !slices.Equal(got, []string{"replenish", "vault"}) {
			t.Fatalf("SetRoles = %q, %v", got, err)
		}
		if stored := picRoles(t, svc, picID); !slices.Equal(stored, []string{"replenish", "vault"}) {
			t.Errorf("stored roles = %q", stored)
		}
		if n := picRolesAuditCount(t, svc, picID); n != 1 {
			t.Errorf("audit rows = %d, want 1", n)
		}
	})

	t.Run("unchanged set writes no audit", func(t *testing.T) {
		if _, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, []string{"vault", "replenish"}, ""); err != nil {
			t.Fatalf("SetRoles: %v", err)
		}
		if n := picRolesAuditCount(t, svc, picID); n != 1 {
			t.Errorf("audit rows = %d, want still 1", n)
		}
	})

	t.Run("audit failure rolls back", func(t *testing.T) {
		const ghost = int64(-424242) // passes the role check, violates audit_logs actor FK
		if _, err := roleSvc.SetRoles(adminCtx(ghost), ghost, vendorID, picID, []string{"cashcount"}, ""); err == nil {
			t.Fatal("SetRoles with a non-existent actor succeeded, want audit error")
		}
		if got := picRoles(t, svc, picID); !slices.Equal(got, []string{"replenish", "vault"}) {
			t.Errorf("roles changed despite failed audit: %q", got)
		}
	})

	t.Run("invalid roles rejected", func(t *testing.T) {
		for _, roles := range [][]string{{"courier"}, {"vault", "vault"}} {
			_, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, roles, "")
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != "roles" {
				t.Errorf("roles %q: want roles ValidationError, got %v", roles, err)
			}
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		if _, err := roleSvc.SetRoles(ctxWithRole(h.makerID, "ATM-SPV"), h.makerID, vendorID, picID, []string{}, ""); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("want ErrMasterDataForbidden, got %v", err)
		}
	})

	t.Run("other vendor is not found", func(t *testing.T) {
		if _, err := roleSvc.SetRoles(ctx, h.makerID, otherVendor, picID, []string{}, ""); !errors.Is(err, ErrVendorPicNotFound) {
			t.Errorf("want ErrVendorPicNotFound, got %v", err)
		}
	})

	t.Run("pending update goes stale after a roles change", func(t *testing.T) {
		before, err := db.New(h.pool).GetVendorPicAdminByID(context.Background(), picID)
		if err != nil {
			t.Fatal(err)
		}
		changeID := h.insertPendingWithBefore("vendor_pic", "update", &picID, VendorPicUpdatePayload{Name: "Stale " + tag}, before)
		if _, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, []string{"vault"}, ""); err != nil {
			t.Fatalf("SetRoles: %v", err)
		}
		if err := approveChange(t, svc, changeID); !errors.Is(err, ErrMasterDataChangeStale) {
			t.Errorf("approve after roles change: want ErrMasterDataChangeStale, got %v", err)
		}
	})

	t.Run("disabled pic is not found", func(t *testing.T) {
		if err := approveChange(t, svc, h.insertPending("vendor_pic", "disable", &picID, nil)); err != nil {
			t.Fatalf("disable: %v", err)
		}
		if _, err := roleSvc.SetRoles(ctx, h.makerID, vendorID, picID, []string{}, ""); !errors.Is(err, ErrVendorPicNotFound) {
			t.Errorf("want ErrVendorPicNotFound, got %v", err)
		}
	})
}
