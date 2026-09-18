//go:build integration

package rolemgmt

import (
	"context"
	"testing"
)

// TestHasPermission_DecisionComesOnlyFromMapping is Property 7: HasPermission
// returns true iff a role_permissions row links the exact (role, catalog
// key) pair -- never consulting a hardcoded role list. Uses the same
// outer-tx-rolled-back harness as service_integration_test.go.
func TestHasPermission_DecisionComesOnlyFromMapping(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	evaluator := NewPermissionEvaluator(svc.repo.(ReplicaReader))

	roleName := "TROLE-EVAL-" + tag
	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	// New role: no grants yet, so no permission for any key -- including a
	// key that exists in the catalog (dashboard) and one that doesn't.
	allowed, err := evaluator.HasPermission(ctx, roleName, "dashboard")
	if err != nil {
		t.Fatalf("HasPermission (before grant): %v", err)
	}
	if allowed {
		t.Fatal("HasPermission returned true for an un-granted role/key pair")
	}
	allowed, err = evaluator.HasPermission(ctx, roleName, "does-not-exist")
	if err != nil {
		t.Fatalf("HasPermission (unknown key): %v", err)
	}
	if allowed {
		t.Fatal("HasPermission returned true for a key that isn't even in the catalog")
	}

	// A role name that has never existed at all must also be false, not
	// error -- proves the decision is a plain mapping lookup, not a role
	// existence/hardcoded-list check.
	allowed, err = evaluator.HasPermission(ctx, "NEVER-EXISTED-ROLE-"+tag, "dashboard")
	if err != nil {
		t.Fatalf("HasPermission (nonexistent role): %v", err)
	}
	if allowed {
		t.Fatal("HasPermission returned true for a role that was never created")
	}

	// catalogIDs is ordered by key ASC ("cash-flow" < "dashboard"), so index
	// 1 is dashboard's id -- grant that one to match the "dashboard" key
	// assertions below.
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{catalogIDs[1]}, ""); err != nil {
		t.Fatalf("UpdateRolePermissions: %v", err)
	}

	allowed, err = evaluator.HasPermission(ctx, roleName, "dashboard")
	if err != nil {
		t.Fatalf("HasPermission (after grant): %v", err)
	}
	if !allowed {
		t.Fatal("HasPermission returned false for a role/key pair that was just granted")
	}

	// A different, non-granted key on the same now-permissioned role must
	// still be false -- the grant is per (role, key), not per role.
	allowed, err = evaluator.HasPermission(ctx, roleName, "cash-flow")
	if err != nil {
		t.Fatalf("HasPermission (sibling ungranted key): %v", err)
	}
	if allowed {
		t.Fatal("HasPermission returned true for a key never granted to this role")
	}
}
