//go:build integration

package rolemgmt

import (
	"context"
	"errors"
	"testing"
)

// role-management-revamp task 3.4 (Req 2.2-2.4, 6.3, 7.1, 7.3, 9.2).

func vendorKeyID(t *testing.T, ctx context.Context, svc *PermissionService) int64 {
	t.Helper()
	var id int64
	if err := queryRowScan(ctx, svc, `SELECT id FROM menu_features WHERE key = 'vendor-orders'`, &id); err != nil {
		t.Fatalf("loading vendor-orders catalog id: %v", err)
	}
	return id
}

// CreateRole persists portal + sorted/deduped scope and the audit row records them.
func TestCreateRole_VendorPersistsPortalScopeAndAudit(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()

	created, err := svc.CreateRole(ctx, adminUserID, "ADMIN",
		CreateRoleRequest{Role: "VENDOR-T-" + tag, Portal: PortalVendor, PartyScope: []string{"vault", "replenish", "vault"}}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if created.Portal != PortalVendor || len(created.PartyScope) != 2 || created.PartyScope[0] != "replenish" || created.PartyScope[1] != "vault" {
		t.Fatalf("stored portal/scope = %q %v", created.Portal, created.PartyScope)
	}
	var withPortal int
	if err := queryRowScan(ctx, svc,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type='role' AND entity_id=$1 AND action='role_created' AND after->>'portal' = 'vendor'`,
		created.ID, &withPortal); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if withPortal != 1 {
		t.Fatalf("role_created audit rows carrying portal = %d, want 1", withPortal)
	}
}

// Invalid portal x prefix x scope: 422-class error, no row.
func TestCreateRole_InvalidPortalScopeCreatesNothing(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()
	cases := []struct {
		name string
		role string
		req  CreateRoleRequest
	}{
		{"vendor without scope", "VENDOR-NS-" + tag, CreateRoleRequest{Portal: PortalVendor}},
		{"main with prefix", "VENDOR-MP-" + tag, CreateRoleRequest{Portal: PortalMain}},
		{"main with scope", "TROLE-MS-" + tag, CreateRoleRequest{Portal: PortalMain, PartyScope: []string{"vault"}}},
		{"vendor without prefix", "TROLE-VP-" + tag, CreateRoleRequest{Portal: PortalVendor, PartyScope: []string{"vault"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.Role = tc.role
			_, err := svc.CreateRole(ctx, adminUserID, "ADMIN", tc.req, "")
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error = %v, want *ValidationError", err)
			}
			var n int
			if err := queryRowScan(ctx, svc, `SELECT COUNT(*) FROM roles WHERE role = $1`, tc.role, &n); err != nil || n != 0 {
				t.Fatalf("roles rows = %d (err %v), want 0", n, err)
			}
		})
	}
}

// Property 4: a cross-portal grant is rejected (ErrRoleNotAssignable -> 422) and
// role_permissions stays exactly as before, in both directions.
func TestUpdateRolePermissions_CrossPortalRejectedMappingUnchanged(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	vKey := vendorKeyID(t, ctx, svc)

	mainRole, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: "TROLE-XP-" + tag, Portal: PortalMain}, "")
	if err != nil {
		t.Fatalf("CreateRole main: %v", err)
	}
	vendorRole, err := svc.CreateRole(ctx, adminUserID, "ADMIN",
		CreateRoleRequest{Role: "VENDOR-XP-" + tag, Portal: PortalVendor, PartyScope: []string{"replenish"}}, "")
	if err != nil {
		t.Fatalf("CreateRole vendor: %v", err)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", mainRole.ID, []int64{catalogIDs[0]}, ""); err != nil {
		t.Fatalf("seed main grant: %v", err)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", vendorRole.ID, []int64{vKey}, ""); err != nil {
		t.Fatalf("seed vendor grant: %v", err)
	}

	_, err = svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", mainRole.ID, []int64{catalogIDs[0], vKey}, "")
	if !errors.Is(err, ErrRoleNotAssignable) {
		t.Fatalf("main role + vendor key: err = %v, want ErrRoleNotAssignable", err)
	}
	assertPersistedPermissions(t, ctx, svc, mainRole.ID, []int64{catalogIDs[0]})

	_, err = svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", vendorRole.ID, []int64{vKey, catalogIDs[0]}, "")
	if !errors.Is(err, ErrRoleNotAssignable) {
		t.Fatalf("vendor role + main key: err = %v, want ErrRoleNotAssignable", err)
	}
	assertPersistedPermissions(t, ctx, svc, vendorRole.ID, []int64{vKey})
}

// ADMIN_PARAM is refused at the service layer for both mutations (route layer: handler RBAC test).
func TestAdminParamForbiddenAtService(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: "TROLE-AP-" + tag, Portal: PortalMain}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if _, err := svc.CreateRole(ctx, adminUserID, "ADMIN_PARAM", CreateRoleRequest{Role: "TROLE-AP2-" + tag, Portal: PortalMain}, ""); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("CreateRole err = %v, want ErrNotAuthorized", err)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN_PARAM", role.ID, catalogIDs, ""); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("UpdateRolePermissions err = %v, want ErrNotAuthorized", err)
	}
	assertPersistedPermissions(t, ctx, svc, role.ID, nil)
}

// active_user_count: 0 for a fresh role; for ADMIN equals active, non-deleted users holding it.
func TestListRoles_ActiveUserCount(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()
	fresh, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: "TROLE-CNT-" + tag, Portal: PortalMain}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	var adminRoleID, wantAdmin int64
	if err := queryRowScan(ctx, svc, `SELECT id FROM roles WHERE role = 'ADMIN'`, &adminRoleID); err != nil {
		t.Fatalf("admin role id: %v", err)
	}
	if err := queryRowScan(ctx, svc,
		`SELECT COUNT(*) FROM users WHERE role_id = $1 AND is_active = true AND deleted_at IS NULL`, adminRoleID, &wantAdmin); err != nil {
		t.Fatalf("count admin users: %v", err)
	}

	rows, err := svc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	var gotFresh, gotAdmin int64 = -1, -1
	for _, r := range rows {
		switch r.RoleID {
		case fresh.ID:
			gotFresh = r.ActiveUserCount
		case adminRoleID:
			gotAdmin = r.ActiveUserCount
		}
	}
	if gotFresh != 0 || gotAdmin != wantAdmin || wantAdmin < 1 {
		t.Fatalf("active_user_count fresh=%d admin=%d (want 0 and %d>=1)", gotFresh, gotAdmin, wantAdmin)
	}
}
