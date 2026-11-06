//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
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

// Spec vendor-pks task 7.5: VendorPksApplier + apply-on-approve (Property 5, 7, 8)
// against a real DB. Skips without DATABASE_URL (harness).

// pksHarness is harness() with the vendor_pks applier registered and the PKS rows
// of the seeded vendors removed before harness cleanup deletes the vendors
// (t.Cleanup runs LIFO).
func pksHarness(t *testing.T) (*MasterDataApprovalService, string, *testHelpers) {
	t.Helper()
	svc, tag, h := harness(t)
	svc.appliers["vendor_pks"] = VendorPksApplier{}
	t.Cleanup(func() { deleteVendorPksOf(h.ctx, t, h.pool, h.vendorIDs) })
	return svc, tag, h
}

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
	rows, err := pool.Query(context.Background(), `SELECT id, pks_name, to_char(effective_date, 'YYYY-MM-DD'),
		to_char(expiry_date, 'YYYY-MM-DD'), replaced_at, replaced_by_pks_id, terminated_at, terminated_by,
		termination_reason, created_by FROM vendor_pks WHERE vendor_id = $1 ORDER BY id`, vendorID)
	if err != nil {
		t.Fatalf("load vendor_pks: %v", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (pksRow, error) {
		var p pksRow
		err := r.Scan(&p.ID, &p.Name, &p.Eff, &p.Exp, &p.ReplacedAt, &p.ReplacedBy, &p.TerminatedAt, &p.TerminatedBy, &p.Reason, &p.CreatedBy)
		return p, err
	})
	if err != nil {
		t.Fatalf("scan vendor_pks: %v", err)
	}
	return out
}

func pksSnapshot(t testing.TB, pool *pgxpool.Pool, id int64) db.VendorPk {
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

func approvePksCreate(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, vendorID int64, name, eff, exp string, replaces *int64) (int64, error) {
	t.Helper()
	id := h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
		VendorID: vendorID, PksName: name, EffectiveDate: eff, ExpiryDate: exp, ReplacesPksID: replaces,
	})
	return id, approveChange(t, svc, id)
}

// approvePksReason stages terminate (disable) / batalkan terminate (enable) with
// the row's current state as Before, the way VendorPksAdminService does.
func approvePksReason(t *testing.T, svc *MasterDataApprovalService, h *testHelpers, op string, vendorID, pksID int64, reason string) (int64, error) {
	t.Helper()
	id := h.insertPendingWithBefore("vendor_pks", op, &pksID, VendorPksReasonPayload{VendorID: vendorID, Reason: reason}, pksSnapshot(t, h.pool, pksID))
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

// Property 5: any order of approved creates (with terminates in between) leaves
// exactly one active PKS, every earlier one in history linked to its successor,
// name/dates exactly as submitted, and the terminate data of a replaced PKS kept.
func TestVendorPksApplier_RandomCreates_OneActiveFullHistory_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seq := 0

	rapid.Check(t, func(rt *rapid.T) {
		seq++
		vendorID := h.seedVendor(fmt.Sprintf("ITPK-%s-%d", tag, seq), "Vendor PKS "+tag)
		type want struct {
			name, eff, exp string
			terminated     bool
		}
		var wants []want
		var active *int64

		n := rapid.IntRange(1, 5).Draw(rt, "creates")
		for i := 0; i < n; i++ {
			eff := base.AddDate(0, 0, rapid.IntRange(-3000, 3000).Draw(rt, "eff_offset"))
			exp := eff.AddDate(0, 0, rapid.IntRange(0, 1500).Draw(rt, "validity_days"))
			w := want{
				name: fmt.Sprintf("PKS %d/%d", i, rapid.IntRange(0, 9999).Draw(rt, "name_no")),
				eff:  eff.Format(vendorPksDateLayout), exp: exp.Format(vendorPksDateLayout),
			}
			if _, err := approvePksCreate(t, svc, h, vendorID, w.name, w.eff, w.exp, active); err != nil {
				rt.Fatalf("create %d: %v", i, err)
			}
			active = activePksID(loadPksRows(t, h.pool, vendorID))
			if rapid.Bool().Draw(rt, "terminate") {
				if _, err := approvePksReason(t, svc, h, "disable", vendorID, *active, "Putus kontrak"); err != nil {
					rt.Fatalf("terminate %d: %v", i, err)
				}
				w.terminated = true
			}
			wants = append(wants, w)
		}

		rows := loadPksRows(t, h.pool, vendorID)
		if len(rows) != n {
			rt.Fatalf("rows = %d, want %d", len(rows), n)
		}
		for i, r := range rows {
			w := wants[i]
			if r.Name != w.name || r.Eff != w.eff || r.Exp != w.exp {
				rt.Fatalf("row %d = %q %s..%s, want %q %s..%s", i, r.Name, r.Eff, r.Exp, w.name, w.eff, w.exp)
			}
			if (r.TerminatedAt != nil) != w.terminated || (r.Reason != nil) != w.terminated {
				rt.Fatalf("row %d terminated=%v reason=%v, want terminated=%v", i, r.TerminatedAt, r.Reason, w.terminated)
			}
			last := i == n-1
			if last != (r.ReplacedAt == nil) {
				rt.Fatalf("row %d replaced_at=%v, want active only for the last row", i, r.ReplacedAt)
			}
			if !last && (r.ReplacedBy == nil || *r.ReplacedBy != rows[i+1].ID) {
				rt.Fatalf("row %d replaced_by=%v, want %d", i, r.ReplacedBy, rows[i+1].ID)
			}
			if last && r.ReplacedBy != nil {
				rt.Fatalf("active row replaced_by=%v, want nil", *r.ReplacedBy)
			}
		}
	})
}

