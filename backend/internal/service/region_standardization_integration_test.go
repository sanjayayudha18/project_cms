//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// region-standardization tasks 3.3 / 4.4 on real Postgres (migration 042 applied).

func regionStdPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// regionStdTx: one rolled-back tx used as primary, replica and RegionAdminPool
// (same convention as regionHarness), plus the seeded admin user id.
func regionStdTx(t *testing.T) (pgx.Tx, *RegionAdminService, int64) {
	t.Helper()
	ctx := context.Background()
	tx, err := regionStdPool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	var admin int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&admin); err != nil {
		t.Fatalf("admin user: %v", err)
	}
	return tx, NewRegionAdminService(repository.NewRegionAdminRepository(tx, tx), tx), admin
}

func stdRow(t *testing.T, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := q.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("seed %q: %v", sql, err)
	}
	return id
}

func stdCode(prefix string) string {
	return prefix + strings.ToUpper(strconv.FormatInt(time.Now().UnixNano()%1_000_000, 36)) // ^[A-Z0-9]{2,10}$
}

// Property 5 (counts are active-only) and Property 6 (disable refused while a
// location or an active vendor branch still uses the region, no audit written).
func TestRegionStd_CountsAndDisableGuards(t *testing.T) {
	tx, svc, admin := regionStdTx(t)
	ctx := context.Background()
	audits := func(regionID int64) int64 {
		return stdRow(t, tx, `SELECT count(*) FROM audit_logs WHERE entity_type = 'region' AND action = 'region_deactivated' AND entity_id = $1`, regionID)
	}

	// Region A: one location with one active + one inactive ATM.
	regA, err := svc.Create(ctx, admin, "ADMIN", CreateRegionRequest{Code: stdCode("RSA"), Region: "IT std A"}, "")
	if err != nil {
		t.Fatalf("create region A: %v", err)
	}
	loc := stdRow(t, tx, `INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		VALUES ($1, 'ATM_SITE', 'IT std loc', 'Jl. Test 1', 'Jakarta', 'DKI Jakarta', 'ID') RETURNING id`, regA.ID)
	for i, active := range []bool{true, false} {
		stdRow(t, tx, `INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
			VALUES ($1, $2, 'ATM', 'B', 'M', '24_HOURS', 'OFFSITE', $3) RETURNING id`, regA.Code+"T"+strconv.Itoa(i), loc, active)
	}
	gotA, err := svc.Get(ctx, regA.ID)
	if err != nil || gotA.AtmCount != 1 || gotA.VendorBranchCount != 0 {
		t.Fatalf("region A counts = %+v %v, want atm 1, branches 0", gotA, err)
	}
	if _, err := svc.Disable(ctx, admin, "ADMIN", regA.ID, ""); !errors.Is(err, ErrRegionHasActiveLocations) || audits(regA.ID) != 0 {
		t.Fatalf("disable A = %v (audits %d), want ErrRegionHasActiveLocations and no audit", err, audits(regA.ID))
	}

	// Region B: one active + one inactive vendor branch, no location.
	regB, err := svc.Create(ctx, admin, "ADMIN", CreateRegionRequest{Code: stdCode("RSB"), Region: "IT std B"}, "")
	if err != nil {
		t.Fatalf("create region B: %v", err)
	}
	vendor := stdRow(t, tx, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITSTD"+regB.Code)
	active := stdRow(t, tx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code) VALUES ($1, $2, 'b1', $3) RETURNING id`, vendor, regB.Code+"B1", regB.Code)
	stdRow(t, tx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code, is_active, deleted_at) VALUES ($1, $2, 'b2', $3, false, now()) RETURNING id`, vendor, regB.Code+"B2", regB.Code)
	gotB, err := svc.Get(ctx, regB.ID)
	if err != nil || gotB.AtmCount != 0 || gotB.VendorBranchCount != 1 {
		t.Fatalf("region B counts = %+v %v, want atm 0, branches 1", gotB, err)
	}
	if _, err := svc.Disable(ctx, admin, "ADMIN", regB.ID, ""); !errors.Is(err, ErrRegionHasActiveVendorBranches) || audits(regB.ID) != 0 {
		t.Fatalf("disable B = %v (audits %d), want ErrRegionHasActiveVendorBranches and no audit", err, audits(regB.ID))
	}
	// Only active branches block: once the last one is disabled the region can go.
	if _, err := tx.Exec(ctx, `UPDATE vendor_branches SET is_active = false, deleted_at = now() WHERE id = $1`, active); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Disable(ctx, admin, "ADMIN", regB.ID, ""); err != nil || audits(regB.ID) != 1 {
		t.Fatalf("disable B after branches disabled = %v (audits %d), want ok + 1 audit", err, audits(regB.ID))
	}
}

func branchCreateChange(t *testing.T, vendorID int64, code string, region *string) db.MasterDataChangeRequest {
	t.Helper()
	raw, err := json.Marshal(VendorBranchPayload{VendorID: vendorID, BranchCode: code, BranchName: code, RegionCode: region, Category: "ATM"})
	if err != nil {
		t.Fatal(err)
	}
	return db.MasterDataChangeRequest{EntityType: "vendor_branch", Op: "create", Payload: raw}
}

