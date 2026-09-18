//go:build integration

package rolemgmt

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// harness opens a real DB connection wrapped in an outer transaction rolled
// back on cleanup (same convention as
// internal/repository/vendor_admin_repository_test.go). The outer tx is
// itself passed as the Pool: pgx.Tx.Begin opens a savepoint-based pseudo
// nested transaction, which is exactly what PermissionService.pool.Begin
// needs — no DB/tx mocking required to exercise CreateRole/
// UpdateRolePermissions' real commit/rollback behavior.
func harness(t *testing.T) (*PermissionService, int64, []int64, string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin outer tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	var adminUserID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&adminUserID); err != nil {
		t.Fatalf("loading seeded admin user: %v", err)
	}

	rows, err := tx.Query(ctx, `SELECT id FROM menu_features WHERE key IN ('dashboard', 'cash-flow') ORDER BY key`)
	if err != nil {
		t.Fatalf("loading seeded catalog entries: %v", err)
	}
	var catalogIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning catalog id: %v", err)
		}
		catalogIDs = append(catalogIDs, id)
	}
	rows.Close()
	if len(catalogIDs) != 2 {
		t.Fatalf("expected 2 seeded catalog entries (dashboard, cash-flow), got %d — is migration 040 applied?", len(catalogIDs))
	}

	repo := NewRepository(tx, tx)
	svc := NewPermissionService(repo, tx)
	// Role names are restricted to [A-Z0-9-] (roleNameRe) -- no dots, so the
	// tag can't use time.Format's usual fractional-second dot separator.
	tag := strconv.FormatInt(time.Now().UnixNano(), 10)
	return svc, adminUserID, catalogIDs, tag
}

func TestPermissionService_ListRoles_And_ListCatalog(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-LIST-" + tag

	created, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	roles, err := svc.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	found := false
	for _, r := range roles {
		if r.RoleID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListRoles did not include the just-created role %q", roleName)
	}

	catalog, err := svc.ListCatalog(ctx)
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("ListCatalog returned an empty catalog, want the seeded menu_features")
	}
}

func TestCreateRole_NewRoleHasZeroAccessAndAuditTrail(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-" + tag

	created, err := svc.CreateRole(ctx, adminUserID, "APPACCESS", CreateRoleRequest{Role: roleName, Description: "test role"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if created.Role != roleName {
		t.Fatalf("created.Role = %q, want %q", created.Role, roleName)
	}

	// Property 1: new role has zero access.
	perms, err := svc.repo.ListRolePermissionIDs(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListRolePermissionIDs: %v", err)
	}
	if len(perms) != 0 {
		t.Fatalf("new role has %d permissions, want 0", len(perms))
	}

	// Property 4 (success branch): an audit_logs row exists for the create.
	var count int
	if err := queryRowScan(ctx, svc,
		`SELECT COUNT(*) FROM audit_logs WHERE entity_type = 'role' AND entity_id = $1 AND action = 'role_created'`,
		created.ID, &count); err != nil {
		t.Fatalf("counting audit_logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit_logs rows for role_created = %d, want 1", count)
	}
}

func TestCreateRole_NameConflictRejectsAndCreatesNoRow(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-DUP-" + tag

	if _, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, ""); err != nil {
		t.Fatalf("first CreateRole: %v", err)
	}

	// Property 2: duplicate name is rejected and creates no new row.
	_, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if !errors.Is(err, ErrRoleNameConflict) {
		t.Fatalf("second CreateRole error = %v, want ErrRoleNameConflict", err)
	}

	var count int
	if err := queryRowScan(ctx, svc, `SELECT COUNT(*) FROM roles WHERE role = $1`, roleName, &count); err != nil {
		t.Fatalf("counting roles: %v", err)
	}
	if count != 1 {
		t.Fatalf("roles named %q = %d rows, want exactly 1 (no duplicate persisted)", roleName, count)
	}
}

func TestCreateRole_ValidationRejectsEmptyAndBadFormatNames(t *testing.T) {
	svc, adminUserID, _, _ := harness(t)
	ctx := context.Background()

	cases := []struct {
		name string
		role string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"lowercase", "not-uppercase"},
		{"invalid chars", "BAD ROLE!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: tc.role}, "")
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("CreateRole(%q) error = %v, want *ValidationError", tc.role, err)
			}
		})
	}
}