// Property 5 under a race: two creates approved at the same time for the same
// vendor (both saw no active PKS) -> one applied, one stale, one active PKS.
// The first is approved-but-not-applied when the second is submitted (the only
// way two can be in flight: the pending index allows one pending per vendor),
// then both applies run in parallel through RetryApply.
func TestVendorPksApplier_ParallelApprove_OneAppliedOneStale_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	ctx := adminCtx(h.makerID)

	for round := 0; round < 5; round++ {
		vendorID := h.seedVendor(fmt.Sprintf("ITPKR-%s-%d", tag, round), "Vendor PKS race "+tag)
		ids := make([]int64, 2)
		for i := range ids {
			ids[i] = h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
				VendorID: vendorID, PksName: fmt.Sprintf("PKS race %d", i), EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01",
			})
			if _, err := h.pool.Exec(ctx, `UPDATE master_data_change_requests SET status = 'approved' WHERE id = $1`, ids[i]); err != nil {
				t.Fatalf("mark approved: %v", err)
			}
		}

		errs := make([]error, 2)
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
	}
}

// Property 7: a change that no longer fits the DB is retired as stale and writes
// nothing (no row change, no master_data_applied audit).
func TestVendorPksApplier_StaleChanges_NotApplied_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	ctx := context.Background()

	assertStale := func(t *testing.T, vendorID, changeID int64, err error, before []pksRow, auditsBefore int) {
		t.Helper()
		if !errors.Is(err, ErrMasterDataChangeStale) {
			t.Fatalf("approve = %v, want ErrMasterDataChangeStale", err)
		}
		if st := changeStatus(t, h.pool, changeID); st != "stale" {
			t.Errorf("status = %q, want stale", st)
		}
		if after := loadPksRows(t, h.pool, vendorID); !reflect.DeepEqual(after, before) {
			t.Errorf("vendor_pks changed:\nbefore %+v\nafter  %+v", before, after)
		}
		if n := countAllPksAudits(t, h.pool, vendorID); n != auditsBefore {
			t.Errorf("master_data_applied audits = %d, want %d", n, auditsBefore)
		}
	}

	t.Run("replaces_pks_id stale", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS1-"+tag, "Vendor stale replace "+tag)
		if _, err := approvePksCreate(t, svc, h, vendorID, "PKS 1", "2025-01-01", "2026-01-01", nil); err != nil {
			t.Fatal(err)
		}
		first := *activePksID(loadPksRows(t, h.pool, vendorID))

		// The maker saw no PKS, but one is active now.
		before, audits := loadPksRows(t, h.pool, vendorID), countAllPksAudits(t, h.pool, vendorID)
		id, err := approvePksCreate(t, svc, h, vendorID, "PKS 2", "2026-01-01", "2027-01-01", nil)
		assertStale(t, vendorID, id, err, before, audits)

		// The maker saw PKS 1, but PKS 2 replaced it in between.
		if _, err := approvePksCreate(t, svc, h, vendorID, "PKS 2", "2026-01-01", "2027-01-01", &first); err != nil {
			t.Fatal(err)
		}
		before, audits = loadPksRows(t, h.pool, vendorID), countAllPksAudits(t, h.pool, vendorID)
		id, err = approvePksCreate(t, svc, h, vendorID, "PKS 3", "2027-01-01", "2028-01-01", &first)
		assertStale(t, vendorID, id, err, before, audits)
	})

	t.Run("vendor disabled before approve", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS2-"+tag, "Vendor stale disabled "+tag)
		id := h.insertPending("vendor_pks", "create", nil, VendorPksCreatePayload{
			VendorID: vendorID, PksName: "PKS", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01",
		})
		if _, err := h.pool.Exec(ctx, `UPDATE vendors SET is_active = false, deleted_at = now() WHERE id = $1`, vendorID); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), []pksRow{}, 0)
	})

	t.Run("terminate row changed", func(t *testing.T) {
		vendorID := h.seedVendor("ITPKS3-"+tag, "Vendor stale terminate "+tag)
		if _, err := approvePksCreate(t, svc, h, vendorID, "PKS", "2026-01-01", "2027-01-01", nil); err != nil {
			t.Fatal(err)
		}
		pksID := *activePksID(loadPksRows(t, h.pool, vendorID))
		audits := countAllPksAudits(t, h.pool, vendorID)

		// terminate staged, then the row is terminated by someone else first.
		id := h.insertPendingWithBefore("vendor_pks", "disable", &pksID, VendorPksReasonPayload{VendorID: vendorID, Reason: "Alasan A"}, pksSnapshot(t, h.pool, pksID))
		if _, err := h.pool.Exec(ctx, `UPDATE vendor_pks SET terminated_at = now(), terminated_by = $2, termination_reason = 'Alasan lain' WHERE id = $1`, pksID, h.makerID); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), loadPksRows(t, h.pool, vendorID), audits)

		// batalkan terminate staged, then the row is un-terminated by someone else first.
		id = h.insertPendingWithBefore("vendor_pks", "enable", &pksID, VendorPksReasonPayload{VendorID: vendorID, Reason: "Alasan B"}, pksSnapshot(t, h.pool, pksID))
		if _, err := h.pool.Exec(ctx, `UPDATE vendor_pks SET terminated_at = NULL, terminated_by = NULL, termination_reason = NULL WHERE id = $1`, pksID); err != nil {
			t.Fatal(err)
		}
		assertStale(t, vendorID, id, approveChange(t, svc, id), loadPksRows(t, h.pool, vendorID), audits)
	})
}

