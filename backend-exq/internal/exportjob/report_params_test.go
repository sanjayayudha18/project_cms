package exportjob

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/reports"
)

var reportNow = time.Date(2026, 10, 29, 10, 0, 0, 0, reports.WIB)

func q(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i < len(kv); i += 2 {
		v.Add(kv[i], kv[i+1])
	}
	return v
}

func sp(s string) *string { return &s }

func TestParseReportParams_Valid(t *testing.T) {
	tests := []struct {
		name   string
		kind   Kind
		query  url.Values
		entity string
		want   ReportParams
	}{
		{"daily full", KindDsrLateDaily, q("month", "2026-10", "vendor_id", "7", "vendor_branch_id", "3", "status", "late"),
			"daily", ReportParams{Month: sp("2026-10"), VendorID: id(7), VendorBranchID: id(3), Status: sp("late")}},
		{"daily month only", KindDsrLateDaily, q("month", "2026-09"), "daily", ReportParams{Month: sp("2026-09")}},
		{"summary ignores status", KindDsrLateSummary, q("month", "2026-10", "status", "bogus", "date_from", "x"),
			"summary", ReportParams{Month: sp("2026-10")}},
		{"mismatch full", KindReplenishmentAmountMismatch,
			q("date_from", "2026-10-01", "date_to", "2026-10-31", "vendor_id", "2", "region_code", "JKT", "classification", "late"),
			"amount_mismatch", ReportParams{DateFrom: sp("2026-10-01"), DateTo: sp("2026-10-31"), VendorID: id(2), RegionCode: sp("JKT"), Classification: sp("late")}},
		{"trips ignores classification and branch", KindReplenishmentTrips,
			q("date_from", "2026-10-01", "date_to", "2026-10-02", "classification", "bogus", "vendor_branch_id", "9", "month", "2026-10"),
			"trips", ReportParams{DateFrom: sp("2026-10-01"), DateTo: sp("2026-10-02")}},
		{"trips region kept", KindReplenishmentTrips, q("date_from", "2026-10-01", "date_to", "2026-10-01", "region_code", "SBY"),
			"trips", ReportParams{DateFrom: sp("2026-10-01"), DateTo: sp("2026-10-01"), RegionCode: sp("SBY")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseReportParams(tt.kind, tt.query, reportNow)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Kind != tt.kind || p.Entity != tt.entity || p.Report == nil {
				t.Fatalf("got kind=%q entity=%q report=%v", p.Kind, p.Entity, p.Report)
			}
			if got, want := string(reportHashBytes(p)), string(reportHashBytes(Params{Kind: tt.kind, Entity: tt.entity, Report: &tt.want})); got != want {
				t.Errorf("report = %s; want %s", got, want)
			}
		})
	}
}

