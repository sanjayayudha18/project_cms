//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Lifecycle of cash count areas + region links through the real apply-on-approve path
// (ScDD 5.1 task 7.3, AC-5.1-08, AC-5.1-13). Codes and region codes are unique per run
// (other sessions share the dev DB); everything created is removed in t.Cleanup.
func TestCashCountAreaApplier_Lifecycle_Integration(t *testing.T) {
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

	var makerID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE username = 'Yudha'`).Scan(&makerID); err != nil {
		t.Fatalf("loading seeded admin user: %v", err)
	}

	// 9 digits of the nanosecond clock: area code "CCA<9>" (12 chars), regions "R<9>"/"S<9>" (10 chars).
	ns := strconv.FormatInt(time.Now().UnixNano(), 10)
	tag := ns[len(ns)-9:]
	areaCode, areaCode2 := "CCA"+tag, "CCB"+tag
	regA, regB, regC := "R"+tag, "S"+tag, "T"+tag

	h := &testHelpers{t: t, ctx: ctx, pool: pool, makerID: makerID}
	var areaIDs []int64
	t.Cleanup(func() {
		if len(areaIDs) > 0 {
			must := func(q string) {
				if _, err := pool.Exec(ctx, q, areaIDs); err != nil {
					t.Logf("cleanup: %v", err)
				}
			}
			must(`DELETE FROM cash_count_pics WHERE area_id = ANY($1)`)
			must(`DELETE FROM cash_count_area_regions WHERE area_id = ANY($1)`)
			must(`DELETE FROM audit_logs WHERE entity_type = 'cash_count_area' AND entity_id = ANY($1)`)
			must(`DELETE FROM cash_count_areas WHERE id = ANY($1)`)
		}
		h.cleanup() // change requests, vendor branches, vendors (registered last => runs first-in-LIFO safe)
	})

	repo := repository.NewMasterDataChangeRepository(pool)
	svc := NewMasterDataApprovalService(nil, repo, pool, ApplierRegistry{
		"cash_count_area": CashCountAreaApplier{},
	}, audit.NewWriter(pool))

	// Active vendor branches carrying the region codes (L648).
	vendorID := h.seedVendor("ITCC-"+tag, "Vendor CC Area "+tag)
	for i, rc := range []string{regA, regB, regC} {
		b := h.seedVendorBranch(vendorID, "ITCCB"+strconv.Itoa(i)+"-"+tag, "Branch "+tag)
		if _, err := pool.Exec(ctx, `UPDATE vendor_branches SET region_code = $1 WHERE id = $2`, rc, b); err != nil {
			t.Fatalf("set region_code: %v", err)
		}
	}

	q := dbtx(svc)
	regionsOf := func(areaID int64) []string {
		t.Helper()
		rows, err := q.Query(ctx, `SELECT region_code FROM cash_count_area_regions WHERE area_id = $1 ORDER BY region_code`, areaID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return out
	}
	areaByCode := func(code string) (id int64, active bool, found bool) {
		t.Helper()
		err := q.QueryRow(ctx, `SELECT id, is_active FROM cash_count_areas WHERE code = $1`, code).Scan(&id, &active)
		return id, active, err == nil
	}

	// --- create: nothing before approve, area + links after (AC-08) ---
	createID := h.insertPending("cash_count_area", "create", nil,
		CashCountAreaPayload{Code: areaCode, Name: "Area " + tag, RegionCodes: []string{regA, regB}})
	if _, _, found := areaByCode(areaCode); found {
		t.Fatal("area exists before approve")
	}
	if err := approveChange(t, svc, createID); err != nil {
		t.Fatalf("create: %v", err)
	}
	areaID, active, found := areaByCode(areaCode)
	if !found || !active {
		t.Fatalf("after create: found=%v active=%v", found, active)
	}
	areaIDs = append(areaIDs, areaID)
	if got := regionsOf(areaID); !reflect.DeepEqual(got, []string{regA, regB}) {
		t.Fatalf("regions after create = %v, want [%s %s]", got, regA, regB)
	}

	// --- AC-13: audit entry for the change, same tx as the apply ---
	var auditN, afterRegions int
	if err := q.QueryRow(ctx, `SELECT count(*), COALESCE(max(jsonb_array_length(after->'region_codes')), 0) FROM audit_logs
		WHERE entity_type = 'cash_count_area' AND action = 'master_data_applied' AND entity_id = $1`, areaID).Scan(&auditN, &afterRegions); err != nil || auditN != 1 || afterRegions != 2 {
		t.Errorf("apply audit for create: rows=%d after.region_codes=%d (err %v), want 1 / 2", auditN, afterRegions, err)
	}

	// --- a region already owned by another active area: second area refused at apply, nothing written ---
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "create", nil,
		CashCountAreaPayload{Code: areaCode2, Name: "Area 2 " + tag, RegionCodes: []string{regB, regC}})); err == nil {
		t.Error("region owned by another area: want apply error")
	}
	if _, _, found := areaByCode(areaCode2); found {
		t.Error("failed apply left an area behind (not atomic)")
	}
	if got := regionsOf(areaID); !reflect.DeepEqual(got, []string{regA, regB}) {
		t.Errorf("first area's regions changed by failed apply: %v", got)
	}

	// --- region used by no active vendor_branches: guarded insert matches no row ---
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "create", nil,
		CashCountAreaPayload{Code: areaCode2, Name: "Area 2 " + tag, RegionCodes: []string{"Q" + tag}})); err == nil {
		t.Error("unknown region: want apply error")
	}
	if _, _, found := areaByCode(areaCode2); found {
		t.Error("failed apply left an area behind (not atomic)")
	}

	// --- duplicate area code → conflict ---
	err = approveChange(t, svc, h.insertPending("cash_count_area", "create", nil,
		CashCountAreaPayload{Code: areaCode, Name: "dup", RegionCodes: []string{regC}}))
	if err == nil {
		t.Error("duplicate area code: want apply error")
	}

	// --- update: rename + rewrite links (regB dropped, regC added) ---
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "update", &areaID,
		CashCountAreaUpdatePayload{Name: "Renamed " + tag, RegionCodes: []string{regA, regC}})); err != nil {
		t.Fatalf("update: %v", err)
	}
	var name string
	if err := q.QueryRow(ctx, `SELECT name FROM cash_count_areas WHERE id = $1`, areaID).Scan(&name); err != nil || name != "Renamed "+tag {
		t.Errorf("name after update = %q (err %v)", name, err)
	}
	if got := regionsOf(areaID); !reflect.DeepEqual(got, []string{regA, regC}) {
		t.Errorf("regions after update = %v, want [%s %s]", got, regA, regC)
	}

	// --- service pre-checks against the real repo (submit side, AC-08) ---
	fake := &fakeVendorBranchSubmitter{}
	areaSvc := NewCashCountAreaAdminService(repository.NewCashCountAreaAdminRepository(pool, pool), fake)
	var verr *ValidationError
	if _, err := areaSvc.Create(ctx, makerID, CashCountAreaPayload{Code: areaCode2, Name: "x", RegionCodes: []string{regA}}, "127.0.0.1"); !errors.As(err, &verr) {
		t.Errorf("region owned by active area: err = %v, want ValidationError", err)
	}
	if _, err := areaSvc.Create(ctx, makerID, CashCountAreaPayload{Code: areaCode2, Name: "x", RegionCodes: []string{"Q" + tag}}, "127.0.0.1"); !errors.As(err, &verr) {
		t.Errorf("region without active branch: err = %v, want ValidationError", err)
	}
	if _, err := areaSvc.Create(ctx, makerID, CashCountAreaPayload{Code: areaCode, Name: "x", RegionCodes: []string{regB}}, "127.0.0.1"); !errors.Is(err, ErrCashCountAreaConflict) {
		t.Errorf("duplicate code: err = %v, want ErrCashCountAreaConflict", err)
	}
	if fake.submitCalled {
		t.Error("a refused request was submitted")
	}

	// --- disable refused while an active executor PIC uses the area ---
	var picID int64
	if err := pool.QueryRow(ctx, `INSERT INTO cash_count_pics (pic_role, user_id, area_id) VALUES ('EXECUTOR', $1, $2) RETURNING id`, makerID, areaID).Scan(&picID); err != nil {
		t.Fatalf("seed executor pic: %v", err)
	}
	if _, err := areaSvc.Disable(ctx, makerID, areaID, "127.0.0.1"); !errors.Is(err, ErrCashCountAreaHasExecutors) {
		t.Errorf("disable with executor (submit): err = %v, want ErrCashCountAreaHasExecutors", err)
	}
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "disable", &areaID, nil)); err == nil {
		t.Error("disable with executor (apply guard): want error")
	}
	if _, active, _ := areaByCode(areaCode); !active {
		t.Error("area disabled despite active executor")
	}
	if _, err := pool.Exec(ctx, `UPDATE cash_count_pics SET is_active = false, deleted_at = now() WHERE id = $1`, picID); err != nil {
		t.Fatal(err)
	}

	// --- disable: area inactive, region links freed; another area can then take them ---
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "disable", &areaID, nil)); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, active, _ := areaByCode(areaCode); active {
		t.Error("area still active after disable")
	}
	if got := regionsOf(areaID); len(got) != 0 {
		t.Errorf("regions after disable = %v, want none", got)
	}
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "create", nil,
		CashCountAreaPayload{Code: areaCode2, Name: "Area 2 " + tag, RegionCodes: []string{regA}})); err != nil {
		t.Fatalf("region freed by disable, second area: %v", err)
	}
	if id2, _, ok := areaByCode(areaCode2); ok {
		areaIDs = append(areaIDs, id2)
	}

	// --- enable: back active, without regions ---
	if err := approveChange(t, svc, h.insertPending("cash_count_area", "enable", &areaID, nil)); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, active, _ := areaByCode(areaCode); !active {
		t.Error("area not active after enable")
	}
	if got := regionsOf(areaID); len(got) != 0 {
		t.Errorf("regions after enable = %v, want none", got)
	}

	// ensure the list read path (replica) sees the final state
	rows, err := repository.NewCashCountAreaAdminRepository(pool, pool).List(ctx, "active")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var seen bool
	for _, r := range rows {
		if r.Code == areaCode {
			seen = true
		}
	}
	if !seen {
		t.Error("enabled area missing from active list")
	}
}
