//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// acmAreaHarness: one outer tx rolled back on cleanup, used as primary, replica
// and pool (pgx.Tx.Begin = savepoint) -- same convention as regionHarness.
func acmAreaHarness(t *testing.T) (*AcmAreaAdminService, pgx.Tx, int64, string) {
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

	var adminID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&adminID); err != nil {
		t.Fatalf("loading seeded admin user: %v", err)
	}
	return NewAcmAreaAdminService(tx, tx), tx, adminID, strconv.FormatInt(time.Now().UnixNano(), 36)
}

func seedRow(t *testing.T, tx pgx.Tx, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("seed %q: %v", sql, err)
	}
	return id
}

func seedAcmUser(t *testing.T, tx pgx.Tx, role, username string) int64 {
	return seedRow(t, tx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $2, $2, $2 || '@test.local', true, 'ldap' FROM roles WHERE role = $1 RETURNING id`, role, username)
}

func countAudit(t *testing.T, tx pgx.Tx, areaID int64, action string) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE entity_type = 'acm_area' AND entity_id = $1 AND action = $2`, areaID, action).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAcmAreaAdmin_Lifecycle_Integration(t *testing.T) {
	svc, tx, adminID, tag := acmAreaHarness(t)
	ctx := context.Background()

	vendorID := seedRow(t, tx, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITACM-"+tag)
	br1 := seedRow(t, tx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2) RETURNING id`, vendorID, "B1-"+tag)
	br2 := seedRow(t, tx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2) RETURNING id`, vendorID, "B2-"+tag)
	acmUser := seedAcmUser(t, tx, "ACM-USER", "acmu-"+tag)
	acmSpv := seedAcmUser(t, tx, "ACM-SPV", "acms-"+tag)

	// RBAC: non-ADMIN refused before touching the DB.
	if _, err := svc.Create(ctx, adminID, "ADMIN_PARAM", "X", ""); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("non-ADMIN Create: want ErrNotAuthorized, got %v", err)
	}

	jkt, err := svc.Create(ctx, adminID, "ADMIN", "  Jabodetabek "+tag+" ", "")
	if err != nil || jkt.Name != "Jabodetabek "+tag {
		t.Fatalf("Create: %+v %v", jkt, err)
	}
	if countAudit(t, tx, jkt.ID, "acm_area_created") != 1 {
		t.Error("create not audited")
	}
	if _, err := svc.Create(ctx, adminID, "ADMIN", "JABODETABEK "+tag, ""); !errors.Is(err, ErrAcmAreaNameConflict) {
		t.Fatalf("duplicate name (case-insensitive): want conflict, got %v", err)
	}
	mks, err := svc.Create(ctx, adminID, "ADMIN", "Makassar "+tag, "")
	if err != nil {
		t.Fatal(err)
	}

	// Branches: unique across areas.
	d, err := svc.SetBranches(ctx, adminID, "ADMIN", jkt.ID, []int64{br1, br1, br2}, "")
	if err != nil || len(d.Branches) != 2 {
		t.Fatalf("SetBranches: %+v %v", d, err)
	}
	var conflict *AcmAreaBranchConflictError
	if _, err := svc.SetBranches(ctx, adminID, "ADMIN", mks.ID, []int64{br2}, ""); !errors.As(err, &conflict) || len(conflict.Conflicts) != 1 || conflict.Conflicts[0].AcmAreaID != jkt.ID {
		t.Fatalf("branch held by another area: want conflict naming area %d, got %v", jkt.ID, err)
	}
	var ve *ValidationError
	if _, err := svc.SetBranches(ctx, adminID, "ADMIN", jkt.ID, []int64{999999999}, ""); !errors.As(err, &ve) {
		t.Fatalf("unknown branch: want ValidationError, got %v", err)
	}

	// Members: only active ACM-USER / ACM-SPV.
	if _, err := svc.SetMembers(ctx, adminID, "ADMIN", jkt.ID, []int64{acmUser, adminID}, ""); !errors.As(err, &ve) {
		t.Fatalf("non-ACM member: want ValidationError, got %v", err)
	}
	if d, err = svc.SetMembers(ctx, adminID, "ADMIN", jkt.ID, []int64{acmUser, acmSpv}, ""); err != nil || len(d.Members) != 2 {
		t.Fatalf("SetMembers: %+v %v", d, err)
	}
	if _, err := svc.SetMembers(ctx, adminID, "ADMIN", mks.ID, []int64{acmUser}, ""); err != nil {
		t.Fatalf("same user in two areas: %v", err)
	}

	// Disable releases branches -> another area can take them; members stay.
	if _, err := svc.SetActive(ctx, adminID, "ADMIN", jkt.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetActive(ctx, adminID, "ADMIN", jkt.ID, false, ""); !errors.Is(err, ErrAcmAreaStatusUnchanged) {
		t.Fatalf("disable twice: want unchanged, got %v", err)
	}
	if _, err := svc.SetBranches(ctx, adminID, "ADMIN", jkt.ID, []int64{br1}, ""); !errors.Is(err, ErrAcmAreaInactive) {
		t.Fatalf("branches on inactive area: want ErrAcmAreaInactive, got %v", err)
	}
	if _, err := svc.SetBranches(ctx, adminID, "ADMIN", mks.ID, []int64{br2}, ""); err != nil {
		t.Fatalf("released branch reassigned: %v", err)
	}
	got, err := svc.Get(ctx, jkt.ID)
	if err != nil || len(got.Branches) != 0 || len(got.Members) != 2 || got.Area.IsActive {
		t.Fatalf("after disable: %+v %v", got, err)
	}
	// FR7.4 query runs against the real schema (no request is in vault_assignment before S5).
	if areas, err := svc.List(ctx, "all"); err != nil || len(areas) < 2 {
		t.Fatalf("List: %d %v", len(areas), err)
	}
	if users, err := svc.EligibleUsers(ctx); err != nil || len(users) < 2 {
		t.Fatalf("EligibleUsers: %d %v", len(users), err)
	}
	if opts, err := svc.BranchOptions(ctx); err != nil || len(opts) == 0 {
		t.Fatalf("BranchOptions: %d %v", len(opts), err)
	}
	if _, err := svc.Warnings(ctx); err != nil {
		t.Fatalf("Warnings: %v", err)
	}
	for _, a := range []string{"acm_area_branches_set", "acm_area_members_set", "acm_area_deactivated"} {
		if countAudit(t, tx, jkt.ID, a) < 1 {
			t.Errorf("%s not audited", a)
		}
	}
}

func TestAcmAreaAdmin_AuditFailureRollsBack_Integration(t *testing.T) {
	svc, _, adminID, tag := acmAreaHarness(t)
	ctx := context.Background()

	area, err := svc.Create(ctx, adminID, "ADMIN", "Before "+tag, "")
	if err != nil {
		t.Fatal(err)
	}
	// Rename writes no actor FK of its own; only audit_logs.actor_id -> users
	// fails for actor -1, so the rename must roll back with it.
	if _, err := svc.Rename(ctx, -1, "ADMIN", area.ID, "After "+tag, ""); err == nil {
		t.Fatal("want audit write failure")
	}
	got, err := svc.Get(ctx, area.ID)
	if err != nil || got.Area.Name != "Before "+tag {
		t.Fatalf("rename persisted despite audit failure: %+v %v", got, err)
	}
}