// Req 3.4: a branch change approved after its region was disabled is not applied.
func TestRegionStd_ApplierRejectsInactiveRegion(t *testing.T) {
	tx, _, _ := regionStdTx(t)
	ctx := context.Background()
	code := stdCode("RSC")
	stdRow(t, tx, `INSERT INTO regions (code, region) VALUES ($1, 'IT std C') RETURNING id`, code)
	vendor := stdRow(t, tx, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITSTD"+code)

	if _, err := tx.Exec(ctx, `UPDATE regions SET is_active = false, deleted_at = now() WHERE code = $1`, code); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (VendorBranchApplier{}).Apply(ctx, tx, branchCreateChange(t, vendor, code+"X", &code)); !errors.Is(err, ErrVendorBranchRegionInactive) {
		t.Fatalf("apply on inactive region = %v, want ErrVendorBranchRegionInactive", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE regions SET is_active = true, deleted_at = NULL WHERE code = $1`, code); err != nil {
		t.Fatal(err)
	}
	id, _, err := (VendorBranchApplier{}).Apply(ctx, tx, branchCreateChange(t, vendor, code+"Y", &code))
	if err != nil {
		t.Fatalf("apply on active region: %v", err)
	}
	if got := stdRow(t, tx, `SELECT count(*) FROM vendor_branches WHERE id = $1 AND region_code = $2`, id, code); got != 1 {
		t.Fatalf("applied branch region_code mismatch")
	}
}

// Property 4: a region disable racing a branch apply on the same region never
// lets both succeed — the disable waits for the apply's FOR SHARE, then sees
// the new active branch and is refused. Uses committed rows (separate
// connections), cleaned up at the end; the region is swept by TestMain.
func TestRegionStd_DisableWaitsForBranchApply(t *testing.T) {
	pool := regionStdPool(t)
	ctx := context.Background()
	tag := uuid.NewString()[:8]
	code := itRegionCode("race-" + tag)
	seedTestRegion(t, pool, code, "IT std race")
	var regionID, admin int64
	if err := pool.QueryRow(ctx, `SELECT id FROM regions WHERE code = $1`, code).Scan(&regionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	vendor := stdRow(t, pool, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITRACE"+tag)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM vendor_branches WHERE vendor_id = $1`, vendor)
		_, _ = pool.Exec(context.Background(), `DELETE FROM vendors WHERE id = $1`, vendor)
	})

	applyTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = applyTx.Rollback(ctx) }()
	if _, _, err := (VendorBranchApplier{}).Apply(ctx, applyTx, branchCreateChange(t, vendor, "ITRACE"+tag+"B", &code)); err != nil {
		t.Fatalf("apply: %v", err)
	}

	svc := NewRegionAdminService(repository.NewRegionAdminRepository(pool, pool), pool)
	done := make(chan error, 1)
	go func() {
		_, err := svc.Disable(ctx, admin, "ADMIN", regionID, "")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("disable finished while the apply still held FOR SHARE: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := applyTx.Commit(ctx); err != nil {
		t.Fatalf("commit apply: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrRegionHasActiveVendorBranches) {
			t.Fatalf("disable after apply = %v, want ErrRegionHasActiveVendorBranches", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("disable still blocked after the apply committed")
	}
	if active := stdRow(t, pool, `SELECT count(*) FROM regions WHERE id = $1 AND is_active`, regionID); active != 1 {
		t.Fatal("region was disabled even though an active branch now uses it")
	}
}

// Property 8: every reader shows the branch's region name from regions.region.
func TestRegionStd_ReadersUseRegionName(t *testing.T) {
	tx, _, _ := regionStdTx(t)
	ctx := context.Background()
	code := stdCode("RSN")
	name := "IT std nama " + code
	stdRow(t, tx, `INSERT INTO regions (code, region) VALUES ($1, $2) RETURNING id`, code, name)
	vendor := stdRow(t, tx, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, "ITSTD"+code)
	branch := stdRow(t, tx, `INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region_code) VALUES ($1, $2, 'b', $3) RETURNING id`, vendor, code+"B", code)
	stdRow(t, tx, `INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code) VALUES ($1, $2, 'CASH', 'CASH', 'IDR') RETURNING id`, branch, code+"V")
	q := db.New(tx)

	branches, err := q.ListVendorBranchesAdmin(ctx, db.ListVendorBranchesAdminParams{VendorID: vendor, Status: "all", PageLimit: 10})
	if err != nil || len(branches) != 1 || branches[0].Region == nil || *branches[0].Region != name {
		t.Errorf("ListVendorBranchesAdmin region = %+v %v, want %q", branches, err, name)
	}
	vaults, err := q.ListVendorVaultsAdmin(ctx, db.ListVendorVaultsAdminParams{VendorID: vendor, Status: "all", PageLimit: 10})
	if err != nil || len(vaults) != 1 || vaults[0].Region == nil || *vaults[0].Region != name {
		t.Errorf("ListVendorVaultsAdmin region = %+v %v, want %q", vaults, err, name)
	}
	export, err := q.ExportVendorBranchesBatch(ctx, db.ExportVendorBranchesBatchParams{VendorID: &vendor, Status: "all", BatchSize: 10})
	if err != nil || len(export) != 1 || export[0].Region != name {
		t.Errorf("ExportVendorBranchesBatch region = %+v %v, want %q", export, err, name)
	}
	names, err := q.ListDistinctVendorBranchRegions(ctx)
	found := false
	for _, n := range names {
		found = found || (n != nil && *n == name)
	}
	if err != nil || !found {
		t.Errorf("ListDistinctVendorBranchRegions misses %q (err %v)", name, err)
	}
}