func TestParseReportParams_Rejections(t *testing.T) {
	tests := []struct {
		name  string
		kind  Kind
		query url.Values
		field string
		msg   string
	}{
		{"month missing", KindDsrLateDaily, q(), "month", "month wajib diisi (YYYY-MM)"},
		{"month format", KindDsrLateSummary, q("month", "2026-13"), "month", "format month YYYY-MM"},
		{"month future", KindDsrLateDaily, q("month", "2026-11"), "month", "bulan tidak boleh setelah bulan berjalan"},
		{"daily vendor nan", KindDsrLateDaily, q("month", "2026-10", "vendor_id", "abc"), "vendor_id", "tidak valid"},
		{"daily vendor zero", KindDsrLateDaily, q("month", "2026-10", "vendor_id", "0"), "vendor_id", "tidak valid"},
		{"daily branch negative", KindDsrLateDaily, q("month", "2026-10", "vendor_branch_id", "-1"), "vendor_branch_id", "tidak valid"},
		{"daily status unknown", KindDsrLateDaily, q("month", "2026-10", "status", "bogus"), "status", "tidak dikenal"},
		{"dates missing", KindReplenishmentTrips, q(), "date_from", "date_from dan date_to wajib diisi"},
		{"only from", KindReplenishmentTrips, q("date_from", "2026-10-01"), "date_from", "date_from dan date_to wajib diisi"},
		{"from format", KindReplenishmentTrips, q("date_from", "01-10-2026", "date_to", "2026-10-02"), "date_from", "format tanggal YYYY-MM-DD"},
		{"to format", KindReplenishmentAmountMismatch, q("date_from", "2026-10-01", "date_to", "nope"), "date_to", "format tanggal YYYY-MM-DD"},
		{"to before from", KindReplenishmentTrips, q("date_from", "2026-10-05", "date_to", "2026-10-01"), "date_to", "date_to tidak boleh sebelum date_from"},
		{"range 32 days", KindReplenishmentTrips, q("date_from", "2026-10-01", "date_to", "2026-11-01"), "date_to", "rentang maksimal 31 hari"},
		{"vendor bad", KindReplenishmentTrips, q("date_from", "2026-10-01", "date_to", "2026-10-02", "vendor_id", "x"), "vendor_id", "tidak valid"},
		{"classification unknown", KindReplenishmentAmountMismatch, q("date_from", "2026-10-01", "date_to", "2026-10-02", "classification", "bogus"), "classification", "tidak dikenal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseReportParams(tt.kind, tt.query, reportNow)
			var ve *reports.ValidationError
			if !errors.Is(err, ErrValidation) || !errors.As(err, &ve) {
				t.Fatalf("err = %v; want ErrValidation wrapping *reports.ValidationError", err)
			}
			if ve.Field != tt.field || ve.Message != tt.msg {
				t.Errorf("got %s/%q; want %s/%q", ve.Field, ve.Message, tt.field, tt.msg)
			}
		})
	}
}

func TestParseReportParams_Boundaries(t *testing.T) {
	// 31 days inclusive passes; current month passes.
	if _, err := ParseReportParams(KindReplenishmentTrips, q("date_from", "2026-10-01", "date_to", "2026-10-31"), reportNow); err != nil {
		t.Errorf("31-day range: %v", err)
	}
	if _, err := ParseReportParams(KindDsrLateDaily, q("month", "2026-10"), reportNow); err != nil {
		t.Errorf("current month: %v", err)
	}
	if _, err := ParseReportParams(KindVendor, q(), reportNow); !errors.Is(err, ErrValidation) {
		t.Errorf("non-report kind: %v", err)
	}
	if !IsReportKind(KindDsrLateDaily) || IsReportKind(KindVendor) {
		t.Error("IsReportKind wrong")
	}
}

func TestReportKindsAliasPkgReports(t *testing.T) {
	for k, want := range map[Kind]string{
		KindDsrLateDaily: reports.DsrLateDaily, KindDsrLateSummary: reports.DsrLateSummary,
		KindReplenishmentAmountMismatch: reports.ReplenishmentAmountMismatch, KindReplenishmentTrips: reports.ReplenishmentTrips,
	} {
		if string(k) != want {
			t.Errorf("%q != %q", k, want)
		}
	}
}

