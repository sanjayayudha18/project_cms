package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func strp(s string) *string { return &s }

func TestDenomCode(t *testing.T) {
	cases := []struct {
		denoms map[int32]bool
		want   string
	}{
		{map[int32]bool{50000: true}, "50K"},
		{map[int32]bool{100000: true}, "100K"},
		{map[int32]bool{50000: true, 100000: true}, "MIX"},
	}
	for _, c := range cases {
		if got := denomCode(c.denoms); got != c.want {
			t.Errorf("denomCode(%v) = %q, want %q", c.denoms, got, c.want)
		}
	}
}

func TestWantedCodes(t *testing.T) {
	items := []resolvedItem{
		{input: ItemInput{TerminalID: "Y18X", Denom: 50000}},
		{input: ItemInput{TerminalID: "Y18X", Denom: 100000}},
		{input: ItemInput{TerminalID: "A001", Denom: 100000}},
		{input: ItemInput{TerminalID: "A001", Denom: 100000}}, // second periode, same denom
	}
	want := map[string]string{"Y18X": "MIX", "A001": "100K"}
	if got := wantedCodes(items); !reflect.DeepEqual(got, want) {
		t.Errorf("wantedCodes = %v, want %v", got, want)
	}
}

func TestSingleRegion(t *testing.T) {
	jkt := branchRegion{branchCode: "B1", regionCode: strp("JKT")}
	jabar := branchRegion{branchCode: "B2", regionCode: strp("JABAR")}
	none := branchRegion{branchCode: "B3"}

	cases := []struct {
		name    string
		atms    map[string]branchRegion
		want    string
		code    string
		wantErr string
	}{
		{name: "one code", atms: map[string]branchRegion{"A": jkt, "B": jkt}, code: "JKT"},
		{name: "mixed codes", atms: map[string]branchRegion{"A": jkt, "C": jabar, "B": jkt},
			wantErr: "region vendor yang sama: JABAR: C; JKT: A,B"},
		{name: "branch without code", atms: map[string]branchRegion{"A": jkt, "D": none},
			wantErr: "region vendor cabang B3 belum punya kode region"},
		{name: "update same code", atms: map[string]branchRegion{"A": jkt}, want: "JKT", code: "JKT"},
		{name: "update other code", atms: map[string]branchRegion{"C": jabar}, want: "JKT",
			wantErr: "region vendor yang sama: request JKT; JABAR: C"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, err := singleRegion(c.atms, c.want)
			if c.wantErr == "" {
				if err != nil || code != c.code {
					t.Fatalf("singleRegion = %q, %v; want %q", code, err, c.code)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != "items" || !strings.Contains(ve.Message, c.wantErr) {
				t.Fatalf("singleRegion err = %v, want ValidationError containing %q", err, c.wantErr)
			}
		})
	}
}

func TestPlanTickets(t *testing.T) {
	tk := func(id int64, terminal, code string, active bool) db.VendorRequestTicket {
		return db.VendorRequestTicket{ID: id, TerminalID: terminal, DenomCode: code, IsActive: active}
	}
	cases := []struct {
		name     string
		existing []db.VendorRequestTicket
		want     map[string]string
		plan     ticketPlan
	}{
		{name: "create issues in terminal order", want: map[string]string{"B": "50K", "A": "MIX"},
			plan: ticketPlan{Issue: []ticketIssue{{"A", "MIX"}, {"B", "50K"}}}},
		{name: "unchanged ATM keeps its ticket", existing: []db.VendorRequestTicket{tk(1, "A", "50K", true)},
			want: map[string]string{"A": "50K"}},
		{name: "removed ATM is deactivated", existing: []db.VendorRequestTicket{tk(1, "A", "50K", true), tk(2, "B", "50K", true)},
			want: map[string]string{"A": "50K"}, plan: ticketPlan{Deactivate: []int64{2}}},
		{name: "code change deactivates and issues", existing: []db.VendorRequestTicket{tk(1, "A", "50K", true)},
			want: map[string]string{"A": "MIX"}, plan: ticketPlan{Deactivate: []int64{1}, Issue: []ticketIssue{{"A", "MIX"}}}},
		{name: "same code back reactivates", existing: []db.VendorRequestTicket{tk(1, "A", "50K", false), tk(2, "A", "MIX", true)},
			want: map[string]string{"A": "50K"}, plan: ticketPlan{Deactivate: []int64{2}, Reactivate: []int64{1}}},
		{name: "removed ATM added back reactivates", existing: []db.VendorRequestTicket{tk(1, "A", "50K", false)},
			want: map[string]string{"A": "50K"}, plan: ticketPlan{Reactivate: []int64{1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := planTickets(c.existing, c.want); !reflect.DeepEqual(got, c.plan) {
				t.Errorf("planTickets = %+v, want %+v", got, c.plan)
			}
		})
	}
}

func TestNormalizeBranchRegionCode(t *testing.T) {
	got, err := normalizeBranchRegionCode(strp("  jkt "))
	if err != nil || got == nil || *got != "JKT" {
		t.Fatalf("normalize(' jkt ') = %v, %v", got, err)
	}
	for _, blank := range []*string{nil, strp(""), strp("   ")} {
		if got, err := normalizeBranchRegionCode(blank); got != nil || err != nil {
			t.Errorf("normalize(blank) = %v, %v; want nil, nil", got, err)
		}
	}
	for _, bad := range []string{"J", "JKT-1", "ABCDEFGHIJK", keepCurrentCell} {
		if _, err := normalizeBranchRegionCode(strp(bad)); err == nil {
			t.Errorf("normalize(%q) accepted, want 422", bad)
		}
	}
}

func TestTicketDate(t *testing.T) {
	rd := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	if got := ticketDate(db.VendorRequest{ReplenishDate: toPgDate(rd)}); !got.Time.Equal(rd) {
		t.Errorf("with replenish_date: %v, want %v", got.Time, rd)
	}
	// Old VR- request: created 2026-09-10 18:30 UTC = 2026-09-11 01:30 WIB -> 2026-09-11 (FR7.3).
	created := pgtype.Timestamptz{Time: time.Date(2026, 9, 10, 18, 30, 0, 0, time.UTC), Valid: true}
	want := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	if got := ticketDate(db.VendorRequest{CreatedAt: created}); !got.Valid || !got.Time.Equal(want) {
		t.Errorf("without replenish_date: %v, want %v", got.Time, want)
	}
}