// TestCreateRole_AuthorizationMatrix is Property 3: only APPACCESS/ADMIN may
// create a role, enforced at the service layer regardless of how the
// request reached it.
func TestCreateRole_AuthorizationMatrix(t *testing.T) {
	svc, adminUserID, _, tag := harness(t)
	ctx := context.Background()

	cases := []struct {
		role    string
		allowed bool
	}{
		{"APPACCESS", true},
		{"ADMIN", true},
		{"appaccess", true}, // case-insensitive, matches RequireRoles convention
		{"ADMIN_PARAM", false},
		{"ATM-USER", false},
		{"VENDOR-USER", false},
		{"", false},
	}
	for i, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			roleName := "TROLE-AUTH-" + tag + "-" + string(rune('A'+i))
			_, err := svc.CreateRole(ctx, adminUserID, tc.role, CreateRoleRequest{Role: roleName}, "")
			if tc.allowed {
				if err != nil {
					t.Fatalf("actor role %q: CreateRole error = %v, want nil", tc.role, err)
				}
				return
			}
			if !errors.Is(err, ErrNotAuthorized) {
				t.Fatalf("actor role %q: CreateRole error = %v, want ErrNotAuthorized", tc.role, err)
			}
			var count int
			if err := queryRowScan(ctx, svc, `SELECT COUNT(*) FROM roles WHERE role = $1`, roleName, &count); err != nil {
				t.Fatalf("counting roles: %v", err)
			}
			if count != 0 {
				t.Fatalf("unauthorized actor %q still created a role row", tc.role)
			}
		})
	}
}

// TestCreateRole_AuditFailureRollsBack is Property 4: an audit-log write
// that fails (here: a non-existent actor_id violating audit_logs_actor_fk)
// must roll back the role insert, so no role exists without an audit trail.
func TestCreateRole_AuditFailureRollsBack(t *testing.T) {
	svc, _, _, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-NOAUDIT-" + tag
	const nonExistentActorID = int64(999999999)

	_, err := svc.CreateRole(ctx, nonExistentActorID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err == nil {
		t.Fatal("CreateRole with a non-existent actor_id succeeded, want an error (audit FK violation)")
	}

	var count int
	if err := queryRowScan(ctx, svc, `SELECT COUNT(*) FROM roles WHERE role = $1`, roleName, &count); err != nil {
		t.Fatalf("counting roles: %v", err)
	}
	if count != 0 {
		t.Fatalf("role %q persisted despite the audit write failing — rollback did not happen", roleName)
	}
}

// TestUpdateRolePermissions_FullSetReplace is Property 5: after the update,
// the role's mapped entries equal exactly the submitted set.
func TestUpdateRolePermissions_FullSetReplace(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-PERM-" + tag

	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	got, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{catalogIDs[0]}, "")
	if err != nil {
		t.Fatalf("UpdateRolePermissions (grant one): %v", err)
	}
	assertInt64Set(t, got, []int64{catalogIDs[0]})
	assertPersistedPermissions(t, ctx, svc, role.ID, []int64{catalogIDs[0]})

	got, err = svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, catalogIDs, "")
	if err != nil {
		t.Fatalf("UpdateRolePermissions (grant both): %v", err)
	}
	assertInt64Set(t, got, catalogIDs)
	assertPersistedPermissions(t, ctx, svc, role.ID, catalogIDs)

	// Replacing with an empty set revokes everything (full-set replace, not additive).
	got, err = svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, nil, "")
	if err != nil {
		t.Fatalf("UpdateRolePermissions (revoke all): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UpdateRolePermissions(nil) = %v, want empty", got)
	}
	assertPersistedPermissions(t, ctx, svc, role.ID, nil)
}

// TestUpdateRolePermissions_InvalidCatalogRefLeavesMappingUnchanged is
// Property 6: an unknown catalog id rejects the whole request atomically.
func TestUpdateRolePermissions_InvalidCatalogRefLeavesMappingUnchanged(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-BADREF-" + tag
	const nonExistentCatalogID = int64(999999999)

	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{catalogIDs[0]}, ""); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	_, err = svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{catalogIDs[0], catalogIDs[1], nonExistentCatalogID}, "")
	if !errors.Is(err, ErrCatalogEntryNotFound) {
		t.Fatalf("UpdateRolePermissions with unknown id: err = %v, want ErrCatalogEntryNotFound", err)
	}

	assertPersistedPermissions(t, ctx, svc, role.ID, []int64{catalogIDs[0]})
}

