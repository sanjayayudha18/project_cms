//go:build integration

package rolemgmt

import (
	"context"
	"net/http"
	"testing"
)

// role-management-revamp 5.4, against the real role_permissions (migration 047
// seeds D6): RequireVendorPermission with the real repository on each vendor key.
// Property 5 (seeded grants), Property 8 (revoke -> next request 403), Property 7
// (vendor role without any grant is denied, never passed).
func TestRequireVendorPermission_RealGrants(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()
	ev := NewPermissionEvaluator(svc.repo.(ReplicaReader))

	// Seeded D6 grants (047): vault user has no vendor-realizations.
	seeded := []struct {
		role, key string
		want      int
	}{
		{"VENDOR-USER", "vendor-dsr", http.StatusOK},
		{"VENDOR-USER", "vendor-orders", http.StatusOK},
		{"VENDOR-USER", "vendor-realizations", http.StatusOK},
		{"VENDOR-ATM-USER", "vendor-realizations", http.StatusOK},
		{"VENDOR-VAULT-USER", "vendor-dsr", http.StatusOK},
		{"VENDOR-VAULT-USER", "vendor-orders", http.StatusOK},
		{"VENDOR-VAULT-USER", "vendor-realizations", http.StatusForbidden},
		{"VENDOR-NEVER-CREATED-" + tag, "vendor-orders", http.StatusForbidden},
		{"ATM-USER", "vendor-realizations", http.StatusOK}, // internal: passes through
	}
	for _, tt := range seeded {
		router, ts, _ := mountVendorChain(t, RequireVendorPermission(ev, tt.key))
		if rec := doTestRequest(router, tokenForTestRole(t, ts, tt.role)); rec.Code != tt.want {
			t.Errorf("%s on %s: code = %d, want %d", tt.role, tt.key, rec.Code, tt.want)
		}
	}

	// Property 8: new vendor role, grant vendor-orders, then revoke.
	name := "VENDOR-T54-" + tag
	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN",
		CreateRoleRequest{Role: name, Portal: PortalVendor, PartyScope: []string{"replenish"}}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	catalog, err := svc.repo.ListCatalog(ctx)
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	var ordersID int64
	for _, f := range catalog {
		if f.Key == "vendor-orders" {
			ordersID = f.ID
		}
	}
	if ordersID == 0 {
		t.Fatal("vendor-orders missing from the catalog — is migration 047 applied?")
	}
	router, ts, _ := mountVendorChain(t, RequireVendorPermission(ev, "vendor-orders"))
	token := tokenForTestRole(t, ts, name)

	if rec := doTestRequest(router, token); rec.Code != http.StatusForbidden {
		t.Fatalf("new role without grants: code = %d, want 403", rec.Code)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{ordersID}, ""); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if rec := doTestRequest(router, token); rec.Code != http.StatusOK {
		t.Fatalf("after grant: code = %d, want 200", rec.Code)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{}, ""); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if rec := doTestRequest(router, token); rec.Code != http.StatusForbidden {
		t.Fatalf("after revoke (same token): code = %d, want 403", rec.Code)
	}
}
