//go:build integration

package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Spec vendor-pks-list task 5.3, real dev DB (migration 044): UpdateReminder (Property 3),
// Daftar PKS paging vs count (Property 4), pks_count (Property 8), remaining_days (Property 9)
// and the create / replace / terminate regression with renewal_days. Skips without DATABASE_URL.

func newReminderSvc(pool *pgxpool.Pool) *VendorPksAdminService {
	return NewVendorPksAdminService(repository.NewVendorPksAdminRepository(pool), nil, pool)
}

func newListSvc(pool *pgxpool.Pool) *PksListService {
	return NewPksListService(repository.NewPksListRepository(pool), repository.NewPksTypeRepository(pool, pool),
		repository.NewVendorAdminRepository(pool, pool))
}

// pksJSONExcept returns the row as jsonb minus the given keys, so "nothing else changed" is one string compare.
func pksJSONExcept(t testing.TB, pool *pgxpool.Pool, id int64, keys ...string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT (to_jsonb(p) - $2::text[])::text FROM vendor_pks p WHERE id = $1`, id, keys).Scan(&s); err != nil {
		t.Fatalf("load pks %d json: %v", id, err)
	}
	return s
}

func reminderAudits(t testing.TB, pool *pgxpool.Pool, pksID int64) int {
	return countInt(t, pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_pks'
		AND action = 'vendor_pks_reminder_updated' AND entity_id = $1`, pksID)
}