func TestUpdateRolePermissions_RoleNotFound(t *testing.T) {
	svc, adminUserID, catalogIDs, _ := harness(t)
	ctx := context.Background()

	_, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", 999999999, catalogIDs, "")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("UpdateRolePermissions(nonexistent role): err = %v, want ErrRoleNotFound", err)
	}
}

// TestUpdateRolePermissions_AuthorizationMatrix is Property 3 for the
// permission-edit path.
func TestUpdateRolePermissions_AuthorizationMatrix(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-PERMAUTH-" + tag
	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	cases := []struct {
		role    string
		allowed bool
	}{
		{"APPACCESS", true},
		{"ADMIN", true},
		{"ADMIN_PARAM", false},
		{"ATM-USER", false},
		{"VENDOR-USER", false},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			_, err := svc.UpdateRolePermissions(ctx, adminUserID, tc.role, role.ID, catalogIDs, "")
			if tc.allowed {
				if err != nil {
					t.Fatalf("actor role %q: UpdateRolePermissions error = %v, want nil", tc.role, err)
				}
				return
			}
			if !errors.Is(err, ErrNotAuthorized) {
				t.Fatalf("actor role %q: UpdateRolePermissions error = %v, want ErrNotAuthorized", tc.role, err)
			}
		})
	}
}

// TestUpdateRolePermissions_AuditFailureRollsBack is Property 4 for the
// permission-edit path.
func TestUpdateRolePermissions_AuditFailureRollsBack(t *testing.T) {
	svc, adminUserID, catalogIDs, tag := harness(t)
	ctx := context.Background()
	roleName := "TROLE-PERMNOAUDIT-" + tag
	const nonExistentActorID = int64(999999999)

	role, err := svc.CreateRole(ctx, adminUserID, "ADMIN", CreateRoleRequest{Role: roleName}, "")
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if _, err := svc.UpdateRolePermissions(ctx, adminUserID, "ADMIN", role.ID, []int64{catalogIDs[0]}, ""); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	_, err = svc.UpdateRolePermissions(ctx, nonExistentActorID, "ADMIN", role.ID, catalogIDs, "")
	if err == nil {
		t.Fatal("UpdateRolePermissions with a non-existent actor_id succeeded, want an error (audit FK violation)")
	}

	assertPersistedPermissions(t, ctx, svc, role.ID, []int64{catalogIDs[0]})
}

func assertPersistedPermissions(t *testing.T, ctx context.Context, svc *PermissionService, roleID int64, want []int64) {
	t.Helper()
	got, err := svc.repo.ListRolePermissionIDs(ctx, roleID)
	if err != nil {
		t.Fatalf("ListRolePermissionIDs: %v", err)
	}
	assertInt64Set(t, got, want)
}

func assertInt64Set(t *testing.T, got, want []int64) {
	t.Helper()
	gotSet := distinctInt64s(got)
	wantSet := distinctInt64s(want)
	if len(gotSet) != len(wantSet) {
		t.Fatalf("set = %v, want %v", got, want)
	}
	index := make(map[int64]struct{}, len(wantSet))
	for _, id := range wantSet {
		index[id] = struct{}{}
	}
	for _, id := range gotSet {
		if _, ok := index[id]; !ok {
			t.Fatalf("set = %v, want %v", got, want)
		}
	}
}

// queryRowScan runs a one-off scalar assertion query directly through the
// service's underlying pool (a pgx.Tx in these tests, which also satisfies
// db.DBTX), avoiding a second DB connection per assertion. The last element
// of args is the scan destination; the rest are query parameters.
func queryRowScan(ctx context.Context, svc *PermissionService, sql string, args ...any) error {
	dest := args[len(args)-1]
	queryArgs := args[:len(args)-1]
	dbtx := svc.pool.(db.DBTX)
	return dbtx.QueryRow(ctx, sql, queryArgs...).Scan(dest)
}
