package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
)

const (
	dsrQuery = "?month=2020-01&vendor_id=3&vendor_branch_id=5"
	repQuery = "?date_from=2026-10-01&date_to=2026-10-07&vendor_id=3&region_code=JKT"
)

func TestReportRoutesKindAndParams(t *testing.T) {
	cases := []struct {
		path, role string
		kind       exportjob.Kind
		entity     string
		check      func(*exportjob.ReportParams) bool
	}{
		{"/reports/dsr-late-daily" + dsrQuery + "&status=late", "ATM-USER", exportjob.KindDsrLateDaily, "daily",
			func(r *exportjob.ReportParams) bool {
				return *r.Month == "2020-01" && *r.VendorID == 3 && *r.VendorBranchID == 5 && *r.Status == "late"
			}},
		{"/reports/dsr-late-summary" + dsrQuery + "&status=late", "BRANCH-ATM-SPV", exportjob.KindDsrLateSummary, "summary",
			func(r *exportjob.ReportParams) bool { return *r.Month == "2020-01" && r.Status == nil }},
		{"/reports/replenishment-amount-mismatch" + repQuery + "&classification=late", "BRANCH-ATM-USER", exportjob.KindReplenishmentAmountMismatch, "amount_mismatch",
			func(r *exportjob.ReportParams) bool {
				return *r.DateFrom == "2026-10-01" && *r.DateTo == "2026-10-07" && *r.RegionCode == "JKT" && *r.Classification == "late"
			}},
		{"/reports/replenishment-trips" + repQuery, "ATM-SPV", exportjob.KindReplenishmentTrips, "trips",
			func(r *exportjob.ReportParams) bool { return *r.VendorID == 3 && r.Classification == nil }},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			f := &fakeSvc{created: true}
			rec := do(newRouter(f, c.role), "POST", c.path)
			if rec.Code != 202 || f.createCalls != 1 {
				t.Fatalf("got %d calls=%d: %s", rec.Code, f.createCalls, rec.Body)
			}
			p := f.gotParams
			if p.Kind != c.kind || p.Entity != c.entity || p.Report == nil || !c.check(p.Report) {
				t.Fatalf("params: %+v %+v", p, p.Report)
			}
		})
	}
}

func TestReportRoleMatrix(t *testing.T) {
	dsr := []string{"/reports/dsr-late-daily" + dsrQuery, "/reports/dsr-late-summary" + dsrQuery}
	rep := []string{"/reports/replenishment-amount-mismatch" + repQuery, "/reports/replenishment-trips" + repQuery}
	cases := []struct {
		role  string
		paths []string
		want  int
	}{
		{"ATM-USER", dsr, 202}, {"ATM-SPV", dsr, 202}, {"BRANCH-ATM-SPV", dsr, 202}, {"ADMIN", dsr, 202},
		{"BRANCH-ATM-USER", dsr, 403}, {"ADMIN_PARAM", dsr, 403}, {"VENDOR-USER", dsr, 403},
		{"ATM-USER", rep, 202}, {"BRANCH-ATM-USER", rep, 202}, {"BRANCH-ATM-SPV", rep, 202}, {"ADMIN", rep, 202},
		{"ADMIN_PARAM", rep, 403}, {"VENDOR-USER", rep, 403},
		{"", append(dsr, rep...), 401},
	}
	for _, c := range cases {
		for _, p := range c.paths {
			f := &fakeSvc{created: true}
			if rec := do(newRouter(f, c.role), "POST", p); rec.Code != c.want {
				t.Errorf("%s %s: got %d want %d", c.role, p, rec.Code, c.want)
			} else if c.want != 202 && f.createCalls != 0 {
				t.Errorf("%s %s: service reached", c.role, p)
			}
		}
	}
	// master-data routes stay admin-only for report roles
	for _, role := range []string{"ATM-SPV", "BRANCH-ATM-USER"} {
		if rec := do(newRouter(&fakeSvc{}, role), "POST", "/master-data/vendors"); rec.Code != 403 {
			t.Errorf("%s master-data: %d", role, rec.Code)
		}
	}
}

