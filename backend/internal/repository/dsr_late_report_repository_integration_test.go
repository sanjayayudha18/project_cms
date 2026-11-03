//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// dsrLateCases mirrors testdata/dsr_late_cases.json (plan D2, shared with Python P6).
type dsrLateCases struct {
	Month     string `json:"month"`
	StartDate string `json:"start_date"`
	Vendors   []struct {
		Key  string `json:"key"`
		Kind string `json:"kind"`
	} `json:"vendors"`
	Branches []struct {
		Key        string  `json:"key"`
		Vendor     string  `json:"vendor"`
		CreatedAt  string  `json:"created_at"`
		IsActive   bool    `json:"is_active"`
		DeletedAt  *string `json:"deleted_at"`
		RegionCode *string `json:"region_code"`
	} `json:"branches"`
	Vaults []struct {
		Key       string `json:"key"`
		Branch    string `json:"branch"`
		CreatedAt string `json:"created_at"`
		IsActive  bool   `json:"is_active"`
	} `json:"vaults"`
	Maps []struct {
		Vendor string `json:"vendor"`
		Label  string `json:"label"`
		Vault  string `json:"vault"`
	} `json:"maps"`
	Receipts []struct {
		Key         string   `json:"key"`
		Vendor      *string  `json:"vendor"`
		VendorLabel string   `json:"vendor_label"`
		ReportDate  string   `json:"report_date"`
		ReceivedAt  string   `json:"received_at"`
		Locations   []string `json:"locations"`
	} `json:"receipts"`
	Scenarios []struct {
		Name     string `json:"name"`
		AsOf     string `json:"as_of"`
		OnlyDate string `json:"only_date"`
		Expected []struct {
			ReportDate       string   `json:"report_date"`
			Branch           string   `json:"branch"`
			Status           string   `json:"status"`
			LateMinutes      int32    `json:"late_minutes"`
			Receipt          string   `json:"receipt"`
			MatchedLocations []string `json:"matched_locations"`
		} `json:"expected"`
		Warnings *struct {
			UnmappedVaults []struct {
				Branch         string `json:"branch"`
				Vault          string `json:"vault"`
				BranchExcluded bool   `json:"branch_excluded"`
			} `json:"unmapped_vaults"`
			UnmappedLabels []struct {
				Vendor       string `json:"vendor"`
				Label        string `json:"label"`
				ReceiptCount int32  `json:"receipt_count"`
			} `json:"unmapped_labels"`
			UnresolvedVendors []struct {
				VendorLabel  string `json:"vendor_label"`
				ReceiptCount int32  `json:"receipt_count"`
			} `json:"unresolved_vendors"`
		} `json:"warnings"`
	} `json:"scenarios"`
}

// dsrLateSeed holds the ids of one seeded fixture run.
type dsrLateSeed struct {
	tag       string
	vendorIDs map[string]int64
	branchIDs map[string]int64
	branchKey map[int64]string
	vaultIDs  map[string]int64
	receipts  map[string]dsrLateReceipt
}

type dsrLateReceipt struct {
	checksum   string
	receivedAt time.Time
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}

func mustDate(t *testing.T, s string) pgtype.Date {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return pgtype.Date{Time: d, Valid: true}
}

func loadDsrLateCases(t *testing.T) dsrLateCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "dsr_late_cases.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var c dsrLateCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return c
}

