//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
)

// report-export-migration 6.3 (Properties 3 and 8) through the real router, reusing the 6.2 env
// (router, token service, pool). Rows written here carry the marker T63_ (one ADMIN user and the
// jobs/audit rows it makes) and are deleted in t.Cleanup; a last step fails the test on any leftover.
// No worker runs: jobs stay queued unless a test changes them with plain SQL.

const (
	t63User = "T63_ADMIN"
	t63Base = "/api/v1/exports/reports/"
)

const t63Users = `(SELECT id FROM users WHERE username LIKE 'T63\_%')`

type t63Env struct {
	*t62Env
	user t62User
}

func newT63Env(t *testing.T) *t63Env {
	t.Helper()
	e := &t63Env{t62Env: newT62Env(t)}
	t.Cleanup(e.assertNoT63Leftovers) // runs after cleanup
	t.Cleanup(e.cleanupT63)           // runs first
	e.cleanupT63()                    // stale rows of a crashed run
	id := e.scanInt(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = 'ADMIN' RETURNING id`, t63User)
	e.user = t62User{id: id, name: t63User, token: e.tokenFor(id, t63User, "ADMIN")}
	return e
}

func (e *t63Env) cleanupT63() {
	for _, s := range []string{
		`DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN ` + t63Users + `)`,
		`DELETE FROM export_jobs WHERE requested_by IN ` + t63Users,
		`DELETE FROM audit_logs WHERE actor_id IN ` + t63Users,
		`DELETE FROM users WHERE username LIKE 'T63\_%'`,
	} {
		if _, err := e.pool.Exec(context.Background(), s); err != nil {
			e.t.Errorf("t63 cleanup: %v\n%s", err, s)
		}
	}
}

func (e *t63Env) assertNoT63Leftovers() {
	total := int64(0)
	for _, q := range []string{
		`SELECT count(*) FROM users WHERE username LIKE 'T63\_%'`,
		`SELECT count(*) FROM export_jobs WHERE requested_by IN ` + t63Users,
		`SELECT count(*) FROM audit_logs WHERE actor_id IN ` + t63Users,
	} {
		n := e.scanInt(q)
		if n != 0 {
			e.t.Errorf("t63 leftover rows: %d from %s", n, q)
		}
		total += n
	}
	e.t.Logf("t63 leftover rows after cleanup: %d", total)
}

func (e *t63Env) counts() (jobs, audits int64) {
	e.t.Helper()
	return e.scanInt(`SELECT count(*) FROM export_jobs WHERE requested_by = $1`, e.user.id),
		e.scanInt(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, e.user.id)
}

func (e *t63Env) post(pathQuery string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, t63Base+pathQuery, e.user.token)
}

// postJob posts and returns the status and job id; the body must carry a job.
func (e *t63Env) postJob(t *testing.T, pathQuery string) (int, int64) {
	t.Helper()
	rec := e.post(pathQuery)
	var b struct{ Data struct{ ID int64 } }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("%s body: %v: %s", pathQuery, err, rec.Body.String())
	}
	return rec.Code, b.Data.ID
}

// Property 3: bad parameters get the legacy field/message as a 400 validation_error, with no job and no audit.
func TestReportExportParamValidationIntegration(t *testing.T) {
	e := newT63Env(t)

	const (
		msgMonthReq   = "month wajib diisi (YYYY-MM)"
		msgMonthFmt   = "format month YYYY-MM"
		msgMonthAfter = "bulan tidak boleh setelah bulan berjalan"
		msgInvalid    = "tidak valid"
		msgUnknown    = "tidak dikenal"
		msgDateFmt    = "format tanggal YYYY-MM-DD"
		msgDatesReq   = "date_from dan date_to wajib diisi"
		msgToBefore   = "date_to tidak boleh sebelum date_from"
		msgRange      = "rentang maksimal 31 hari"
	)
	const (
		dd  = "dsr-late-daily"
		ds  = "dsr-late-summary"
		ram = "replenishment-amount-mismatch"
		rt  = "replenishment-trips"
	)
	rng := "date_from=2026-06-01&date_to=2026-06-30"

	cases := []struct {
		name, route, query, field, message string
	}{
		{"daily missing month", dd, "", "month", msgMonthReq},
		{"daily bad month", dd, "month=2026-13", "month", msgMonthFmt},
		{"daily garbage month", dd, "month=juni", "month", msgMonthFmt},
		{"daily future month", dd, "month=2999-01", "month", msgMonthAfter},
		{"daily vendor_id text", dd, "month=2026-06&vendor_id=abc", "vendor_id", msgInvalid},
		{"daily vendor_id zero", dd, "month=2026-06&vendor_id=0", "vendor_id", msgInvalid},
		{"daily vendor_id negative", dd, "month=2026-06&vendor_id=-3", "vendor_id", msgInvalid},
		{"daily vendor_branch_id text", dd, "month=2026-06&vendor_branch_id=x", "vendor_branch_id", msgInvalid},
		{"daily vendor_branch_id zero", dd, "month=2026-06&vendor_branch_id=0", "vendor_branch_id", msgInvalid},
		{"daily bad status", dd, "month=2026-06&status=bogus", "status", msgUnknown},
		{"summary missing month", ds, "", "month", msgMonthReq},
		{"summary bad month", ds, "month=06-2026", "month", msgMonthFmt},
		{"summary future month", ds, "month=2999-12", "month", msgMonthAfter},
		{"summary vendor_id text", ds, "month=2026-06&vendor_id=abc", "vendor_id", msgInvalid},
		{"summary vendor_branch_id negative", ds, "month=2026-06&vendor_branch_id=-1", "vendor_branch_id", msgInvalid},
		{"mismatch missing dates", ram, "", "date_from", msgDatesReq},
		{"mismatch only date_from", ram, "date_from=2026-06-01", "date_from", msgDatesReq},
		{"mismatch only date_to", ram, "date_to=2026-06-30", "date_from", msgDatesReq},
		{"mismatch bad date_from", ram, "date_from=01-06-2026&date_to=2026-06-30", "date_from", msgDateFmt},
		{"mismatch bad date_to", ram, "date_from=2026-06-01&date_to=2026-02-30", "date_to", msgDateFmt},
		{"mismatch to before from", ram, "date_from=2026-06-10&date_to=2026-06-09", "date_to", msgToBefore},
		{"mismatch 32 days", ram, "date_from=2026-06-01&date_to=2026-07-02", "date_to", msgRange},
		{"mismatch vendor_id text", ram, rng + "&vendor_id=abc", "vendor_id", msgInvalid},
		{"mismatch bad classification", ram, rng + "&classification=x", "classification", msgUnknown},
		{"trips missing dates", rt, "", "date_from", msgDatesReq},
		{"trips bad date_from", rt, "date_from=nope&date_to=2026-06-30", "date_from", msgDateFmt},
		{"trips to before from", rt, "date_from=2026-06-10&date_to=2026-06-01", "date_to", msgToBefore},
		{"trips 32 days", rt, "date_from=2026-01-01&date_to=2026-02-01", "date_to", msgRange},
		{"trips vendor_id zero", rt, rng + "&vendor_id=0", "vendor_id", msgInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := c.route
			if c.query != "" {
				path += "?" + c.query
			}
			rec := e.post(path)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			var b struct {
				Error struct {
					Code    string
					Details []struct{ Field, Message string }
				}
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
				t.Fatalf("body: %v: %s", err, rec.Body.String())
			}
			if b.Error.Code != "validation_error" || len(b.Error.Details) != 1 {
				t.Fatalf("error = %+v, want validation_error with one detail: %s", b.Error, rec.Body.String())
			}
			if d := b.Error.Details[0]; d.Field != c.field || d.Message != c.message {
				t.Errorf("detail = {%s %q}, want {%s %q}", d.Field, d.Message, c.field, c.message)
			}
			if jobs, audits := e.counts(); jobs != 0 || audits != 0 {
				t.Errorf("side effects after 400: jobs=%d audits=%d, want 0/0", jobs, audits)
			}
		})
	}

	// Parameters that do not apply to the kind are ignored, never rejected.
	ignored := []struct {
		name, pathQuery, absentKey string
	}{
		{"summary ignores status", ds + "?month=2026-06&status=bogus", "status"},
		{"summary ignores classification", ds + "?month=2026-06&classification=x", "classification"},
		{"daily ignores classification", dd + "?month=2026-06&classification=x", "classification"},
		{"mismatch ignores status", ram + "?" + rng + "&status=bogus", "status"},
		{"trips ignores classification", rt + "?" + rng + "&classification=x", "classification"},
		{"trips ignores status", rt + "?" + rng + "&status=bogus", "status"},
		{"daily ignores date range", dd + "?month=2026-06&date_from=junk&date_to=junk", "date_from"},
		{"trips ignores month", rt + "?" + rng + "&month=junk", "month"},
	}
	for _, c := range ignored {
		t.Run(c.name, func(t *testing.T) {
			code, id := e.postJob(t, c.pathQuery)
			if code != http.StatusAccepted || id == 0 {
				t.Fatalf("status = %d id = %d, want 202 with a job", code, id)
			}
			var raw []byte
			if err := e.pool.QueryRow(context.Background(), `SELECT report_params FROM export_jobs WHERE id = $1`, id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if _, ok := got[c.absentKey]; ok {
				t.Errorf("report_params has ignored key %q: %v", c.absentKey, got)
			}
			e.dropJobs(e.user.id)
			e.exec(`DELETE FROM audit_logs WHERE actor_id = $1`, e.user.id)
		})
	}
}

// Property 8: stored params, legacy entity/file name, dedupe of active jobs and the active cap.
func TestReportExportParamsStoredAndDedupeIntegration(t *testing.T) {
	e := newT63Env(t)
	vid, bid := e.vendorID, e.branchID

	t.Run("stored params", func(t *testing.T) {
		cases := []struct {
			kind      exportjob.Kind
			pathQuery string
			entity    string
			fileName  string
			params    map[string]any
		}{
			{exportjob.KindDsrLateDaily, fmt.Sprintf("dsr-late-daily?month=2026-06&vendor_id=%d&vendor_branch_id=%d&status=late", vid, bid),
				"daily", "dsr-late-daily-2026-06.csv",
				map[string]any{"month": "2026-06", "vendor_id": vid, "vendor_branch_id": bid, "status": "late"}},
			{exportjob.KindDsrLateSummary, "dsr-late-summary?month=2026-05",
				"summary", "dsr-late-summary-2026-05.csv",
				map[string]any{"month": "2026-05"}},
			{exportjob.KindReplenishmentAmountMismatch, fmt.Sprintf("replenishment-amount-mismatch?date_from=2026-06-01&date_to=2026-06-30&vendor_id=%d&region_code=JKT&classification=late", vid),
				"amount_mismatch", "replenishment-amount-mismatch-2026-06-01-2026-06-30.csv",
				map[string]any{"date_from": "2026-06-01", "date_to": "2026-06-30", "vendor_id": vid, "region_code": "JKT", "classification": "late"}},
			{exportjob.KindReplenishmentTrips, "replenishment-trips?date_from=2026-06-01&date_to=2026-06-30",
				"trips", "replenishment-trips-2026-06-01-2026-06-30.csv",
				map[string]any{"date_from": "2026-06-01", "date_to": "2026-06-30"}},
		}
		for _, c := range cases {
			t.Run(string(c.kind), func(t *testing.T) {
				code, id := e.postJob(t, c.pathQuery)
				if code != http.StatusAccepted {
					t.Fatalf("status = %d, want 202", code)
				}
				var kind, entity, fileName, status string
				var params []byte
				var statusFilter *string
				if err := e.pool.QueryRow(context.Background(),
					`SELECT kind, entity, file_name, status, status_filter, report_params FROM export_jobs WHERE id = $1`, id,
				).Scan(&kind, &entity, &fileName, &status, &statusFilter, &params); err != nil {
					t.Fatal(err)
				}
				if kind != string(c.kind) || entity != c.entity || fileName != c.fileName || status != "queued" {
					t.Errorf("row = kind %s entity %s file %s status %s, want %s %s %s queued", kind, entity, fileName, status, c.kind, c.entity, c.fileName)
				}
				if statusFilter != nil {
					t.Errorf("status_filter = %q, want NULL", *statusFilter)
				}
				var got map[string]any
				if err := json.Unmarshal(params, &got); err != nil {
					t.Fatal(err)
				}
				want := norm(t, c.params)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("report_params = %v, want %v", got, want)
				}
				e.dropJobs(e.user.id)
			})
		}
		e.exec(`DELETE FROM audit_logs WHERE actor_id = $1`, e.user.id)
	})

	t.Run("dedupe", func(t *testing.T) {
		e.dropJobs(e.user.id)
		e.exec(`DELETE FROM audit_logs WHERE actor_id = $1`, e.user.id)
		a := fmt.Sprintf("dsr-late-daily?month=2026-06&vendor_id=%d&status=late", vid)
		reordered := fmt.Sprintf("dsr-late-daily?status=late&vendor_id=%d&month=2026-06", vid)

		code, id1 := e.postJob(t, a)
		if code != http.StatusAccepted {
			t.Fatalf("first = %d, want 202", code)
		}
		expect := func(step string, wantCode int, wantID int64, wantJobs, wantAudits int64) {
			t.Helper()
			c, id := e.postJob(t, step)
			if c != wantCode || (wantID != 0 && id != wantID) {
				t.Fatalf("%s = %d id %d, want %d id %d", step, c, id, wantCode, wantID)
			}
			if jobs, audits := e.counts(); jobs != wantJobs || audits != wantAudits {
				t.Fatalf("%s: jobs=%d audits=%d, want %d/%d", step, jobs, audits, wantJobs, wantAudits)
			}
		}
		expect(a, http.StatusOK, id1, 1, 1)         // same, queued
		expect(reordered, http.StatusOK, id1, 1, 1) // same filter, other query order

		e.exec(`UPDATE export_jobs SET status = 'running', started_at = now() WHERE id = $1`, id1)
		expect(a, http.StatusOK, id1, 1, 1) // still active while running

		// A different filter (and a different kind with the same month) is a new job.
		expect(fmt.Sprintf("dsr-late-daily?month=2026-06&vendor_id=%d&status=ok", vid), http.StatusAccepted, 0, 2, 2)
		expect(fmt.Sprintf("dsr-late-daily?month=2026-05&vendor_id=%d&status=late", vid), http.StatusAccepted, 0, 3, 3)

		// Not active any more: the same request starts a new job.
		e.exec(`UPDATE export_jobs SET status = 'failed', finished_at = now(), error_message = 'T63 test' WHERE id = $1`, id1)
		c, id4 := e.postJob(t, a)
		if c != http.StatusAccepted || id4 == id1 || id4 == 0 {
			t.Fatalf("after failed: %d id %d (old %d), want 202 and a new id", c, id4, id1)
		}
		e.exec(`UPDATE export_jobs SET status = 'expired', finished_at = now(), expired_at = now() WHERE id = $1`, id4)
		if c, id5 := e.postJob(t, a); c != http.StatusAccepted || id5 == id4 {
			t.Fatalf("after expired: %d id %d (old %d), want 202 and a new id", c, id5, id4)
		}
	})

	t.Run("active cap", func(t *testing.T) {
		e.dropJobs(e.user.id)
		e.exec(`DELETE FROM audit_logs WHERE actor_id = $1`, e.user.id)
		for _, m := range []string{"2026-04", "2026-05", "2026-06"} {
			if code, _ := e.postJob(t, "dsr-late-summary?month="+m); code != http.StatusAccepted {
				t.Fatalf("month %s = %d, want 202", m, code)
			}
		}
		rec := e.post("dsr-late-summary?month=2026-03")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("4th active job = %d, want 429: %s", rec.Code, rec.Body.String())
		}
		if jobs, audits := e.counts(); jobs != 3 || audits != 3 {
			t.Errorf("after 429: jobs=%d audits=%d, want 3/3", jobs, audits)
		}
		// Dedupe wins over the cap: an identical active request still answers 200.
		if code, _ := e.postJob(t, "dsr-late-summary?month=2026-04"); code != http.StatusOK {
			t.Errorf("repeat at cap = %d, want 200", code)
		}
	})
}
