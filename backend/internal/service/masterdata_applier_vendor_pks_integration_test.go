//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Spec vendor-pks task 7.5, per type (vendor-pks-type task 4.4): VendorPksApplier +
// apply-on-approve + VendorPksAdminService submit against a real DB. Properties:
// 1 (one active per vendor+type under concurrent approves), 2 (a change on one type
// leaves the other type's rows identical), 3 (WOD unique: case, trim, history,
// pending), 5 (inactive type: 422 at submit, stale at apply), 8 (pending per type).
// Skips without DATABASE_URL (harness). Dev must have migration 043.

// pksHarness is harness() with the vendor_pks applier registered. Cleanup (LIFO):
// PKS rows of the seeded vendors, then the test PKS types, then harness cleanup.
func pksHarness(t *testing.T) (*MasterDataApprovalService, string, *testHelpers) {
	t.Helper()
	svc, tag, h := harness(t)
	svc.appliers["vendor_pks"] = VendorPksApplier{}
	t.Cleanup(func() {
		if _, err := h.pool.Exec(h.ctx, `DELETE FROM vendor_pks_types WHERE code LIKE $1`, pksTypeTestCode(tag)+"%"); err != nil {
			t.Logf("cleanup: delete vendor_pks_types: %v", err)
		}
	})
	t.Cleanup(func() { deleteVendorPksOf(h.ctx, t, h.pool, h.vendorIDs) })
	return svc, tag, h
}

// seedPksType inserts an active test type (code = pksTypeTestCode(tag)+suffix).
func seedPksType(t testing.TB, h *testHelpers, tag, suffix string) (id int64, code string) {
	t.Helper()
	code = pksTypeTestCode(tag) + suffix
	if err := h.pool.QueryRow(h.ctx, `INSERT INTO vendor_pks_types (code, name) VALUES ($1, $2) RETURNING id`,
		code, "Tipe uji "+suffix).Scan(&id); err != nil {
		t.Fatalf("seed pks type %s: %v", code, err)
	}
	return id, code
}

var pksWodSeq atomic.Int64

// nextWod returns a WOD no other test row holds.
func nextWod(tag string) string { return fmt.Sprintf("WOD-%s-%d", tag, pksWodSeq.Add(1)) }

func deleteVendorPksOf(ctx context.Context, t *testing.T, pool *pgxpool.Pool, vendorIDs []int64) {
	if len(vendorIDs) == 0 {
		return
	}
	if _, err := pool.Exec(ctx, `DELETE FROM audit_logs WHERE entity_type = 'vendor_pks'
		AND entity_id IN (SELECT id FROM vendor_pks WHERE vendor_id = ANY($1))`, vendorIDs); err != nil {
		t.Logf("cleanup: delete audit_logs (vendor_pks): %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM vendor_pks WHERE vendor_id = ANY($1)`, vendorIDs); err != nil {
		t.Logf("cleanup: delete vendor_pks: %v", err)
	}
}

// pksRow is a vendor_pks row without updated_at (which the trigger moves).
type pksRow struct {
	ID           int64
	TypeID       int64
	Wod          string
	Name         string
	Eff, Exp     string
	ReplacedAt   *time.Time
	ReplacedBy   *int64
	TerminatedAt *time.Time
	TerminatedBy *int64
	Reason       *string
	CreatedBy    int64
}

func loadPksRows(t testing.TB, pool *pgxpool.Pool, vendorID int64) []pksRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id, pks_type_id, wod_number, pks_name, to_char(effective_date, 'YYYY-MM-DD'),
		to_char(expiry_date, 'YYYY-MM-DD'), replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
		termination_reason, created_by FROM vendor_pks WHERE vendor_id = $1 ORDER BY id`, vendorID)
	if err != nil {
		t.Fatalf("load vendor_pks: %v", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (pksRow, error) {
		var p pksRow
		err := r.Scan(&p.ID, &p.TypeID, &p.Wod, &p.Name, &p.Eff, &p.Exp, &p.ReplacedAt, &p.ReplacedBy, &p.TerminatedAt, &p.TerminatedBy, &p.Reason, &p.CreatedBy)
		return p, err
	})
	if err != nil {
		t.Fatalf("scan vendor_pks: %v", err)
	}
	return out
}

// rowsOfType keeps the rows of one type (order preserved).
func rowsOfType(rows []pksRow, typeID int64) []pksRow {
	out := []pksRow{}
	for _, r := range rows {
		if r.TypeID == typeID {
			out = append(out, r)
		}
	}
	return out
}

func pksSnapshot(t testing.TB, pool *pgxpool.Pool, id int64) db.GetVendorPksByIDRow {
	t.Helper()
	row, err := db.New(pool).GetVendorPksByID(context.Background(), id)
	if err != nil {
		t.Fatalf("load vendor_pks %d: %v", id, err)
	}
	return row
}

func changeStatus(t testing.TB, pool *pgxpool.Pool, id int64) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM master_data_change_requests WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("load change %d status: %v", id, err)
	}
	return s
}

func countInt(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

func pksAppliedAudits(t testing.TB, pool *pgxpool.Pool, pksID int64) int {
	return countInt(t, pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_pks'
		AND action = 'master_data_applied' AND entity_id = $1`, pksID)
}

