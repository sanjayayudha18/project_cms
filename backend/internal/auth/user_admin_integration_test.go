//go:build integration

package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// TestUserAdminService_VendorBranch_Integration proves vendor-party-scope
// Req 10.1 (f) on a real DB: the service rejects a branch that is not an
// active branch of the account's vendor (400 vendor_branch_id), and the
// composite FK users_vendor_branch_owner_fk rejects it underneath the service
// (Req 2.5). Everything runs in one transaction rolled back on cleanup.
func TestUserAdminService_VendorBranch_Integration(t *testing.T) {
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
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	tag := time.Now().Format("150405.000000000")
	seed := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := tx.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
		return id
	}
	vendorA := seed(`INSERT INTO vendors (code, name, is_active) VALUES ($1, $1, true) RETURNING id`, "TVPA_"+tag)
	vendorB := seed(`INSERT INTO vendors (code, name, is_active) VALUES ($1, $1, true) RETURNING id`, "TVPB_"+tag)
	branchA := seed(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, 'Medan') RETURNING id`, vendorA, "A1-"+tag)
	branchAOff := seed(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active) VALUES ($1, $2, 'Tutup', false) RETURNING id`, vendorA, "A2-"+tag)
	branchB := seed(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, 'Bandung') RETURNING id`, vendorB, "B1-"+tag)

	repo := repository.NewUserAdminRepository(tx)
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, repo, auditW)

	create := func(username string, branchID int64) (db.CreateUserAdminRow, error) {
		return svc.Create(ctx, 1, "APPACCESS", CreateUserRequest{
			Username: username + tag, FullName: "Vendor " + username, Email: username + tag + "@example.com",
			Role: "VENDOR-ATM-USER", AuthSource: "local", TemporaryPassword: "Passw0rd1",
			VendorID: &vendorA, VendorBranchID: &branchID,
		}, "")
	}

	t.Run("service rejects other vendor's branch", func(t *testing.T) {
		_, err := create("other.", branchB)
		assertFieldError(t, err, "vendor_branch_id", true)
	})
	t.Run("service rejects inactive branch", func(t *testing.T) {
		_, err := create("inactive.", branchAOff)
		assertFieldError(t, err, "vendor_branch_id", true)
	})
	if len(auditW.entries) != 0 {
		t.Fatalf("rejected creates wrote %d audit entries", len(auditW.entries))
	}

	created, err := create("ok.", branchA)
	if err != nil {
		t.Fatalf("create with own branch: %v", err)
	}
	if created.VendorBranchName == nil || *created.VendorBranchName != "Medan" {
		t.Errorf("create vendor_branch_name = %v, want Medan", created.VendorBranchName)
	}
	got, err := svc.Get(ctx, created.ID)
	if err != nil || got == nil || got.VendorBranchID == nil || *got.VendorBranchID != branchA || got.Role != "VENDOR-ATM-USER" {
		t.Fatalf("Get after create = %+v, %v", got, err)
	}

	t.Run("service rejects vendor change that keeps the branch", func(t *testing.T) {
		_, err := svc.Update(ctx, 1, "ADMIN", created.ID, UpdateUserRequest{
			FullName: "Vendor ok", Email: got.Email, Role: "VENDOR-ATM-USER", VendorID: &vendorB,
		}, "")
		assertFieldError(t, err, "vendor_branch_id", true)
	})

	t.Run("FK rejects other vendor's branch below the service", func(t *testing.T) {
		sp, err := tx.Begin(ctx) // savepoint: the FK error must not abort the outer tx
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		defer func() { _ = sp.Rollback(ctx) }()
		_, err = sp.Exec(ctx, `UPDATE users SET vendor_id = $1 WHERE id = $2`, vendorB, created.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "users_vendor_branch_owner_fk" {
			t.Fatalf("err = %v, want FK users_vendor_branch_owner_fk violation", err)
		}
	})

	t.Run("update clears branch to vendor-wide", func(t *testing.T) {
		updated, err := svc.Update(ctx, 1, "APPACCESS", created.ID, UpdateUserRequest{
			FullName: "Vendor ok", Email: got.Email, Role: "VENDOR-VAULT-USER", VendorID: &vendorA,
			VendorBranchIDSet: true,
		}, "")
		if err != nil || updated.VendorBranchID != nil || updated.VendorBranchName != nil {
			t.Fatalf("update = %+v, %v", updated, err)
		}
		last := auditW.entries[len(auditW.entries)-1]
		before, _ := last.Before.(*db.GetUserAdminByIDRow)
		if last.Action != "user_updated" || before == nil || before.VendorBranchID == nil || *before.VendorBranchID != branchA {
			t.Errorf("audit = %s before=%+v, want user_updated with old branch", last.Action, before)
		}
	})

	t.Run("vendor-options lists active branches only", func(t *testing.T) {
		opts, err := svc.VendorOptions(ctx)
		if err != nil {
			t.Fatalf("VendorOptions: %v", err)
		}
		seen := map[int64]bool{}
		for _, b := range opts.Branches {
			seen[b.ID] = true
		}
		if !seen[branchA] || !seen[branchB] || seen[branchAOff] {
			t.Errorf("branches A=%v B=%v inactive=%v, want true/true/false", seen[branchA], seen[branchB], seen[branchAOff])
		}
		vendorSeen := false
		for _, v := range opts.Vendors {
			vendorSeen = vendorSeen || v.ID == vendorA
		}
		if !vendorSeen {
			t.Error("vendor A missing from options")
		}
	})
}
