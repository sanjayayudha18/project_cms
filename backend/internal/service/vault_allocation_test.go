package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// cit-vault-select spec v3 S2: pure allocation, coverage, capacity, ticket logic.

func allocFixture() (map[int64]db.ListVaultCandidateVaultsRow, requestScope) {
	region := "JKTSEL"
	eligible := map[int64]db.ListVaultCandidateVaultsRow{
		1: {VendorVaultID: 1, VaultCode: "VLT_A", VendorBranchID: 10, VendorID: 100, RegionCode: "JKTSEL"}, // tier 1
		2: {VendorVaultID: 2, VaultCode: "VLT_B", VendorBranchID: 20, VendorID: 200, RegionCode: "JKTSEL"}, // tier 2
		3: {VendorVaultID: 3, VaultCode: "VLT_C", VendorBranchID: 30, VendorID: 100, RegionCode: "JATIM"},  // tier 3
	}
	return eligible, requestScope{vendorID: 100, regionCode: &region}
}

func TestValidateAllocationLines(t *testing.T) {
	eligible, req := allocFixture()
	long := strings.Repeat("x", 501)
	tests := []struct {
		name    string
		in      []AllocationLineInput
		wantErr string
		tiers   []int16
	}{
		{"valid tier 1 + tier 2", []AllocationLineInput{
			{VendorVaultID: 1, Amounts: map[string]string{"100000": "500000000", "50000": "80000000"}},
			{VendorVaultID: 2, Amounts: map[string]string{"100000": "200000000"}},
		}, "", []int16{1, 2}},
		{"tier 3 urgent with reason", []AllocationLineInput{
			{VendorVaultID: 3, Amounts: map[string]string{"100000": "100000"}, IsUrgent: true, UrgentReason: strp("vault region kosong")},
		}, "", []int16{3}},
		{"empty list is a valid draft", nil, "", []int16{}},
		{"not eligible", []AllocationLineInput{{VendorVaultID: 9, Amounts: map[string]string{"100000": "100000"}}}, "tidak aktif", nil},
		{"duplicate vault", []AllocationLineInput{
			{VendorVaultID: 1, Amounts: map[string]string{"100000": "100000"}},
			{VendorVaultID: 1, Amounts: map[string]string{"50000": "50000"}},
		}, "lebih dari sekali", nil},
		{"no amount", []AllocationLineInput{{VendorVaultID: 1, Amounts: map[string]string{}}}, "minimal satu nominal", nil},
		{"zero amount", []AllocationLineInput{{VendorVaultID: 1, Amounts: map[string]string{"100000": "0"}}}, "lebih dari 0", nil},
		{"negative amount", []AllocationLineInput{{VendorVaultID: 1, Amounts: map[string]string{"100000": "-100000"}}}, "lebih dari 0", nil},
		{"not a multiple", []AllocationLineInput{{VendorVaultID: 1, Amounts: map[string]string{"100000": "150050000"}}}, "kelipatan Rp100000", nil},
		{"decimal rejected", []AllocationLineInput{{VendorVaultID: 1, Amounts: map[string]string{"100000": "100000.50"}}}, "bilangan bulat", nil},
		{"unknown denom", []AllocationLineInput{{VendorVaultID: 1, Amounts: map[string]string{"75000": "75000"}}}, "tidak dikenal", nil},
		{"tier 3 without urgent", []AllocationLineInput{{VendorVaultID: 3, Amounts: map[string]string{"100000": "100000"}}}, "hanya boleh sebagai urgent", nil},
		{"urgent reason too short", []AllocationLineInput{
			{VendorVaultID: 3, Amounts: map[string]string{"100000": "100000"}, IsUrgent: true, UrgentReason: strp("pendek")},
		}, "alasan urgent wajib", nil},
		{"urgent reason too long", []AllocationLineInput{
			{VendorVaultID: 1, Amounts: map[string]string{"100000": "100000"}, IsUrgent: true, UrgentReason: &long},
		}, "alasan urgent wajib", nil},
		{"reason without urgent", []AllocationLineInput{
			{VendorVaultID: 1, Amounts: map[string]string{"100000": "100000"}, UrgentReason: strp("alasan tanpa urgent")},
		}, "hanya diisi bila urgent", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := validateAllocationLines(tc.in, eligible, req)
			if tc.wantErr != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) || !strings.Contains(ve.Message, tc.wantErr) {
					t.Fatalf("err = %v, want ValidationError containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := make([]int16, len(lines))
			for i, l := range lines {
				got[i] = l.tier
			}
			if !reflect.DeepEqual(got, tc.tiers) {
				t.Fatalf("tiers = %v, want %v", got, tc.tiers)
			}
		})
	}
}