func countAllPksAudits(t testing.TB, pool *pgxpool.Pool, vendorID int64) int {
	return countInt(t, pool, `SELECT count(*) FROM audit_logs WHERE entity_type = 'vendor_pks' AND action = 'master_data_applied'
		AND entity_id IN (SELECT id FROM vendor_pks WHERE vendor_id = $1)`, vendorID)
}

func vendorJSON(t testing.TB, pool *pgxpool.Pool, vendorID int64) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT row_to_json(v)::text FROM vendors v WHERE id = $1`, vendorID).Scan(&s); err != nil {
		t.Fatalf("load vendor %d: %v", vendorID, err)
	}
	return s
}

func approvePksCreate(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, vendorID, typeID int64, wod, name, eff, exp string, replaces *int64) (int64, error) {
	t.Helper()
	id := h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
		VendorID: vendorID, PksTypeID: typeID, WodNumber: wod, PksName: name, EffectiveDate: eff, ExpiryDate: exp, ReplacesPksID: replaces,
	})
	return id, approveChange(t, svc, id)
}

// approvePksReason stages terminate (disable) / batalkan terminate (enable) with
// the row's current state as Before, the way VendorPksAdminService does.
func approvePksReason(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, op string, vendorID, pksID int64, reason string) (int64, error) {
	t.Helper()
	snap := pksSnapshot(t, h.pool, pksID)
	id := h.insertPendingWithBefore("vendor_pks", op, &pksID,
		VendorPksReasonPayload{VendorID: vendorID, PksTypeID: snap.PksTypeID, Reason: reason}, snap)
	return id, approveChange(t, svc, id)
}

func activePksID(rows []pksRow) *int64 {
	for _, r := range rows {
		if r.ReplacedAt == nil {
			id := r.ID
			return &id
		}
	}
	return nil
}

// Property 5 (v1) per type + Property 2: any order of approved creates and
// terminates over two types leaves exactly one active PKS per type, earlier ones
// in that type's history linked to their successor, and every change leaves the
// other type's rows identical.
func TestVendorPksApplier_RandomCreates_OneActivePerType_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	typeIDs := make([]int64, 2)
	typeIDs[0], _ = seedPksType(t, h, tag, "A")
	typeIDs[1], _ = seedPksType(t, h, tag, "B")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seq := 0

	rapid.Check(t, func(rt *rapid.T) {
		seq++
		vendorID := h.seedVendor(fmt.Sprintf("ITPK-%s-%d", tag, seq), "Vendor PKS "+tag)
		type want struct {
			name, wod, eff, exp string
			terminated          bool
		}
		wants := make([][]want, 2)
		active := make([]*int64, 2)

		n := rapid.IntRange(2, 6).Draw(rt, "creates")
		for i := 0; i < n; i++ {
			k := rapid.IntRange(0, 1).Draw(rt, "type")
			otherBefore := rowsOfType(loadPksRows(t, h.pool, vendorID), typeIDs[1-k])

			eff := base.AddDate(0, 0, rapid.IntRange(-3000, 3000).Draw(rt, "eff_offset"))
			exp := eff.AddDate(0, 0, rapid.IntRange(0, 1500).Draw(rt, "validity_days"))
			w := want{
				name: fmt.Sprintf("PKS %d/%d", i, rapid.IntRange(0, 9999).Draw(rt, "name_no")), wod: nextWod(tag),
				eff: eff.Format(vendorPksDateLayout), exp: exp.Format(vendorPksDateLayout),
			}
			if _, err := approvePksCreate(t, svc, h, vendorID, typeIDs[k], w.wod, w.name, w.eff, w.exp, active[k]); err != nil {
				rt.Fatalf("create %d: %v", i, err)
			}
			active[k] = activePksID(rowsOfType(loadPksRows(t, h.pool, vendorID), typeIDs[k]))
			if rapid.Bool().Draw(rt, "terminate") {
				if _, err := approvePksReason(t, svc, h, "disable", vendorID, *active[k], "Putus kontrak"); err != nil {
					rt.Fatalf("terminate %d: %v", i, err)
				}
				w.terminated = true
			}
			wants[k] = append(wants[k], w)

			if got := rowsOfType(loadPksRows(t, h.pool, vendorID), typeIDs[1-k]); !reflect.DeepEqual(got, otherBefore) {
				rt.Fatalf("step %d on type %d changed the other type:\nbefore %+v\nafter  %+v", i, k, otherBefore, got)
			}
		}

		for k := range wants {
			rows := rowsOfType(loadPksRows(t, h.pool, vendorID), typeIDs[k])
			if len(rows) != len(wants[k]) {
				rt.Fatalf("type %d rows = %d, want %d", k, len(rows), len(wants[k]))
			}
			for i, r := range rows {
				w := wants[k][i]
				if r.Name != w.name || r.Wod != w.wod || r.Eff != w.eff || r.Exp != w.exp {
					rt.Fatalf("type %d row %d = %q %q %s..%s, want %q %q %s..%s", k, i, r.Name, r.Wod, r.Eff, r.Exp, w.name, w.wod, w.eff, w.exp)
				}
				if (r.TerminatedAt != nil) != w.terminated || (r.Reason != nil) != w.terminated {
					rt.Fatalf("type %d row %d terminated=%v reason=%v, want terminated=%v", k, i, r.TerminatedAt, r.Reason, w.terminated)
				}
				last := i == len(rows)-1
				if last != (r.ReplacedAt == nil) {
					rt.Fatalf("type %d row %d replaced_at=%v, want active only for the last row", k, i, r.ReplacedAt)
				}
				if !last && (r.ReplacedBy == nil || *r.ReplacedBy != rows[i+1].ID) {
					rt.Fatalf("type %d row %d replaced_by=%v, want %d", k, i, r.ReplacedBy, rows[i+1].ID)
				}
				if last && r.ReplacedBy != nil {
					rt.Fatalf("type %d active row replaced_by=%v, want nil", k, *r.ReplacedBy)
				}
			}
		}
	})
}

// Property 1 under a race: two creates of the same (vendor, type) approved at the
// same time (both saw no active PKS) -> one applied, one stale, one active PKS.
// Creates of different types do not conflict: both apply. The first change is
// approved-but-not-applied when the next is submitted (the pending index allows
// one pending per (vendor, type)), then the applies run in parallel via RetryApply.
func TestVendorPksApplier_ParallelApprove_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	ctx := adminCtx(h.makerID)
	typeA, _ := seedPksType(t, h, tag, "A")
	typeB, _ := seedPksType(t, h, tag, "B")

	stage := func(vendorID int64, typeIDs ...int64) []int64 {
		ids := make([]int64, len(typeIDs))
		for i, ty := range typeIDs {
			ids[i] = h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
				VendorID: vendorID, PksTypeID: ty, WodNumber: nextWod(tag), PksName: fmt.Sprintf("PKS race %d", i),
				EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01",
			})
			if _, err := h.pool.Exec(ctx, `UPDATE master_data_change_requests SET status = 'approved' WHERE id = $1`, ids[i]); err != nil {
				t.Fatalf("mark approved: %v", err)
			}
		}
		return ids
	}
	race := func(ids []int64) []error {
		errs := make([]error, len(ids))
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = svc.RetryApply(ctx, id, h.makerID, "127.0.0.1")
			}()
		}
		close(start)
		wg.Wait()
		return errs
	}

	for round := 0; round < 5; round++ {
		vendorID := h.seedVendor(fmt.Sprintf("ITPKR-%s-%d", tag, round), "Vendor PKS race "+tag)
		ids := stage(vendorID, typeA, typeA)
		errs := race(ids)

		applied, stale := 0, 0
		for i, id := range ids {
			switch st := changeStatus(t, h.pool, id); {
			case st == "applied" && errs[i] == nil:
				applied++
			case st == "stale" && errors.Is(errs[i], ErrMasterDataChangeStale):
				stale++
			default:
				t.Errorf("round %d change %d: status %q err %v", round, id, st, errs[i])
			}
		}
		if applied != 1 || stale != 1 {
			t.Errorf("round %d: applied=%d stale=%d, want 1/1", round, applied, stale)
		}
		if rows := loadPksRows(t, h.pool, vendorID); len(rows) != 1 || rows[0].ReplacedAt != nil {
			t.Errorf("round %d: rows = %+v, want exactly one active PKS", round, rows)
		}

		other := h.seedVendor(fmt.Sprintf("ITPKS-%s-%d", tag, round), "Vendor PKS two types "+tag)
		ids = stage(other, typeA, typeB)
		for i, err := range race(ids) {
			if err != nil || changeStatus(t, h.pool, ids[i]) != "applied" {
				t.Errorf("round %d two types change %d: status %q err %v, want applied", round, ids[i], changeStatus(t, h.pool, ids[i]), err)
			}
		}
		rows := loadPksRows(t, h.pool, other)
		if len(rows) != 2 || rows[0].ReplacedAt != nil || rows[1].ReplacedAt != nil {
			t.Errorf("round %d: rows = %+v, want one active PKS per type", round, rows)
		}
	}
}

// A change that no longer fits the DB is retired as stale and writes nothing (no
// row change, no master_data_applied audit): stale replaced_pks_id, a type
// disabled before approve (Property 5), a disabled vendor, a terminate/unterminate
// whose row changed meanwhile. A WOD taken meanwhile (Property 3) is a plain apply
// error (409), not stale.
func TestVendorPksApplier_StaleChanges_NotApplied_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	ctx := context.Background()
	typeA, _ := seedPksType(t, h, tag, "A")
	typeB, _ := seedPksType(t, h, tag, "B")

	assertNotApplied := func(t *testing.T, vendorID int64, before []pksRow, auditsBefore int) {
		t.Helper()
		if after := loadPksRows(t, h.pool, vendorID); !reflect.DeepEqual(after, before) {
			t.Errorf("vendor_pks changed:\nbefore %+v\nafter  %+v", before, after)
		}
		if n := countAllPksAudits(t, h.pool, vendorID); n != auditsBefore {
			t.Errorf("master_data_applied audits = %d, want %d", n, auditsBefore)
		}
	}
	assertStale := func(t *testing.T, vendorID, changeID int64, err error, before []pksRow, auditsBefore int) {
		t.Helper()
		if !errors.Is(err, ErrMasterDataChangeStale) {
			t.Fatalf("approve = %v, want ErrMasterDataChangeStale", err)
		}
		if st := changeStatus(t, h.pool, changeID); st != "stale" {
			t.Errorf("status = %q, want stale", st)
		}
		assertNotApplied(t, vendorID, before, auditsBefore)
	}

	t.Run("replaces_pks_id stale (per type)", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS1-"+tag, "Vendor stale replace "+tag)
		if _, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS 1", "2025-01-01", "2026-01-01", nil); err != nil {
			t.Fatal(err)
		}
		first := *activePksID(rowsOfType(loadPksRows(t, h.pool, vendorID), typeA))

		// The maker saw no PKS of this type, but one is active now.
		before, audits := loadPksRows(t, h.pool, vendorID), countAllPksAudits(t, h.pool, vendorID)
		id, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS 2", "2026-01-01", "2027-01-01", nil)
		assertStale(t, vendorID, id, err, before, audits)

		// An active PKS of another type is not a conflict: a first PKS of type B applies.
		if _, err := approvePksCreate(t, svc, h, vendorID, typeB, nextWod(tag), "PKS B", "2026-01-01", "2027-01-01", nil); err != nil {
			t.Fatalf("first PKS of another type: %v", err)
		}

		// The maker saw PKS 1, but PKS 2 replaced it in between.
		if _, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS 2", "2026-01-01", "2027-01-01", &first); err != nil {
			t.Fatal(err)
		}
		before, audits = loadPksRows(t, h.pool, vendorID), countAllPksAudits(t, h.pool, vendorID)
		id, err = approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS 3", "2027-01-01", "2028-01-01", &first)
		assertStale(t, vendorID, id, err, before, audits)
	})

	t.Run("type disabled before approve is stale (Property 5)", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS4-"+tag, "Vendor stale type "+tag)
		disabled, _ := seedPksType(t, h, tag, "D")
		id := h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
			VendorID: vendorID, PksTypeID: disabled, WodNumber: nextWod(tag), PksName: "PKS", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01",
		})
		if _, err := h.pool.Exec(ctx, `UPDATE vendor_pks_types SET is_active = false WHERE id = $1`, disabled); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), []pksRow{}, 0)
	})

	t.Run("vendor disabled before approve", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS2-"+tag, "Vendor stale disabled "+tag)
		id := h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
			VendorID: vendorID, PksTypeID: typeA, WodNumber: nextWod(tag), PksName: "PKS", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01",
		})
		if _, err := h.pool.Exec(ctx, `UPDATE vendors SET is_active = false, deleted_at = now() WHERE id = $1`, vendorID); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), []pksRow{}, 0)
	})

	t.Run("terminate row changed", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS3-"+tag, "Vendor stale terminate "+tag)
		if _, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS", "2026-01-01", "2027-01-01", nil); err != nil {
			t.Fatal(err)
		}
		pksID := *activePksID(loadPksRows(t, h.pool, vendorID))
		audits := countAllPksAudits(t, h.pool, vendorID)

		// terminate staged, then the row is terminated by someone else first.
		id := h.insertPendingWithBefore("vendor_pks", "disable", &pksID,
			VendorPksReasonPayload{VendorID: vendorID, PksTypeID: typeA, Reason: "Alasan A"}, pksSnapshot(t, h.pool, pksID))
		if _, err := h.pool.Exec(ctx, `UPDATE vendor_pks SET terminated_at = now(), terminated_by = $2, termination_reason = 'Alasan lain' WHERE id = $1`, pksID, h.makerID); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), loadPksRows(t, h.pool, vendorID), audits)

		// batalkan terminate staged, then the row is un-terminated by someone else first.
		id = h.insertPendingWithBefore("vendor_pks", "enable", &pksID,
			VendorPksReasonPayload{VendorID: vendorID, PksTypeID: typeA, Reason: "Alasan B"}, pksSnapshot(t, h.pool, pksID))
		if _, err := h.pool.Exec(ctx, `UPDATE vendor_pks SET terminated_at = NULL, terminated_by = NULL, termination_reason = NULL WHERE id = $1`, pksID); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), loadPksRows(t, h.pool, vendorID), audits)
	})

	t.Run("wod taken at apply is a conflict, whatever case/trim/history (Property 3)", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS5-"+tag, "Vendor wod "+tag)
		wod := nextWod(tag)
		if _, err := approvePksCreate(t, svc, h, vendorID, typeA, wod, "PKS 1", "2026-01-01", "2027-01-01", nil); err != nil {
			t.Fatal(err)
		}
		first := *activePksID(loadPksRows(t, h.pool, vendorID))
		// PKS 1 goes to history; its WOD stays taken.
		if _, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS 2", "2027-01-01", "2028-01-01", &first); err != nil {
			t.Fatal(err)
		}

		for name, taken := range map[string]string{
			"same":       wod,
			"lower case": strings.ToLower(wod),
			"padded":     "  " + strings.ToUpper(wod) + " ",
		} {
			before, audits := loadPksRows(t, h.pool, vendorID), countAllPksAudits(t, h.pool, vendorID)
			current := *activePksID(rowsOfType(before, typeA))
			id, err := approvePksCreate(t, svc, h, vendorID, typeA, taken, "PKS 3", "2028-01-01", "2029-01-01", &current)
			if !errors.Is(err, ErrVendorPksWodTaken) {
				t.Errorf("%s: approve = %v, want ErrVendorPksWodTaken", name, err)
			}
			if st := changeStatus(t, h.pool, id); st == "applied" {
				t.Errorf("%s: change applied despite a taken WOD", name)
			}
			assertNotApplied(t, vendorID, before, audits)
		}
	})
}

// Property 8 (terminate/unterminate side) + reject: terminate then batalkan
// terminate puts the row back as it was (updated_at aside), never touches vendors
// or the other type's PKS, one master_data_applied audit per step; a new PKS over a
// terminated one keeps the old terminate data in history (Req 3.10); a rejected
// change writes nothing.
func TestVendorPksApplier_TerminateUnterminate_Reject_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	typeA, _ := seedPksType(t, h, tag, "A")
	typeB, _ := seedPksType(t, h, tag, "B")
	vendorID := h.seedVendor("ITPKT-"+tag, "Vendor PKS terminate "+tag)

	if _, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS awal", "2026-01-01", "2027-12-31", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := approvePksCreate(t, svc, h, vendorID, typeB, nextWod(tag), "PKS B", "2026-01-01", "2027-12-31", nil); err != nil {
		t.Fatal(err)
	}
	pksID := *activePksID(rowsOfType(loadPksRows(t, h.pool, vendorID), typeA))
	original := pksSnapshot(t, h.pool, pksID)
	vendorBefore := vendorJSON(t, h.pool, vendorID)
	typeBBefore := rowsOfType(loadPksRows(t, h.pool, vendorID), typeB)

	if _, err := approvePksReason(t, svc, h, "disable", vendorID, pksID, "Vendor wanprestasi"); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	terminated := pksSnapshot(t, h.pool, pksID)
	if !terminated.TerminatedAt.Valid || terminated.TerminatedBy == nil || *terminated.TerminatedBy != h.makerID ||
		terminated.TerminationReason == nil || *terminated.TerminationReason != "Vendor wanprestasi" {
		t.Fatalf("after terminate = %+v, want terminated by maker with reason", terminated)
	}
	if got := vendorJSON(t, h.pool, vendorID); got != vendorBefore {
		t.Errorf("terminate changed vendors:\nbefore %s\nafter  %s", vendorBefore, got)
	}

	if _, err := approvePksReason(t, svc, h, "enable", vendorID, pksID, "Salah input"); err != nil {
		t.Fatalf("batalkan terminate: %v", err)
	}
	restored := pksSnapshot(t, h.pool, pksID)
	original.UpdatedAt, restored.UpdatedAt = pgtype.Timestamptz{}, pgtype.Timestamptz{}
	if !reflect.DeepEqual(restored, original) {
		t.Errorf("after batalkan terminate:\n got  %+v\n want %+v", restored, original)
	}
	if got := vendorJSON(t, h.pool, vendorID); got != vendorBefore {
		t.Errorf("batalkan terminate changed vendors:\nbefore %s\nafter  %s", vendorBefore, got)
	}
	if n := pksAppliedAudits(t, h.pool, pksID); n != 3 {
		t.Errorf("master_data_applied audits = %d, want 3 (create, terminate, batalkan)", n)
	}

	// New PKS on top of a Terminated one: old row goes to history with its terminate data.
	if _, err := approvePksReason(t, svc, h, "disable", vendorID, pksID, "Putus kontrak"); err != nil {
		t.Fatalf("terminate again: %v", err)
	}
	if _, err := approvePksCreate(t, svc, h, vendorID, typeA, nextWod(tag), "PKS baru", "2028-01-01", "2029-12-31", &pksID); err != nil {
		t.Fatalf("create over terminated: %v", err)
	}
	rows := rowsOfType(loadPksRows(t, h.pool, vendorID), typeA)
	if len(rows) != 2 || rows[0].ReplacedAt == nil || rows[0].ReplacedBy == nil || *rows[0].ReplacedBy != rows[1].ID ||
		rows[0].TerminatedAt == nil || rows[0].Reason == nil || *rows[0].Reason != "Putus kontrak" ||
		rows[1].ReplacedAt != nil || rows[1].TerminatedAt != nil {
		t.Fatalf("rows after create over terminated = %+v", rows)
	}
	if got := rowsOfType(loadPksRows(t, h.pool, vendorID), typeB); !reflect.DeepEqual(got, typeBBefore) {
		t.Errorf("type A operations changed type B:\nbefore %+v\nafter  %+v", typeBBefore, got)
	}

	// Reject: no PKS write.
	newID := rows[1].ID
	before := loadPksRows(t, h.pool, vendorID)
	changeID := h.insertPendingWithBefore("vendor_pks", "disable", &newID,
		VendorPksReasonPayload{VendorID: vendorID, PksTypeID: typeA, Reason: "Ditolak"}, pksSnapshot(t, h.pool, newID))
	svc.orchestrator = &canApproveOnce{result: db.ApprovalRequest{DocumentType: masterDataDocumentType, DocumentID: changeID, Status: "rejected"}}
	if _, err := svc.Reject(context.Background(), 999, 1, "127.0.0.1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if st := changeStatus(t, h.pool, changeID); st != "rejected" {
		t.Errorf("status = %q, want rejected", st)
	}
	if after := loadPksRows(t, h.pool, vendorID); !reflect.DeepEqual(after, before) {
		t.Errorf("reject changed vendor_pks:\nbefore %+v\nafter  %+v", before, after)
	}
}

// failingApplier lets the applyOne regression drive both error branches.
type failingApplier struct{ err error }

func (f failingApplier) Apply(context.Context, pgx.Tx, db.MasterDataChangeRequest) (int64, any, error) {
	return 0, nil, f.err
}

func (failingApplier) CurrentState(context.Context, pgx.Tx, int64) (any, error) { return nil, nil }

// applyOne regression (task 7.3): a plain apply error leaves the change approved
// with the error recorded (retryable); only ErrMasterDataChangeStale retires it.
func TestApplyOne_ErrorBranches_Integration(t *testing.T) {
	svc, _, h := pksHarness(t)

	svc.appliers["itest_pks_fail"] = failingApplier{err: errors.New("boom")}
	id := h.insertPending("itest_pks_fail", "create", nil, map[string]any{})
	err := approveChange(t, svc, id)
	if err == nil || errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("approve = %v, want a non-stale apply error", err)
	}
	var status string
	var msg *string
	if err := h.pool.QueryRow(h.ctx, `SELECT status, error FROM master_data_change_requests WHERE id = $1`, id).Scan(&status, &msg); err != nil {
		t.Fatal(err)
	}
	if status != "approved" || msg == nil || !strings.Contains(*msg, "boom") {
		t.Errorf("change = status %q error %v, want approved + boom", status, msg)
	}

	svc.appliers["itest_pks_stale"] = failingApplier{err: fmt.Errorf("guard: %w", ErrMasterDataChangeStale)}
	id = h.insertPending("itest_pks_stale", "create", nil, map[string]any{})
	if err := approveChange(t, svc, id); !errors.Is(err, ErrMasterDataChangeStale) {
		t.Fatalf("approve = %v, want ErrMasterDataChangeStale", err)
	}
	if st := changeStatus(t, h.pool, id); st != "stale" {
		t.Errorf("status = %q, want stale", st)
	}
}

// pksApprovalFixture is the production wiring for PKS submit + approve: real
// approval.Orchestrator, MasterDataChangeService, VendorPksAdminService, applier
// registry. It owns its users (maker -> approver1 -> approver2, all ADMIN), one
// FLM vendor and two active PKS types; cleanup deletes by owner since nothing here
// rolls back (the types go with pksHarness cleanup).
type pksApprovalFixture struct {
	t                           *testing.T
	ctx                         context.Context
	pool                        *pgxpool.Pool
	tag                         string
	maker, approver1, approver2 int64
	vendorID                    int64
	typeAID, typeBID            int64
	typeA, typeB                string // codes
	pks                         *VendorPksAdminService
	approvals                   *MasterDataApprovalService
}

func newPksApprovalFixture(t *testing.T) *pksApprovalFixture {
	t.Helper()
	svc, tag, h := pksHarness(t) // skips without DATABASE_URL; supplies pool + vendor/type cleanup
	pool := h.pool
	f := &pksApprovalFixture{t: t, ctx: h.ctx, pool: pool, tag: tag}

	user := func(suffix string, supervisor *int64, level *int32) int64 {
		name := "itpks" + tag + "-" + suffix
		var id int64
		if err := pool.QueryRow(f.ctx, `INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, supervisor_id, approval_level)
			SELECT id, $1, $1, $1 || '@pks.example.com', true, 'ldap', $2::bigint, $3::int FROM roles WHERE role = 'ADMIN' RETURNING id`,
			name, supervisor, level).Scan(&id); err != nil {
			t.Fatalf("seed user %s: %v", suffix, err)
		}
		return id
	}
	one, two := int32(1), int32(2)
	f.approver2 = user("a2", nil, &two)
	f.approver1 = user("a1", &f.approver2, &one)
	f.maker = user("m", &f.approver1, nil)
	t.Cleanup(f.cleanup) // runs before pksHarness cleanup (LIFO): PKS, change requests, approvals, users

	f.vendorID = h.seedVendor("ITPKF-"+tag, "Vendor PKS fixture "+tag)
	f.typeAID, f.typeA = seedPksType(t, h, tag, "A")
	f.typeBID, f.typeB = seedPksType(t, h, tag, "B")

	repo := approval.NewRepository(pool)
	auditWriter := audit.NewWriter(pool)
	orchestrator := approval.NewOrchestrator(repo, repo, repo, auditWriter, nil)
	changeRepo := repository.NewMasterDataChangeRepository(pool)
	changes := NewMasterDataChangeService(changeRepo, orchestrator, auditWriter)
	f.pks = NewVendorPksAdminService(repository.NewVendorPksAdminRepository(pool), changes, 9)
	f.approvals = NewMasterDataApprovalService(orchestrator, changeRepo, pool, svc.appliers, auditWriter)
	return f
}

func (f *pksApprovalFixture) cleanup() {
	ctx := context.Background()
	users := []int64{f.maker, f.approver1, f.approver2}
	for _, s := range []string{
		`DELETE FROM notification_emails WHERE notification_id IN (SELECT id FROM notifications WHERE recipient_user_id = ANY($1))`,
		`DELETE FROM notifications WHERE recipient_user_id = ANY($1)`,
		`DELETE FROM audit_logs WHERE actor_id = ANY($1)`,
		`DELETE FROM vendor_pks WHERE created_by = ANY($1)`,
		`DELETE FROM master_data_change_requests WHERE maker_id = ANY($1)`,
		`DELETE FROM approval_steps WHERE request_id IN (SELECT id FROM approval_requests WHERE maker_id = ANY($1))`,
		`DELETE FROM approval_requests WHERE maker_id = ANY($1)`,
		`DELETE FROM users WHERE id = ANY($1)`,
	} {
		if _, err := f.pool.Exec(ctx, s, users); err != nil {
			f.t.Errorf("cleanup %q: %v", s, err)
		}
	}
}

func (f *pksApprovalFixture) req(typeCode, wod string) SubmitVendorPksRequest {
	return SubmitVendorPksRequest{PksType: typeCode, WodNumber: wod, PksName: "PKS fixture", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01"}
}

// Maker != checker: the real orchestrator refuses the maker approving their own
// PKS change; the change stays pending and nothing is written.
func TestVendorPks_MakerCannotApproveOwnChange_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)

	change, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeA, nextWod(f.tag)), "127.0.0.1")
	if err != nil {
		t.Fatalf("SubmitNew: %v", err)
	}
	if change.ApprovalRequestID == nil {
		t.Fatal("change has no approval request")
	}
	if _, err := f.approvals.Approve(ctx, *change.ApprovalRequestID, f.maker, "127.0.0.1"); !errors.Is(err, approval.ErrNotAuthorized) {
		t.Fatalf("maker Approve = %v, want approval.ErrNotAuthorized", err)
	}
	if st := changeStatus(t, f.pool, change.ID); st != "pending" {
		t.Errorf("status = %q, want pending", st)
	}
	if rows := loadPksRows(t, f.pool, f.vendorID); len(rows) != 0 {
		t.Errorf("vendor_pks rows = %+v, want none", rows)
	}
}

// RBAC at the service layer (Submit re-checks ADMIN/ADMIN_PARAM): any other role is
// refused before anything is staged.
func TestVendorPks_SubmitRBAC_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)

	for _, role := range []string{"ATM-USER", "VENDOR", "APPACCESS"} {
		ctx := ctxWithRole(f.maker, role)
		if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeA, nextWod(f.tag)), "127.0.0.1"); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("role %s: SubmitNew = %v, want ErrMasterDataForbidden", role, err)
		}
	}
	if n := countInt(t, f.pool, `SELECT count(*) FROM master_data_change_requests WHERE entity_type = 'vendor_pks' AND maker_id = $1`, f.maker); n != 0 {
		t.Errorf("staged changes = %d, want 0", n)
	}
	if _, err := f.pks.SubmitNew(ctxWithRole(f.maker, "ADMIN_PARAM"), f.maker, f.vendorID, f.req(f.typeA, nextWod(f.tag)), "127.0.0.1"); err != nil {
		t.Errorf("ADMIN_PARAM SubmitNew: %v", err)
	}
}

// Property 3 at submit: a WOD already on a PKS (any case, padded, history) or on a
// pending create is refused with ErrVendorPksWodTaken; nothing is staged.
func TestVendorPks_SubmitWodUnique_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)
	countPending := func() int {
		return countInt(t, f.pool, `SELECT count(*) FROM master_data_change_requests WHERE entity_type = 'vendor_pks' AND maker_id = $1 AND status = 'pending'`, f.maker)
	}

	// A PKS row of type A, with the replaced one in history.
	insert := func(wod string, replaced bool) {
		sql := `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
			VALUES ($1, $2, $3, 'PKS seed', '2026-01-01', '2027-01-01', $4)`
		if replaced {
			sql = `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by, replaced_at)
				VALUES ($1, $2, $3, 'PKS seed', '2025-01-01', '2025-12-31', $4, now())`
		}
		if _, err := f.pool.Exec(f.ctx, sql, f.vendorID, f.typeAID, wod, f.maker); err != nil {
			t.Fatalf("seed vendor_pks %s: %v", wod, err)
		}
	}
	activeWod, historyWod := "Wod-Aktif-"+f.tag, "Wod-Riwayat-"+f.tag
	insert(historyWod, true)
	insert(activeWod, false)

	for name, wod := range map[string]string{
		"active, lower case": strings.ToLower(activeWod),
		"active, padded":     "  " + activeWod + "  ",
		"history, upper":     strings.ToUpper(historyWod),
	} {
		if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeB, wod), "127.0.0.1"); !errors.Is(err, ErrVendorPksWodTaken) {
			t.Errorf("%s: SubmitNew = %v, want ErrVendorPksWodTaken", name, err)
		}
	}
	if n := countPending(); n != 0 {
		t.Fatalf("pending changes = %d, want 0", n)
	}

	// A pending create holds its WOD too (here on type B), whatever the case.
	pendingWod := nextWod(f.tag)
	if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeB, pendingWod), "127.0.0.1"); err != nil {
		t.Fatalf("SubmitNew pending: %v", err)
	}
	if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeA, strings.ToLower(pendingWod)), "127.0.0.1"); !errors.Is(err, ErrVendorPksWodTaken) {
		t.Errorf("pending wod: SubmitNew = %v, want ErrVendorPksWodTaken", err)
	}
}

// Property 5 at submit: an unknown or inactive type is a validation error on
// pks_type (422); nothing is staged.
func TestVendorPks_SubmitInactiveType_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)

	if _, err := f.pool.Exec(f.ctx, `UPDATE vendor_pks_types SET is_active = false WHERE id = $1`, f.typeBID); err != nil {
		t.Fatal(err)
	}
	for name, code := range map[string]string{"inactive": f.typeB, "unknown": "ZZNOPE9"} {
		_, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(code, nextWod(f.tag)), "127.0.0.1")
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "pks_type" {
			t.Errorf("%s: err = %v, want ValidationError on pks_type", name, err)
		}
	}
	if n := countInt(t, f.pool, `SELECT count(*) FROM master_data_change_requests WHERE entity_type = 'vendor_pks' AND maker_id = $1`, f.maker); n != 0 {
		t.Errorf("staged changes = %d, want 0", n)
	}
}

// Property 8: one pending change per (vendor, type): a second submit of the same
// type gets ErrMasterDataChangePending (also for a terminate of that type's PKS),
// the other type is unaffected, and rejecting frees the slot.
func TestVendorPks_PendingPerType_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)

	first, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeA, nextWod(f.tag)), "127.0.0.1")
	if err != nil {
		t.Fatalf("SubmitNew A: %v", err)
	}
	if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeA, nextWod(f.tag)), "127.0.0.1"); !errors.Is(err, ErrMasterDataChangePending) {
		t.Fatalf("second SubmitNew A = %v, want ErrMasterDataChangePending", err)
	}
	if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeB, nextWod(f.tag)), "127.0.0.1"); err != nil {
		t.Fatalf("SubmitNew B must not be blocked by a pending A: %v", err)
	}

	// A PKS of type A can't be terminated while type A has a pending change.
	var pksID int64
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
		VALUES ($1, $2, $3, 'PKS aktif', '2026-01-01', '2027-01-01', $4) RETURNING id`, f.vendorID, f.typeAID, nextWod(f.tag), f.maker).Scan(&pksID); err != nil {
		t.Fatalf("seed active pks: %v", err)
	}
	if _, err := f.pks.SubmitTerminate(ctx, f.maker, f.vendorID, pksID, "Putus kontrak", "127.0.0.1"); !errors.Is(err, ErrMasterDataChangePending) {
		t.Fatalf("SubmitTerminate A = %v, want ErrMasterDataChangePending", err)
	}

	// Reject frees the slot.
	if _, err := f.pool.Exec(f.ctx, `UPDATE master_data_change_requests SET status = 'rejected' WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pks.SubmitTerminate(ctx, f.maker, f.vendorID, pksID, "Putus kontrak", "127.0.0.1"); err != nil {
		t.Fatalf("SubmitTerminate after the pending A was rejected: %v", err)
	}
}