func renewalOf(t testing.TB, pool *pgxpool.Pool, id int64) int32 {
	t.Helper()
	var n int32
	if err := pool.QueryRow(context.Background(), `SELECT renewal_days FROM vendor_pks WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("renewal_days of %d: %v", id, err)
	}
	return n
}

// pksListFixture is one FLM vendor with six rows: type A history (2 replaced + 1 active), type B terminated,
// type C expired, type D not yet effective.
type pksListFixture struct {
	h                   *testHelpers
	tag                 string
	vendorID            int64
	vendorCode          string
	a1, a2, a3, b, c, d int64
}

func newPksListFixture(t *testing.T) *pksListFixture {
	t.Helper()
	svc, tag, h := pksHarness(t)
	f := &pksListFixture{h: h, tag: tag, vendorCode: "ITPL-" + tag}
	f.vendorID = h.seedVendor(f.vendorCode, "Vendor daftar PKS "+tag)
	ta, _ := seedPksType(t, h, tag, "A")
	tb, _ := seedPksType(t, h, tag, "B")
	tc, _ := seedPksType(t, h, tag, "C")
	td, _ := seedPksType(t, h, tag, "D")

	create := func(typeID int64, eff, exp string, replaces *int64) int64 {
		t.Helper()
		if _, err := approvePksCreate(t, svc, h, f.vendorID, typeID, nextWod(tag), "PKS "+eff, eff, exp, replaces); err != nil {
			t.Fatalf("create pks: %v", err)
		}
		id := activePksID(rowsOfType(loadPksRows(t, h.pool, f.vendorID), typeID))
		if id == nil {
			t.Fatalf("no active pks for type %d after create", typeID)
		}
		return *id
	}
	f.a1 = create(ta, "2024-01-01", "2025-12-31", nil)
	f.a2 = create(ta, "2026-01-01", "2027-12-31", &f.a1)
	f.a3 = create(ta, "2028-01-01", "2035-12-31", &f.a2)
	// a3 is in the future relative to a2's term; make a3 effective now-ish instead so it is "aktif".
	if _, err := h.pool.Exec(h.ctx, `UPDATE vendor_pks SET effective_date = '2026-01-01' WHERE id = $1`, f.a3); err != nil {
		t.Fatalf("adjust a3: %v", err)
	}
	f.b = create(tb, "2025-01-01", "2030-12-31", nil)
	if _, err := approvePksReason(t, svc, h, "disable", f.vendorID, f.b, "Putus kontrak"); err != nil {
		t.Fatalf("terminate b: %v", err)
	}
	f.c = create(tc, "2020-01-01", "2021-01-01", nil)
	f.d = create(td, "2040-01-01", "2041-01-01", nil)
	return f
}

// Property 3: UpdateReminder changes renewal_days only (also on replaced and terminated rows), writes
// exactly one audit with before/after, and an unchanged value writes nothing.
func TestUpdateReminder_OnlyRenewalDaysAndOneAudit_Integration(t *testing.T) {
	f := newPksListFixture(t)
	pool, svc := f.h.pool, newReminderSvc(f.h.pool)
	f.h.t.Cleanup(func() {}) // keep h alive; cleanup is registered by the harness

	for _, tc := range []struct {
		name string
		id   int64
		role string
		days int32
	}{
		{"active row, ADMIN", f.a3, "ADMIN", 100},
		{"replaced row, ADMIN_PARAM", f.a1, "ADMIN_PARAM", 45},
		{"terminated row", f.b, "ADMIN", 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := pksJSONExcept(t, pool, tc.id, "renewal_days", "updated_at")
			old := renewalOf(t, pool, tc.id)
			row, err := svc.UpdateReminder(context.Background(), f.h.makerID, tc.role, tc.id, tc.days, "127.0.0.1")
			if err != nil {
				t.Fatalf("UpdateReminder: %v", err)
			}
			if row.RenewalDays != tc.days || renewalOf(t, pool, tc.id) != tc.days {
				t.Errorf("renewal_days = %d/%d, want %d", row.RenewalDays, renewalOf(t, pool, tc.id), tc.days)
			}
			if after := pksJSONExcept(t, pool, tc.id, "renewal_days", "updated_at"); after != before {
				t.Errorf("another column changed:\nbefore %s\nafter  %s", before, after)
			}
			if n := reminderAudits(t, pool, tc.id); n != 1 {
				t.Fatalf("audits = %d, want 1", n)
			}
			var beforeD, afterD int32
			if err := pool.QueryRow(context.Background(), `SELECT ("before"->>'renewal_days')::int, (after->>'renewal_days')::int
				FROM audit_logs WHERE entity_type = 'vendor_pks' AND action = 'vendor_pks_reminder_updated' AND entity_id = $1`, tc.id).Scan(&beforeD, &afterD); err != nil {
				t.Fatalf("audit payload: %v", err)
			}
			if beforeD != old || afterD != tc.days {
				t.Errorf("audit before/after = %d/%d, want %d/%d", beforeD, afterD, old, tc.days)
			}
			// Same value again: row returned, no second update, no second audit.
			if _, err := svc.UpdateReminder(context.Background(), f.h.makerID, tc.role, tc.id, tc.days, "127.0.0.1"); err != nil {
				t.Fatalf("no-op UpdateReminder: %v", err)
			}
			if n := reminderAudits(t, pool, tc.id); n != 1 {
				t.Errorf("audits after no-op = %d, want 1", n)
			}
		})
	}
}

func TestUpdateReminder_Rejections_Integration(t *testing.T) {
	f := newPksListFixture(t)
	pool, svc := f.h.pool, newReminderSvc(f.h.pool)
	ctx := context.Background()
	old := renewalOf(t, pool, f.a3) // term 2026-01-01..2035-12-31 is long; c has a 366-day term

	t.Run("role", func(t *testing.T) {
		if _, err := svc.UpdateReminder(ctx, f.h.makerID, "ATM-SPV", f.a3, 100, "ip"); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("err = %v, want ErrMasterDataForbidden", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		if _, err := svc.UpdateReminder(ctx, f.h.makerID, "ADMIN", 999999999, 100, "ip"); !errors.Is(err, ErrVendorPksNotFound) {
			t.Errorf("err = %v, want ErrVendorPksNotFound", err)
		}
	})
	for _, tc := range []struct {
		name string
		id   int64
		days int32
	}{
		{"zero", f.a3, 0}, {"below min", f.a3, 29}, {"above max", f.a3, 3667}, {"longer than term", f.c, 367},
	} {
		t.Run("422 "+tc.name, func(t *testing.T) {
			before := renewalOf(t, pool, tc.id)
			_, err := svc.UpdateReminder(ctx, f.h.makerID, "ADMIN", tc.id, tc.days, "ip")
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != "renewal_days" {
				t.Fatalf("err = %v, want ValidationError renewal_days", err)
			}
			if renewalOf(t, pool, tc.id) != before || reminderAudits(t, pool, tc.id) != 0 {
				t.Error("row or audit changed on a rejected request")
			}
		})
	}
	if renewalOf(t, pool, f.a3) != old {
		t.Error("a3 changed")
	}
}

// Audit failure rolls the update back: an actor that is not a user violates audit_logs_actor_fk.
func TestUpdateReminder_AuditFailureRollsBack_Integration(t *testing.T) {
	f := newPksListFixture(t)
	pool, svc := f.h.pool, newReminderSvc(f.h.pool)
	before := renewalOf(t, pool, f.a3)

	_, err := svc.UpdateReminder(context.Background(), -1, "ADMIN", f.a3, before+10, "ip")
	if err == nil {
		t.Fatal("want an error when the audit write fails")
	}
	if renewalOf(t, pool, f.a3) != before {
		t.Errorf("renewal_days = %d after failed audit, want rollback to %d", renewalOf(t, pool, f.a3), before)
	}
	if n := reminderAudits(t, pool, f.a3); n != 0 {
		t.Errorf("audits = %d, want 0", n)
	}
}

// Properties 4 and 9: paging through every status filter returns each row once, the pages add up to
// Count, and remaining_days = expiry − today (WIB).
func TestPksList_PagingMatchesCountAndRemainingDays_Integration(t *testing.T) {
	f := newPksListFixture(t)
	list := newListSvc(f.h.pool)
	ctx := context.Background()
	today := reports.TodayWIB(time.Now())

	wantIDs := map[string][]int64{
		"terkini":       {f.a3, f.b, f.c, f.d},
		"semua":         {f.a1, f.a2, f.a3, f.b, f.c, f.d},
		"diganti":       {f.a1, f.a2},
		"terminated":    {f.b},
		"expired":       {f.c},
		"belum_berlaku": {f.d},
		"aktif":         {f.a3},
	}
	statuses := make([]string, 0, len(wantIDs))
	for k := range wantIDs {
		statuses = append(statuses, k)
	}
	sort.Strings(statuses)

	rapid.Check(t, func(rt *rapid.T) {
		status := rapid.SampledFrom(statuses).Draw(rt, "status")
		size := rapid.IntRange(1, 4).Draw(rt, "size")
		var got []int64
		var total int64
		for page := 1; page <= 10; page++ {
			res, err := list.List(ctx, "ADMIN", PksListParams{Status: status, Vendor: f.vendorCode, Page: page, PageSize: size})
			if err != nil {
				rt.Fatalf("List: %v", err)
			}
			total = res.Total
			for _, it := range res.Items {
				if it.ID == nil {
					rt.Fatalf("unexpected pending create row %+v", it)
				}
				got = append(got, *it.ID)
				exp, _ := time.Parse(vendorPksDateLayout, it.ExpiryDate)
				if want := int(exp.Sub(today).Hours() / 24); it.RemainingDays != want {
					rt.Fatalf("pks %d remaining_days = %d, want %d", *it.ID, it.RemainingDays, want)
				}
			}
			if len(res.Items) < size {
				break
			}
		}
		want := wantIDs[status]
		if total != int64(len(want)) || len(got) != len(want) {
			rt.Fatalf("status %s size %d: total=%d got=%d want=%d", status, size, total, len(got), len(want))
		}
		seen := map[int64]bool{}
		for _, id := range got {
			if seen[id] {
				rt.Fatalf("status %s: id %d on two pages", status, id)
			}
			seen[id] = true
		}
		for _, id := range want {
			if !seen[id] {
				rt.Fatalf("status %s: id %d missing from pages", status, id)
			}
		}
	})
}

// Property 8: pks_count = the vendor's current (not replaced) PKS rows, terminated ones included.
func TestVendorList_PksCount_Integration(t *testing.T) {
	f := newPksListFixture(t)
	empty := f.h.seedVendor("ITPL0-"+f.tag, "Vendor tanpa PKS "+f.tag)
	_ = empty
	svc := NewVendorAdminService(repository.NewVendorAdminRepository(f.h.pool, f.h.pool), nil)

	for code, want := range map[string]int32{f.vendorCode: 4, "ITPL0-" + f.tag: 0} {
		q := code
		rows, err := svc.List(context.Background(), "ATM-SPV", db.ListVendorsAdminParams{Q: &q, Status: "all", PageLimit: 10})
		if err != nil {
			t.Fatalf("List %s: %v", code, err)
		}
		if len(rows) != 1 || rows[0].PksCount != want {
			t.Errorf("%s: rows=%d pks_count=%v, want 1 row with %d", code, len(rows), rows, want)
		}
	}
}

// Regression: create stores renewal_days, replace keeps the old row's value, terminate leaves it alone,
// WOD stays unique and there is one active row per (vendor, type).
func TestVendorPks_CreateReplaceTerminateKeepRenewalDays_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	vendorID := h.seedVendor("ITPR-"+tag, "Vendor regresi PKS "+tag)
	typeID, _ := seedPksType(t, h, tag, "R")

	wod1 := nextWod(tag)
	if _, err := approvePksCreate(t, svc, h, vendorID, typeID, wod1, "PKS 1", "2026-01-01", "2026-06-30", nil); err != nil {
		t.Fatal(err)
	}
	first := *activePksID(rowsOfType(loadPksRows(t, h.pool, vendorID), typeID))
	if got := renewalOf(t, h.pool, first); got != 180 { // default 270 capped at the 180-day term
		t.Errorf("first renewal_days = %d, want 180", got)
	}
	if _, err := approvePksCreate(t, svc, h, vendorID, typeID, wod1, "PKS dup", "2027-01-01", "2028-01-01", &first); err == nil {
		t.Error("duplicate WOD was accepted")
	}
	if _, err := approvePksCreate(t, svc, h, vendorID, typeID, nextWod(tag), "PKS 2", "2026-07-01", "2028-07-01", &first); err != nil {
		t.Fatal(err)
	}
	rows := rowsOfType(loadPksRows(t, h.pool, vendorID), typeID)
	active := 0
	for _, r := range rows {
		if r.ReplacedAt == nil {
			active++
		}
	}
	if len(rows) != 2 || active != 1 {
		t.Fatalf("rows/active = %d/%d, want 2/1", len(rows), active)
	}
	second := *activePksID(rows)
	if renewalOf(t, h.pool, first) != 180 || renewalOf(t, h.pool, second) != 270 {
		t.Errorf("renewal_days first/second = %d/%d, want 180/270", renewalOf(t, h.pool, first), renewalOf(t, h.pool, second))
	}
	if _, err := approvePksReason(t, svc, h, "disable", vendorID, second, "Putus kontrak"); err != nil {
		t.Fatal(err)
	}
	if got := renewalOf(t, h.pool, second); got != 270 {
		t.Errorf("renewal_days after terminate = %d, want 270", got)
	}
}