func TestReportParamsHash(t *testing.T) {
	parse := func(kind Kind, query url.Values) Params {
		t.Helper()
		p, err := ParseReportParams(kind, query, reportNow)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := parse(KindDsrLateDaily, q("month", "2026-10", "vendor_id", "7", "status", "late"))
	b := parse(KindDsrLateDaily, q("status", "late", "vendor_id", "7", "month", "2026-10"))
	if ParamsHash(a) != ParamsHash(b) {
		t.Error("query order changed the hash")
	}
	a2 := parse(KindDsrLateDaily, q("month", "2026-10", "vendor_id", "7", "status", "late"))
	if ParamsHash(a) != ParamsHash(a2) || len(ParamsHash(a)) != 64 {
		t.Error("hash unstable or not hex sha256")
	}

	diff := []Params{
		parse(KindDsrLateDaily, q("month", "2026-10", "vendor_id", "8", "status", "late")),
		parse(KindDsrLateDaily, q("month", "2026-10", "vendor_id", "7")),
		parse(KindDsrLateDaily, q("month", "2026-09", "vendor_id", "7", "status", "late")),
		parse(KindDsrLateSummary, q("month", "2026-10", "vendor_id", "7")),
	}
	for i, d := range diff {
		if ParamsHash(d) == ParamsHash(a) {
			t.Errorf("diff[%d] collides with a", i)
		}
	}

	// A filter ignored by the kind does not change the hash.
	s1 := parse(KindDsrLateSummary, q("month", "2026-10"))
	s2 := parse(KindDsrLateSummary, q("month", "2026-10", "status", "late"))
	if ParamsHash(s1) != ParamsHash(s2) {
		t.Error("ignored status changed the summary hash")
	}

	// Report vs master data never equal, even with similar fields.
	master, err := ParseParams(KindVendor, "vendor", id(7), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if ParamsHash(master) == ParamsHash(a) {
		t.Error("report hash equals master-data hash")
	}
}

// vendor-pks-list: the vendor_pks branch of ParseReportParams (Req 10.1, 10.2).
func TestParseReportParams_VendorPks(t *testing.T) {
	q100, q101 := strings.Repeat("ü", 100), strings.Repeat("ü", 101) // runes, not bytes
	tests := []struct {
		name  string
		query url.Values
		want  ReportParams
	}{
		{"defaults", q(), ReportParams{Status: sp("terkini")}},
		{"all filters", q("q", "  Alfa ", "status", "need_renewal", "pks_type", " cit ", "vendor", " pt-a "),
			ReportParams{Q: sp("Alfa"), Status: sp("need_renewal"), PksType: sp("CIT"), Vendor: sp("PT-A")}},
		{"blank q dropped", q("q", "   "), ReportParams{Status: sp("terkini")}},
		{"q 100 runes ok", q("q", q100), ReportParams{Q: sp(q100), Status: sp("terkini")}},
		{"semua status", q("status", "semua"), ReportParams{Status: sp("semua")}},
		{"foreign params ignored", q("month", "bogus", "vendor_id", "x", "date_from", "x"), ReportParams{Status: sp("terkini")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseReportParams(KindVendorPks, tt.query, reportNow)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Kind != KindVendorPks || p.Entity != "vendor_pks" || p.Report == nil {
				t.Fatalf("got kind=%q entity=%q report=%v", p.Kind, p.Entity, p.Report)
			}
			if got, want := string(reportHashBytes(p)), string(reportHashBytes(Params{Kind: KindVendorPks, Entity: "vendor_pks", Report: &tt.want})); got != want {
				t.Errorf("report = %s; want %s", got, want)
			}
		})
	}

	for _, key := range []string{"terkini", "semua", "aktif", "need_renewal", "belum_berlaku", "expired", "terminated", "diganti"} {
		if _, err := ParseReportParams(KindVendorPks, q("status", key), reportNow); err != nil {
			t.Errorf("status %q rejected: %v", key, err)
		}
	}

	rejects := []struct {
		name  string
		query url.Values
		field string
		msg   string
	}{
		{"pks_status rejected", q("pks_status", "aktif"), "pks_status", "tidak dikenal"},
		{"pks_status belum_diisi rejected", q("pks_status", "belum_diisi"), "pks_status", "tidak dikenal"},
		{"vendor-level status active rejected", q("status", "active"), "status", "invalid status \"active\""},
		{"vendor-level status disabled rejected", q("status", "disabled"), "status", "invalid status \"disabled\""},
		{"vendor-level status all rejected", q("status", "all"), "status", "invalid status \"all\""},
		{"status belum_diisi rejected", q("status", "belum_diisi"), "status", "invalid status \"belum_diisi\""},
		{"status unknown", q("status", "bogus"), "status", "invalid status \"bogus\""},
		{"q 101 runes", q("q", q101), "q", "q must be at most 100 characters"},
	}
	for _, tt := range rejects {
		t.Run("reject "+tt.name, func(t *testing.T) {
			_, err := ParseReportParams(KindVendorPks, tt.query, reportNow)
			var ve *reports.ValidationError
			if !errors.Is(err, ErrValidation) || !errors.As(err, &ve) {
				t.Fatalf("err = %v; want ErrValidation wrapping *reports.ValidationError", err)
			}
			if ve.Field != tt.field || ve.Message != tt.msg {
				t.Errorf("got %s/%q; want %s/%q", ve.Field, ve.Message, tt.field, tt.msg)
			}
		})
	}
}

func TestVendorPksParamsHashAndAudit(t *testing.T) {
	parse := func(query url.Values) Params {
		t.Helper()
		p, err := ParseReportParams(KindVendorPks, query, reportNow)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := parse(q("q", "abc", "status", "semua", "pks_type", "FLM", "vendor", "PTA"))
	b := parse(q("vendor", "pta", "pks_type", "flm", "status", "semua", "q", " abc "))
	if ParamsHash(a) != ParamsHash(b) || len(ParamsHash(a)) != 64 {
		t.Error("hash depends on query order / q whitespace, or is not hex sha256")
	}
	// Default status spelled out equals the default.
	if ParamsHash(parse(q())) != ParamsHash(parse(q("status", "terkini"))) {
		t.Error("explicit default status changed the hash")
	}
	for i, d := range []Params{
		parse(q("q", "abd", "status", "semua", "pks_type", "FLM", "vendor", "PTA")),
		parse(q("q", "abc", "status", "aktif", "pks_type", "FLM", "vendor", "PTA")),
		parse(q("q", "abc", "status", "semua", "pks_type", "CIT", "vendor", "PTA")),
		parse(q("q", "abc", "status", "semua")),
	} {
		if ParamsHash(d) == ParamsHash(a) {
			t.Errorf("diff[%d] collides with a", i)
		}
	}
	// Other report kinds never share the hash, even with an empty filter.
	other, err := ParseReportParams(KindReplenishmentTrips, q("date_from", "2026-10-01", "date_to", "2026-10-01"), reportNow)
	if err != nil {
		t.Fatal(err)
	}
	if ParamsHash(other) == ParamsHash(parse(q())) {
		t.Error("vendor_pks hash equals a replenishment hash")
	}

	// Audit filter keeps the four keys (q, status, pks_type, vendor), absent = nil; file name is vendor-pks-<WIB day>.csv.
	f := a.Report.legacyFilter(KindVendorPks)
	if len(f) != 4 || *f["pks_type"].(*string) != "FLM" || *f["q"].(*string) != "abc" || *f["status"].(*string) != "semua" || *f["vendor"].(*string) != "PTA" {
		t.Errorf("legacyFilter = %v", f)
	}
	if f := parse(q()).Report.legacyFilter(KindVendorPks); f["q"].(*string) != nil || f["pks_type"].(*string) != nil || f["vendor"].(*string) != nil || *f["status"].(*string) != "terkini" {
		t.Errorf("absent filters must be nil except status=terkini, got %v", f)
	}
	late := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC) // 03:00 WIB next day
	if got := reports.VendorPksFileName(late); got != "vendor-pks-20261030.csv" {
		t.Errorf("file name = %q", got)
	}
}

func TestReportKindVendorPks(t *testing.T) {
	if string(KindVendorPks) != reports.VendorPks || !IsReportKind(KindVendorPks) {
		t.Error("KindVendorPks must alias reports.VendorPks and be a report kind")
	}
	for role, want := range map[string]bool{"ADMIN": true, "ADMIN_PARAM": true, "ATM-SPV": true, "VENDOR-USER": false, "": false} {
		if allowed(KindVendorPks, role) != want {
			t.Errorf("allowed(vendor_pks, %q) = %v, want %v", role, !want, want)
		}
	}
}