// seedDsrLateCases inserts the fixture inside tx; every code/name/label carries tag.
func seedDsrLateCases(t *testing.T, ctx context.Context, tx pgx.Tx, c dsrLateCases) dsrLateSeed {
	t.Helper()
	tag := time.Now().Format("150405.000000")
	s := dsrLateSeed{tag: tag, vendorIDs: map[string]int64{}, branchIDs: map[string]int64{},
		branchKey: map[int64]string{}, vaultIDs: map[string]int64{}, receipts: map[string]dsrLateReceipt{}}
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed %s: %v", what, err)
		}
	}
	for _, v := range c.Vendors {
		var id int64
		must("vendor "+v.Key, tx.QueryRow(ctx,
			`INSERT INTO vendors (code, name, kind) VALUES ($1, $1, $2) RETURNING id`,
			"DLR_"+v.Key+"_"+tag, v.Kind).Scan(&id))
		s.vendorIDs[v.Key] = id
	}
	for _, b := range c.Branches {
		var deletedAt *time.Time
		if b.DeletedAt != nil {
			d := mustTime(t, *b.DeletedAt)
			deletedAt = &d
		}
		var id int64
		must("branch "+b.Key, tx.QueryRow(ctx,
			`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active, deleted_at, created_at, region_code)
			 VALUES ($1, $2, $2, $3, $4, $5, $6) RETURNING id`,
			s.vendorIDs[b.Vendor], b.Key+"_"+tag, b.IsActive, deletedAt, mustTime(t, b.CreatedAt), b.RegionCode).Scan(&id))
		s.branchIDs[b.Key] = id
		s.branchKey[id] = b.Key
	}
	for _, v := range c.Vaults {
		var id int64
		must("vault "+v.Key, tx.QueryRow(ctx,
			`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, type, category, currency_code, is_active, created_at)
			 VALUES ($1, $2, 'CASH', 'CASH', 'IDR', $3, $4) RETURNING id`,
			s.branchIDs[v.Branch], v.Key+"_"+tag, v.IsActive, mustTime(t, v.CreatedAt)).Scan(&id))
		s.vaultIDs[v.Key] = id
	}
	for _, m := range c.Maps {
		_, err := tx.Exec(ctx,
			`INSERT INTO dsr_location_vault_maps (vendor_id, dsr_location, vendor_vault_id) VALUES ($1, $2, $3)`,
			s.vendorIDs[m.Vendor], m.Label+" "+tag, s.vaultIDs[m.Vault])
		must("map "+m.Label, err)
	}
	for _, r := range c.Receipts {
		var vendorID *int64
		label := r.VendorLabel + " " + tag
		if r.Vendor != nil {
			id := s.vendorIDs[*r.Vendor]
			vendorID = &id
			label = "DLR_" + *r.Vendor + "_" + tag
		}
		locs := make([]string, len(r.Locations))
		for i, l := range r.Locations {
			locs[i] = l + " " + tag
		}
		sum := sha256.Sum256([]byte(tag + r.Key))
		rc := dsrLateReceipt{checksum: hex.EncodeToString(sum[:]), receivedAt: mustTime(t, r.ReceivedAt)}
		_, err := tx.Exec(ctx,
			`INSERT INTO dsr_receipts (vendor_label, vendor_id, report_date, checksum, locations, received_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			label, vendorID, mustDate(t, r.ReportDate), rc.checksum, locs, rc.receivedAt)
		must("receipt "+r.Key, err)
		s.receipts[r.Key] = rc
	}
	return s
}

func (s dsrLateSeed) isFixtureVendor(id int64) bool {
	for _, v := range s.vendorIDs {
		if v == id {
			return true
		}
	}
	return false
}

func TestDsrLateReportRepository_SharedCases(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	c := loadDsrLateCases(t)
	s := seedDsrLateCases(t, ctx, tx, c)
	repo := NewDsrLateReportRepository(tx)
	monthStart := mustDate(t, c.Month+"-01")
	startDate := mustDate(t, c.StartDate)

	for _, sc := range c.Scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			asOf := pgtype.Timestamptz{Time: mustTime(t, sc.AsOf), Valid: true}
			rows, err := repo.ListDaily(ctx, db.ListDsrLateDailyParams{MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
			if err != nil {
				t.Fatalf("ListDaily: %v", err)
			}
			var got []db.ListDsrLateDailyRow
			for _, r := range rows {
				if !s.isFixtureVendor(r.VendorID) {
					continue
				}
				if sc.OnlyDate != "" && r.ReportDate.Time.Format(time.DateOnly) != sc.OnlyDate {
					continue
				}
				got = append(got, r)
			}
			if len(got) != len(sc.Expected) {
				for _, r := range got {
					t.Logf("got %s %s %s", r.ReportDate.Time.Format(time.DateOnly), s.branchKey[r.VendorBranchID], r.Status)
				}
				t.Fatalf("rows = %d, want %d", len(got), len(sc.Expected))
			}
			for i, want := range sc.Expected {
				r := got[i]
				where := want.ReportDate + " " + want.Branch
				if d := r.ReportDate.Time.Format(time.DateOnly); d != want.ReportDate || s.branchKey[r.VendorBranchID] != want.Branch {
					t.Fatalf("row %d = %s %s, want %s", i, d, s.branchKey[r.VendorBranchID], where)
				}
				if r.Status != want.Status || r.LateMinutes != want.LateMinutes {
					t.Errorf("%s: status/late = %s/%d, want %s/%d", where, r.Status, r.LateMinutes, want.Status, want.LateMinutes)
				}
				wantDeadline := mustTime(t, want.ReportDate+"T09:00:00+07:00")
				if !r.Deadline.Time.Equal(wantDeadline) {
					t.Errorf("%s: deadline = %v, want %v", where, r.Deadline.Time, wantDeadline)
				}
				if want.Receipt == "" {
					if r.FirstReceivedAt.Valid || r.ReceiptChecksum != "" || len(r.MatchedLocations) != 0 {
						t.Errorf("%s: want no receipt, got %+v", where, r)
					}
					continue
				}
				rc := s.receipts[want.Receipt]
				if !r.FirstReceivedAt.Time.Equal(rc.receivedAt) || r.ReceiptChecksum != rc.checksum {
					t.Errorf("%s: receipt = %v/%s, want %s at %v", where, r.FirstReceivedAt.Time, r.ReceiptChecksum, want.Receipt, rc.receivedAt)
				}
				wantLocs := make([]string, len(want.MatchedLocations))
				for j, l := range want.MatchedLocations {
					wantLocs[j] = l + " " + s.tag
				}
				if !reflect.DeepEqual(r.MatchedLocations, wantLocs) {
					t.Errorf("%s: matched = %v, want %v", where, r.MatchedLocations, wantLocs)
				}
			}

			if sc.OnlyDate != "" {
				return
			}
			assertDsrLateSummary(t, ctx, repo, s, got, monthStart, startDate, asOf)
			if sc.Warnings != nil {
				assertDsrLateWarnings(t, ctx, repo, s, sc.Warnings, monthStart, startDate, asOf)
			}
		})
	}
}

// assertDsrLateSummary checks ListSummary equals the per-branch aggregate of the daily rows.
func assertDsrLateSummary(t *testing.T, ctx context.Context, repo *DsrLateReportRepository, s dsrLateSeed,
	daily []db.ListDsrLateDailyRow, monthStart, startDate pgtype.Date, asOf pgtype.Timestamptz) {
	t.Helper()
	want := map[int64]*db.ListDsrLateSummaryRow{}
	for _, r := range daily {
		w, ok := want[r.VendorBranchID]
		if !ok {
			w = &db.ListDsrLateSummaryRow{VendorBranchID: r.VendorBranchID}
			want[r.VendorBranchID] = w
		}
		w.ObligedDays++
		w.LateMinutesTotal += r.LateMinutes
		switch r.Status {
		case "ok":
			w.OkDays++
		case "late":
			w.LateDays++
		case "not_sent":
			w.NotSentDays++
		case "pending":
			w.PendingDays++
		}
	}
	rows, err := repo.ListSummary(ctx, db.ListDsrLateSummaryParams{MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		t.Fatalf("ListSummary: %v", err)
	}
	seen := 0
	for _, r := range rows {
		if !s.isFixtureVendor(r.VendorID) {
			continue
		}
		seen++
		w := want[r.VendorBranchID]
		if w == nil {
			t.Errorf("summary has unexpected branch %s", s.branchKey[r.VendorBranchID])
			continue
		}
		got := [6]int32{r.ObligedDays, r.OkDays, r.LateDays, r.NotSentDays, r.PendingDays, r.LateMinutesTotal}
		exp := [6]int32{w.ObligedDays, w.OkDays, w.LateDays, w.NotSentDays, w.PendingDays, w.LateMinutesTotal}
		if got != exp {
			t.Errorf("summary %s = %v, want %v", s.branchKey[r.VendorBranchID], got, exp)
		}
	}
	if seen != len(want) {
		t.Errorf("summary branches = %d, want %d", seen, len(want))
	}
}

func assertDsrLateWarnings(t *testing.T, ctx context.Context, repo *DsrLateReportRepository, s dsrLateSeed, w *struct {
	UnmappedVaults []struct {
		Branch         string `json:"branch"`
		Vault          string `json:"vault"`
		BranchExcluded bool   `json:"branch_excluded"`
	} `json:"unmapped_vaults"`
	UnmappedLabels []struct {
		Vendor       string `json:"vendor"`
		Label        string `json:"label"`
		ReceiptCount int32  `json:"receipt_count"`
	} `json:"unmapped_labels"`
	UnresolvedVendors []struct {
		VendorLabel  string `json:"vendor_label"`
		ReceiptCount int32  `json:"receipt_count"`
	} `json:"unresolved_vendors"`
}, monthStart, startDate pgtype.Date, asOf pgtype.Timestamptz) {
	t.Helper()
	vaults, err := repo.ListUnmappedVaults(ctx, db.ListDsrLateUnmappedVaultsParams{MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		t.Fatalf("ListUnmappedVaults: %v", err)
	}
	var gotVaults []db.ListDsrLateUnmappedVaultsRow
	for _, v := range vaults {
		if s.isFixtureVendor(v.VendorID) {
			gotVaults = append(gotVaults, v)
		}
	}
	if len(gotVaults) != len(w.UnmappedVaults) {
		t.Fatalf("unmapped vaults = %+v, want %+v", gotVaults, w.UnmappedVaults)
	}
	for i, want := range w.UnmappedVaults {
		g := gotVaults[i]
		if g.VendorBranchID != s.branchIDs[want.Branch] || g.VendorVaultID != s.vaultIDs[want.Vault] || g.BranchExcluded != want.BranchExcluded {
			t.Errorf("unmapped vault %d = %+v, want %+v", i, g, want)
		}
	}

	labels, err := repo.ListUnmappedLabels(ctx, db.ListDsrLateUnmappedLabelsParams{MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		t.Fatalf("ListUnmappedLabels: %v", err)
	}
	var gotLabels []db.ListDsrLateUnmappedLabelsRow
	for _, l := range labels {
		if s.isFixtureVendor(l.VendorID) {
			gotLabels = append(gotLabels, l)
		}
	}
	if len(gotLabels) != len(w.UnmappedLabels) {
		t.Fatalf("unmapped labels = %+v, want %+v", gotLabels, w.UnmappedLabels)
	}
	for i, want := range w.UnmappedLabels {
		g := gotLabels[i]
		if g.VendorID != s.vendorIDs[want.Vendor] || g.DsrLocation != want.Label+" "+s.tag || g.ReceiptCount != want.ReceiptCount {
			t.Errorf("unmapped label %d = %+v, want %+v", i, g, want)
		}
	}

	vendors, err := repo.ListUnresolvedVendors(ctx, db.ListDsrLateUnresolvedVendorsParams{MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		t.Fatalf("ListUnresolvedVendors: %v", err)
	}
	for _, want := range w.UnresolvedVendors {
		found := false
		for _, g := range vendors {
			if g.VendorLabel == want.VendorLabel+" "+s.tag {
				found = g.ReceiptCount == want.ReceiptCount
			}
		}
		if !found {
			t.Errorf("unresolved vendors = %+v, want %+v", vendors, want)
		}
	}
}

// Filters, paging and the date window, on the same fixture (first scenario).
func TestDsrLateReportRepository_FiltersAndWindow(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	c := loadDsrLateCases(t)
	s := seedDsrLateCases(t, ctx, tx, c)
	repo := NewDsrLateReportRepository(tx)
	monthStart := mustDate(t, c.Month+"-01")
	startDate := mustDate(t, c.StartDate)
	asOf := pgtype.Timestamptz{Time: mustTime(t, c.Scenarios[0].AsOf), Valid: true}
	flm := s.vendorIDs["FLM"]
	ptr := func(v int64) *int64 { return &v }
	str := func(v string) *string { return &v }

	t.Run("vendor filter, count and paging agree", func(t *testing.T) {
		total, err := repo.CountDaily(ctx, db.CountDsrLateDailyParams{VendorID: &flm, MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
		if err != nil {
			t.Fatalf("CountDaily: %v", err)
		}
		if total != int64(len(c.Scenarios[0].Expected)) {
			t.Fatalf("CountDaily = %d, want %d", total, len(c.Scenarios[0].Expected))
		}
		page2, err := repo.ListDaily(ctx, db.ListDsrLateDailyParams{VendorID: &flm, PageLimit: ptr(5), PageOffset: 5,
			MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
		if err != nil {
			t.Fatalf("ListDaily page 2: %v", err)
		}
		if len(page2) != 5 || page2[0].ReportDate.Time.Format(time.DateOnly) != "2026-10-04" || s.branchKey[page2[0].VendorBranchID] != "NEW" {
			t.Fatalf("page 2 = %+v, want 5 rows starting 2026-10-04 NEW", page2)
		}
	})

	t.Run("branch + status filter", func(t *testing.T) {
		params := db.ListDsrLateDailyParams{VendorBranchID: ptr(s.branchIDs["PARTIAL"]), Status: str("not_sent"),
			MonthStart: monthStart, StartDate: startDate, AsOf: asOf}
		rows, err := repo.ListDaily(ctx, params)
		if err != nil {
			t.Fatalf("ListDaily: %v", err)
		}
		total, err := repo.CountDaily(ctx, db.CountDsrLateDailyParams{VendorBranchID: params.VendorBranchID, Status: params.Status,
			MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
		if err != nil {
			t.Fatalf("CountDaily: %v", err)
		}
		if len(rows) != 2 || total != 2 {
			t.Fatalf("PARTIAL not_sent = %d rows / count %d, want 2", len(rows), total)
		}
		sum, err := repo.ListSummary(ctx, db.ListDsrLateSummaryParams{VendorBranchID: params.VendorBranchID,
			MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
		if err != nil {
			t.Fatalf("ListSummary: %v", err)
		}
		if len(sum) != 1 || sum[0].ObligedDays != 4 || sum[0].LateMinutesTotal != 60 {
			t.Fatalf("PARTIAL summary = %+v, want 4 days / 60 min", sum)
		}
	})

	t.Run("warnings vendor filter", func(t *testing.T) {
		other := s.vendorIDs["ROH"]
		vaults, err := repo.ListUnmappedVaults(ctx, db.ListDsrLateUnmappedVaultsParams{VendorID: &other, MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
		if err != nil {
			t.Fatalf("ListUnmappedVaults: %v", err)
		}
		labels, err := repo.ListUnmappedLabels(ctx, db.ListDsrLateUnmappedLabelsParams{VendorID: &other, MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
		if err != nil {
			t.Fatalf("ListUnmappedLabels: %v", err)
		}
		if len(vaults) != 0 || len(labels) != 0 {
			t.Fatalf("INTERNAL vendor warnings = %+v / %+v, want none", vaults, labels)
		}
	})

	t.Run("empty window: month before start date, start after month", func(t *testing.T) {
		for name, p := range map[string]db.ListDsrLateDailyParams{
			"month before start":  {VendorID: &flm, MonthStart: mustDate(t, "2026-09-01"), StartDate: startDate, AsOf: asOf},
			"future start date":   {VendorID: &flm, MonthStart: monthStart, StartDate: mustDate(t, "2026-11-01"), AsOf: asOf},
			"as_of before window": {VendorID: &flm, MonthStart: monthStart, StartDate: startDate, AsOf: pgtype.Timestamptz{Time: mustTime(t, "2026-10-01T23:59:59+07:00"), Valid: true}},
		} {
			rows, err := repo.ListDaily(ctx, p)
			if err != nil {
				t.Fatalf("%s: ListDaily: %v", name, err)
			}
			if len(rows) != 0 {
				t.Errorf("%s: rows = %d, want 0", name, len(rows))
			}
		}
	})

	t.Run("as_of 00:30 WIB is already the next day (UTC still the day before)", func(t *testing.T) {
		at := pgtype.Timestamptz{Time: mustTime(t, "2026-10-04T00:30:00+07:00"), Valid: true}
		rows, err := repo.ListDaily(ctx, db.ListDsrLateDailyParams{VendorID: &flm, MonthStart: monthStart, StartDate: startDate, AsOf: at})
		if err != nil {
			t.Fatalf("ListDaily: %v", err)
		}
		last := rows[len(rows)-1]
		if d := last.ReportDate.Time.Format(time.DateOnly); d != "2026-10-04" || last.Status != "pending" {
			t.Fatalf("last row = %s %s, want 2026-10-04 pending", d, last.Status)
		}
	})
}