func TestReportValidation400(t *testing.T) {
	cases := []struct {
		name, path, field, message string
	}{
		{"bad month", "/reports/dsr-late-daily?month=2026-13", "month", "format month YYYY-MM"},
		{"missing month", "/reports/dsr-late-summary", "month", "month wajib diisi (YYYY-MM)"},
		{"future month", "/reports/dsr-late-daily?month=2999-01", "month", "bulan tidak boleh setelah bulan berjalan"},
		{"bad status", "/reports/dsr-late-daily?month=2020-01&status=zzz", "status", "tidak dikenal"},
		{"range over 31 days", "/reports/replenishment-trips?date_from=2026-09-01&date_to=2026-10-07", "date_to", "rentang maksimal 31 hari"},
		{"missing dates", "/reports/replenishment-amount-mismatch", "date_from", "date_from dan date_to wajib diisi"},
		{"bad vendor", "/reports/replenishment-trips?date_from=2026-10-01&date_to=2026-10-07&vendor_id=x", "vendor_id", "tidak valid"},
		{"bad classification", "/reports/replenishment-amount-mismatch" + repQuery + "&classification=zzz", "classification", "tidak dikenal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeSvc{created: true}
			rec := do(newRouter(f, "ADMIN"), "POST", c.path)
			if rec.Code != 400 || f.createCalls != 0 {
				t.Fatalf("got %d calls=%d: %s", rec.Code, f.createCalls, rec.Body)
			}
			var b struct {
				Success bool
				Error   struct {
					Code, Message string
					Details       []struct{ Field, Message string }
				}
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
				t.Fatal(err)
			}
			if b.Success || b.Error.Code != "validation_error" || b.Error.Message != "Validasi gagal" ||
				len(b.Error.Details) != 1 || b.Error.Details[0].Field != c.field || b.Error.Details[0].Message != c.message {
				t.Fatalf("body: %s", rec.Body)
			}
		})
	}
}

func TestReportCreateStatusAndErrors(t *testing.T) {
	path := "/reports/dsr-late-summary" + dsrQuery
	m := "2020-01"
	job := exportjob.Job{ID: 8, Params: exportjob.Params{Kind: exportjob.KindDsrLateSummary, Entity: "summary", Report: &exportjob.ReportParams{Month: &m}},
		FileName: "x.csv", Status: exportjob.StatusQueued, CreatedAt: time.Now()}
	for created, want := range map[bool]int{true: 202, false: 200} {
		if rec := do(newRouter(&fakeSvc{createJob: job, created: created}, "ADMIN"), "POST", path); rec.Code != want {
			t.Errorf("created=%v: %d", created, rec.Code)
		}
	}
	for _, c := range []struct {
		err  error
		want int
	}{{exportjob.ErrTooManyActive, 429}, {exportjob.ErrForbidden, 403}, {exportjob.ErrUnauthenticated, 401}, {errors.New("boom"), 500}} {
		if rec := do(newRouter(&fakeSvc{err: c.err}, "ADMIN"), "POST", path); rec.Code != c.want {
			t.Errorf("%v: %d", c.err, rec.Code)
		}
	}
}

func TestJobJSONReportParams(t *testing.T) {
	m, st := "2020-01", "late"
	vid := int64(3)
	report := exportjob.Job{ID: 8, Params: exportjob.Params{Kind: exportjob.KindDsrLateDaily, Entity: "daily",
		Report: &exportjob.ReportParams{Month: &m, Status: &st, VendorID: &vid}}, FileName: "x.csv", CreatedAt: time.Now()}
	master := exportjob.Job{ID: 9, Params: exportjob.Params{Kind: exportjob.KindMasterDataGlobal, Entity: "atms", StatusFilter: "all"}, FileName: "y.csv", CreatedAt: time.Now()}

	decode := func(j exportjob.Job) map[string]any {
		rec := httptest.NewRecorder()
		writeJSON(rec, 200, toJobJSON(j))
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	r := decode(report)
	rp, _ := r["report_params"].(map[string]any)
	if r["kind"] != "dsr_late_daily" || r["status_filter"] != nil || rp["month"] != "2020-01" || rp["status"] != "late" || rp["vendor_id"] != float64(3) {
		t.Fatalf("report job json: %v", r)
	}
	if _, ok := rp["date_from"]; ok {
		t.Fatalf("unset filters must be absent: %v", rp)
	}
	mj := decode(master)
	if v, ok := mj["report_params"]; !ok || v != nil || mj["status_filter"] != "all" {
		t.Fatalf("master job json: %v", mj)
	}

	// list and get expose report_params too
	f := &fakeSvc{createJob: report}
	for _, p := range []string{"/jobs", "/jobs/8"} {
		if rec := do(newRouter(f, "ATM-USER"), "GET", p); !strings.Contains(rec.Body.String(), `"report_params":{"month":"2020-01"`) {
			t.Errorf("%s: %s", p, rec.Body)
		}
	}
}
