package exportjob

import (
	"encoding/json"
	"net/url"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// Report kinds (report-export-migration 5.3): roles = report screens, audit = legacy report_exported.
func TestKindSpecs_ReportKinds(t *testing.T) {
	dsr := []string{"ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ADMIN"}
	repl := []string{"ADMIN", "ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"}
	cases := []struct {
		kind       Kind
		roles      []string
		entityType string
	}{
		{KindDsrLateDaily, dsr, "dsr_late_report"},
		{KindDsrLateSummary, dsr, "dsr_late_report"},
		{KindReplenishmentAmountMismatch, repl, "replenishment_report"},
		{KindReplenishmentTrips, repl, "replenishment_report"},
	}
	all := []string{"ADMIN", "ADMIN_PARAM", "APPACCESS", "ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV",
		"ACM-USER", "ACM-SPV", "VENDOR-USER", "VENDOR-ATM-USER", "VENDOR-VAULT-USER", ""}
	for _, tc := range cases {
		s, ok := KindSpecs[tc.kind]
		if !ok || s.DownloadAction != "report_exported" {
			t.Fatalf("%s: spec %+v", tc.kind, s)
		}
		if et, id := s.EntityRef(Job{}); et != tc.entityType || id != 0 {
			t.Errorf("%s: entity = %s/%d", tc.kind, et, id)
		}
		for _, role := range all {
			if got, want := allowed(tc.kind, role), slices.Contains(tc.roles, role); got != want {
				t.Errorf("%s role %q: allowed = %v, want %v", tc.kind, role, got, want)
			}
		}
	}
	// Master data stays admin-only even for report viewers (Req 4.3).
	for _, k := range []Kind{KindVendor, KindCabang, KindHargaPaket, KindMasterDataGlobal} {
		if allowed(k, "ATM-SPV") || allowed(k, "BRANCH-ATM-USER") {
			t.Errorf("%s opened to report roles", k)
		}
	}
}

func mustReport(t *testing.T, kind Kind, q string) Params {
	t.Helper()
	v, err := url.ParseQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseReportParams(kind, v, time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("%s %s: %v", kind, q, err)
	}
	return p
}

func TestReportJobAuditAndFileName(t *testing.T) {
	a := Actor{UserID: 5, Username: "atm.spv", Role: "ATM-SPV"}
	cases := []struct {
		kind       Kind
		query      string
		wantFile   string
		wantFilter map[string]any
	}{
		{KindDsrLateDaily, "month=2026-10&vendor_id=7&status=late", "dsr-late-daily-2026-10.csv",
			map[string]any{"month": "2026-10", "vendor_id": float64(7), "vendor_branch_id": nil, "status": "late"}},
		{KindDsrLateSummary, "month=2026-09&vendor_branch_id=3", "dsr-late-summary-2026-09.csv",
			map[string]any{"month": "2026-09", "vendor_id": nil, "vendor_branch_id": float64(3)}},
		{KindReplenishmentAmountMismatch, "date_from=2026-10-01&date_to=2026-10-31&classification=late", "replenishment-amount-mismatch-2026-10-01-2026-10-31.csv",
			map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-31", "vendor_id": nil, "region_code": nil, "classification": "late"}},
		{KindReplenishmentTrips, "date_from=2026-10-01&date_to=2026-10-07&region_code=JKT", "replenishment-trips-2026-10-01-2026-10-07.csv",
			map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-07", "vendor_id": nil, "region_code": "JKT"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			p := mustReport(t, tc.kind, tc.query)
			if got := FileName(p, "", "", time.Now()); got != tc.wantFile {
				t.Errorf("file name = %q, want %q", got, tc.wantFile)
			}
			if statusFilterOf(p) != nil {
				t.Error("report job must store status_filter NULL")
			}
			row := repository.ExportJob{ID: 88, Kind: string(p.Kind), Entity: p.Entity, ReportParams: reportParamsJSON(p.Report)}
			if j := jobFromRow(row); !reflect.DeepEqual(j.Params.Report, p.Report) {
				t.Errorf("report_params round trip = %+v, want %+v", j.Params.Report, p.Report)
			}
			after := requestedAfter(row, a)
			if after["report"] != p.Entity || after["export_job_id"] != int64(88) || after["export_kind"] != string(tc.kind) ||
				after["username"] != "atm.spv" || after["role"] != "ATM-SPV" {
				t.Errorf("after = %v", after)
			}
			// Compare the filter as it lands in audit_logs.after (JSON), nulls included.
			got := map[string]any{}
			for k, v := range after["filter"].(map[string]any) {
				got[k] = jsonValue(t, v)
			}
			if !reflect.DeepEqual(got, tc.wantFilter) {
				t.Errorf("filter = %v, want %v", got, tc.wantFilter)
			}
			if _, ok := after["status_filter"]; ok {
				t.Error("report audit must not carry master-data keys")
			}
		})
	}
}

func TestMasterDataJobUnchangedByReportColumns(t *testing.T) {
	p := Params{Kind: KindVendor, Entity: "vaults", StatusFilter: "active"}
	if sf := statusFilterOf(p); sf == nil || *sf != "active" || reportParamsJSON(p.Report) != nil {
		t.Fatalf("master-data insert values changed: %v", sf)
	}
	if reportParamsFromJSON(nil) != nil || reportParamsFromJSON([]byte("[1]")) != nil {
		t.Fatal("NULL / non-object report_params must decode to nil")
	}
	after := requestedAfter(repository.ExportJob{Kind: string(KindVendor), Entity: "vaults"}, Actor{})
	if _, ok := after["export_entity"]; !ok {
		t.Fatalf("master-data audit shape changed: %v", after)
	}
}

// jsonValue round-trips v through JSON, as the audit writer stores it.
func jsonValue(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
