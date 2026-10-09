package service

import (
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func ptr[T any](v T) *T { return &v }

func TestVaultTier(t *testing.T) {
	jkt := "JKT"
	cases := []struct {
		name       string
		candVendor int64
		candRegion string
		atmVendor  int64
		atmRegion  *string
		want       int16
	}{
		{"same vendor same region", 1, "JKT", 1, &jkt, 1},
		{"other vendor same region", 2, "JKT", 1, &jkt, 2},
		{"same vendor other region", 1, "MKS", 1, &jkt, 3},
		{"atm without region", 1, "JKT", 1, nil, 3},
	}
	for _, tc := range cases {
		if got := vaultTier(tc.candVendor, tc.candRegion, tc.atmVendor, tc.atmRegion); got != tc.want {
			t.Errorf("%s: tier = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// Branch 10 = cash branch, 20 = replenish branch of ATM-B (unassigned -> charged to 20).
func demandFixture() *vaultDemand {
	return newVaultDemand([]db.ListVaultDemandRow{
		{VendorRequestID: 1, TerminalID: "A", Denom: 100000, Amount: 300, ReplenishBranchID: ptr(int64(20)), VaultBranchID: ptr(int64(10))},
		{VendorRequestID: 1, TerminalID: "A", Denom: 50000, Amount: 100, ReplenishBranchID: ptr(int64(20)), VaultBranchID: ptr(int64(10))},
		{VendorRequestID: 1, TerminalID: "B", Denom: 100000, Amount: 200, ReplenishBranchID: ptr(int64(20))},
		{VendorRequestID: 2, TerminalID: "A", Denom: 100000, Amount: 50, ReplenishBranchID: ptr(int64(20)), VaultBranchID: ptr(int64(10))},
	})
}

func TestVaultDemandCapacity(t *testing.T) {
	d := demandFixture()
	saldo := branchSaldo{amounts: denomAmounts{100000: 1000, 50000: 100}}

	// Other ATM looking at branch 10: everything charged to 10 is subtracted (300+50 / 100).
	capa, ok := d.capacity(saldo, 10, atmKey{1, "B"})
	if !ok || capa[100000] != 650 || capa[50000] != 0 {
		t.Fatalf("capacity for B @10 = %v %v, want 650/0", capa, ok)
	}
	// ATM A itself (assigned to 10) is not counted against its own capacity.
	capa, _ = d.capacity(saldo, 10, atmKey{1, "A"})
	if capa[100000] != 950 || capa[50000] != 100 {
		t.Fatalf("capacity for A @10 = %v, want 950/100", capa)
	}
	// Same terminal on another request is a different order: still charged.
	if d.needs[atmKey{2, "A"}][100000] != 50 {
		t.Fatalf("request 2 order lost: %v", d.needs)
	}
	// Unassigned B is charged to its replenish branch 20 (FR3.2).
	capa, _ = d.capacity(branchSaldo{amounts: denomAmounts{100000: 500}}, 20, atmKey{1, "A"})
	if capa[100000] != 300 {
		t.Fatalf("capacity @20 = %v, want 500-200=300", capa)
	}
	if _, ok := d.capacity(branchSaldo{unknown: true}, 10, atmKey{1, "B"}); ok {
		t.Fatal("unknown saldo must report ok=false")
	}
}

func TestCapacityWarning(t *testing.T) {
	need := denomAmounts{100000: 200, 50000: 0}
	cases := []struct {
		name  string
		capa  denomAmounts
		known bool
		want  bool
	}{
		{"enough", denomAmounts{100000: 200}, true, false},
		{"short", denomAmounts{100000: 199}, true, true},
		{"negative capacity", denomAmounts{100000: -5}, true, true},
		{"unknown saldo", nil, false, true},
		{"zero need ignored", denomAmounts{100000: 500, 50000: -1}, true, false},
	}
	for _, tc := range cases {
		if got := capacityWarning(tc.capa, tc.known, need); got != tc.want {
			t.Errorf("%s: warning = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSortCandidates(t *testing.T) {
	c := []VaultCandidate{
		{BranchCode: "T2", Tier: 2, SaldoKnown: true, coverage: 999},
		{BranchCode: "T1-unknown", Tier: 1, SaldoKnown: false},
		{BranchCode: "T1-small", Tier: 1, SaldoKnown: true, coverage: 10},
		{BranchCode: "T1-big", Tier: 1, SaldoKnown: true, coverage: 500},
		{BranchCode: "T3", Tier: 3, SaldoKnown: true, coverage: 9999},
	}
	sortCandidates(c)
	want := []string{"T1-big", "T1-small", "T1-unknown", "T2", "T3"}
	for i, w := range want {
		if c[i].BranchCode != w {
			t.Fatalf("order[%d] = %s, want %s (full %v)", i, c[i].BranchCode, w, c)
		}
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	if snapshotBytes(nil) != nil {
		t.Fatal("nil snapshot must stay SQL NULL")
	}
	got := decodeSnapshot(snapshotBytes(denomAmounts{100000: 1500000000000}))
	if got["100000"] != "1500000000000" {
		t.Fatalf("round trip = %v", got)
	}
}

func TestCheckReason(t *testing.T) {
	if r, err := checkReason("  ok  ", "f"); err != nil || r != "ok" {
		t.Fatalf("trim: %q %v", r, err)
	}
	if _, err := checkReason("   ", "f"); err != ErrRejectReasonEmpty {
		t.Fatalf("blank: %v", err)
	}
	long := make([]rune, 501)
	for i := range long {
		long[i] = 'é' // multi-byte: limit is runes, not bytes
	}
	if _, err := checkReason(string(long[:500]), "f"); err != nil {
		t.Fatalf("500 runes must pass: %v", err)
	}
	var ve *ValidationError
	if _, err := checkReason(string(long), "f"); !errors.As(err, &ve) {
		t.Fatalf("501 runes: want ValidationError, got %v", err)
	}
}