// Property 8: terminate then batalkan terminate puts the row back as it was
// (updated_at aside), never touches vendors, one master_data_applied audit per
// step; a new PKS over a terminated one keeps the old terminate data in history
// (Req 3.10); a rejected change writes nothing.
func TestVendorPksApplier_TerminateUnterminate_Reject_Integration(t *testing.T) {
	svc, tag, h := pksHarness(t)
	vendorID := h.seedVendor("ITPKT-"+tag, "Vendor PKS terminate "+tag)

	if _, err := approvePksCreate(t, svc, h, vendorID, "PKS awal", "2026-01-01", "2027-12-31", nil); err != nil {
		t.Fatal(err)
	}
	pksID := *activePksID(loadPksRows(t, h.pool, vendorID))
	original := pksSnapshot(t, h.pool, pksID)
	vendorBefore := vendorJSON(t, h.pool, vendorID)

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
	if _, err := approvePksCreate(t, svc, h, vendorID, "PKS baru", "2028-01-01", "2029-12-31", &pksID); err != nil {
		t.Fatalf("create over terminated: %v", err)
	}
	rows := loadPksRows(t, h.pool, vendorID)
	if len(rows) != 2 || rows[0].ReplacedAt == nil || rows[0].ReplacedBy == nil || *rows[0].ReplacedBy != rows[1].ID ||
		rows[0].TerminatedAt == nil || rows[0].Reason == nil || *rows[0].Reason != "Putus kontrak" ||
		rows[1].ReplacedAt != nil || rows[1].TerminatedAt != nil {
		t.Fatalf("rows after create over terminated = %+v", rows)
	}

	// Reject: no PKS write.
	newID := rows[1].ID
	before := loadPksRows(t, h.pool, vendorID)
	changeID := h.insertPendingWithBefore("vendor_pks", "disable", &newID, VendorPksReasonPayload{VendorID: vendorID, Reason: "Ditolak"}, pksSnapshot(t, h.pool, newID))
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
// registry. It owns its users (maker -> approver1 -> approver2, all ADMIN) and one
// FLM vendor; cleanup deletes by owner since nothing here rolls back.
type pksApprovalFixture struct {
	t                           *testing.T
	ctx                         context.Context
	pool                        *pgxpool.Pool
	maker, approver1, approver2 int64
	vendorID                    int64
	pks                         *VendorPksAdminService
	approvals                   *MasterDataApprovalService
}

func newPksApprovalFixture(t *testing.T) *pksApprovalFixture {
	t.Helper()
	svc, tag, h := pksHarness(t) // skips without DATABASE_URL; supplies pool + vendor cleanup
	pool := h.pool
	f := &pksApprovalFixture{t: t, ctx: h.ctx, pool: pool}

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

// Maker != checker: the real orchestrator refuses the maker approving their own
// PKS change; the change stays pending and nothing is written.
func TestVendorPks_MakerCannotApproveOwnChange_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)

	change, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, SubmitVendorPksRequest{
		PksName: "PKS maker", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01",
	}, "127.0.0.1")
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