func TestValidateAllocationLines_TrimsUrgentReason(t *testing.T) {
	eligible, req := allocFixture()
	lines, err := validateAllocationLines([]AllocationLineInput{
		{VendorVaultID: 3, Amounts: map[string]string{"100000": " 200000 "}, IsUrgent: true, UrgentReason: strp("  vault region kosong  ")},
	}, eligible, req)
	if err != nil {
		t.Fatal(err)
	}
	if *lines[0].urgentReason != "vault region kosong" || lines[0].amounts[100000] != 200000 {
		t.Fatalf("line = %+v", lines[0])
	}
}

func TestCheckCoverage(t *testing.T) {
	order := denomAmounts{100000: 700000000, 50000: 80000000}
	tests := []struct {
		name      string
		allocated denomAmounts
		reason    *string
		wantErr   string
		wantNil   bool
	}{
		{"exact match, no reason needed", denomAmounts{100000: 700000000, 50000: 80000000}, nil, "", true},
		{"exact match drops a given reason", denomAmounts{100000: 700000000, 50000: 80000000}, strp("tidak dipakai sama sekali"), "", true},
		{"short by 1 rupiah on 50K", denomAmounts{100000: 700000000, 50000: 79999999}, nil, "50K kurang Rp1", false},
		{"missing denom entirely", denomAmounts{100000: 700000000}, nil, "50K kurang Rp80000000", false},
		{"excess without reason", denomAmounts{100000: 800000000, 50000: 80000000}, nil, "alasan wajib", false},
		{"excess with short reason", denomAmounts{100000: 800000000, 50000: 80000000}, strp("buffer"), "alasan wajib", false},
		{"excess with reason", denomAmounts{100000: 800000000, 50000: 80000000}, strp("  buffer hari raya  "), "", false},
		{"unordered denom counts as excess", denomAmounts{100000: 700000000, 50000: 80000000, 20000: 20000}, nil, "alasan wajib", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := checkCoverage(order, tc.allocated, tc.reason)
			if tc.wantErr != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) || !strings.Contains(ve.Message, tc.wantErr) {
					t.Fatalf("err = %v, want ValidationError containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil != (got == nil) {
				t.Fatalf("reason = %v, wantNil %v", got, tc.wantNil)
			}
			if got != nil && *got != "buffer hari raya" {
				t.Fatalf("reason not trimmed: %q", *got)
			}
		})
	}
}

func TestAllocationTotals(t *testing.T) {
	got := allocationTotals([]allocationLine{
		{amounts: denomAmounts{100000: 500000000, 50000: 80000000}},
		{amounts: denomAmounts{100000: 200000000}},
	})
	want := denomAmounts{100000: 700000000, 50000: 80000000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("totals = %v, want %v", got, want)
	}
}

func TestVaultCapacity(t *testing.T) {
	saldo := vaultSaldoFromRow(db.ListVaultSaldoRow{VendorVaultID: 1, D100000: 1000000000, D50000: 50000000})
	alloc := allocatedByVault([]db.ListVaultAllocatedTotalsRow{
		{VendorVaultID: 1, Denom: 100000, Amount: 300000000},
		{VendorVaultID: 1, Denom: 100000, Amount: 200000000},
		{VendorVaultID: 1, Denom: 50000, Amount: 80000000},
		{VendorVaultID: 2, Denom: 100000, Amount: 999},
	})
	capa, ok := vaultCapacity(saldo, alloc[1])
	if !ok || capa[100000] != 500000000 || capa[50000] != -30000000 {
		t.Fatalf("capacity = %v ok=%v", capa, ok)
	}
	if !capacityWarning(capa, ok, denomAmounts{50000: 10000000}) {
		t.Fatal("negative capacity must warn")
	}
	if capacityWarning(capa, ok, denomAmounts{100000: 500000000}) {
		t.Fatal("exact capacity must not warn")
	}

	unknown := vaultSaldoFromRow(db.ListVaultSaldoRow{VendorVaultID: 3, Unknown: true})
	if c, ok := vaultCapacity(unknown, nil); ok || c != nil {
		t.Fatalf("unknown saldo must give no capacity, got %v %v", c, ok)
	}
	if !capacityWarning(nil, false, denomAmounts{100000: 1}) {
		t.Fatal("unknown saldo must warn")
	}
}

func TestGroupVaultOptions(t *testing.T) {
	branches := map[int64]*VaultBranchOption{
		10: {VendorBranchID: 10, BranchCode: "B10", Tier: 2, Vaults: []VaultOption{{VaultCode: "V10", SaldoKnown: true, coverage: 900}}},
		20: {VendorBranchID: 20, BranchCode: "B20", Tier: 1, Vaults: []VaultOption{
			{VaultCode: "V20B", SaldoKnown: true, coverage: 100},
			{VaultCode: "V20A", SaldoKnown: false},
			{VaultCode: "V20C", SaldoKnown: true, coverage: 500},
		}},
		30: {VendorBranchID: 30, BranchCode: "B30", Tier: 1, Vaults: []VaultOption{{VaultCode: "V30", SaldoKnown: true, coverage: 800}}},
		40: {VendorBranchID: 40, BranchCode: "B40", Tier: 1, Vaults: []VaultOption{{VaultCode: "V40", SaldoKnown: false}}},
	}
	got := groupVaultOptions(branches)
	var order []string
	for _, b := range got {
		order = append(order, b.BranchCode)
	}
	if want := []string{"B30", "B20", "B40", "B10"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("branch order = %v, want %v", order, want)
	}
	var vaults []string
	for _, v := range got[1].Vaults {
		vaults = append(vaults, v.VaultCode)
	}
	if want := []string{"V20C", "V20B", "V20A"}; !reflect.DeepEqual(vaults, want) {
		t.Fatalf("vault order = %v, want %v", vaults, want)
	}
}

func TestPlanVaultTickets(t *testing.T) {
	existing := []db.ListVaultTicketsForRequestRow{
		{ID: 1, VendorVaultID: 10, TicketNumber: "VLT_A_20261014_001", IsActive: true},  // kept
		{ID: 2, VendorVaultID: 20, TicketNumber: "VLT_B_20261014_001", IsActive: true},  // dropped -> deactivate
		{ID: 3, VendorVaultID: 30, TicketNumber: "VLT_C_20261014_004", IsActive: false}, // back -> reactivate
	}
	got := planVaultTickets(existing, []int64{50, 10, 30, 40})
	want := vaultTicketPlan{
		Keep:       []string{"VLT_A_20261014_001"},
		Deactivate: []int64{2},
		Reactivate: []int64{3},
		Issue:      []int64{40, 50},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %+v, want %+v", got, want)
	}

	if p := planVaultTickets(nil, nil); len(p.Keep)+len(p.Deactivate)+len(p.Reactivate)+len(p.Issue) != 0 {
		t.Fatalf("empty input must give empty plan, got %+v", p)
	}
	all := planVaultTickets(existing, nil)
	if !reflect.DeepEqual(all.Deactivate, []int64{1, 2}) || len(all.Issue) != 0 || len(all.Reactivate) != 0 {
		t.Fatalf("no wanted vaults must deactivate every active ticket, got %+v", all)
	}
}

func TestDenomLabel(t *testing.T) {
	for d, want := range map[int32]string{100000: "100K", 50000: "50K", 1000: "1K", 500: "500"} {
		if got := denomLabel(d); got != want {
			t.Errorf("denomLabel(%d) = %q, want %q", d, got, want)
		}
	}
}
