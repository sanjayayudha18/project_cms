package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func atmRow(terminal string, denom int32, amount, replenish int64) db.ListRequestPartyAtmRowsRow {
	ticket := terminal + "_T"
	return db.ListRequestPartyAtmRowsRow{
		TerminalID: terminal, LokasiAtm: "Lokasi " + terminal, TicketNumber: &ticket, Denom: denom, AmountReplenish: amount,
		ReplenishBranchID: replenish, ReplenishBranchCode: fmt.Sprintf("R%d", replenish), ReplenishBranchName: "R",
		ReplenishVendorID: 1, ReplenishVendorName: "V1",
	}
}

func vaultRow(vault, branch int64, denom int32, amount int64) db.ListRequestPartyVaultRowsRow {
	ticket := fmt.Sprintf("V%d_20261014_001", vault)
	return db.ListRequestPartyVaultRowsRow{
		VendorVaultID: vault, VaultCode: fmt.Sprintf("V%d", vault), TicketNumber: &ticket,
		VaultBranchID: branch, VaultBranchCode: fmt.Sprintf("C%d", branch), VaultBranchName: "C",
		VaultVendorID: 2, VaultVendorName: "V2", VaultAddress: "Jl. Vault", Denom: denom, Amount: amount,
	}
}

// cit-vault-select FR7: replenish parties see their own ATMs + every vault of
// the request; vault parties see their own vaults (two vaults of one branch =
// one party, two tickets) + the request's replenish branches; no ATM per vault.
func TestBuildPartyContents(t *testing.T) {
	atms := []db.ListRequestPartyAtmRowsRow{
		atmRow("A1", 50000, 100, 10), atmRow("A1", 100000, 200, 10),
		atmRow("A2", 100000, 300, 10),
		atmRow("A3", 100000, 400, 11),
	}
	vaults := []db.ListRequestPartyVaultRowsRow{
		vaultRow(201, 20, 50000, 100), vaultRow(201, 20, 100000, 500),
		vaultRow(202, 20, 100000, 300),
		vaultRow(301, 30, 100000, 100),
	}
	got := buildPartyContents(atms, vaults)
	if len(got) != 4 {
		t.Fatalf("parties = %d, want 4 (2 replenish + 2 vault)", len(got))
	}
	r10 := got[partyKey{10, partyRoleReplenish}]
	if len(r10.Atms) != 2 || r10.Atms[0].TerminalID != "A1" || len(r10.Atms[0].Denoms) != 2 || r10.Currency != "IDR" {
		t.Fatalf("replenish 10 ATMs = %+v", r10)
	}
	if want := []DenomAmount{{50000, 100}, {100000, 500}}; !equalDenoms(r10.Totals, want) {
		t.Fatalf("replenish 10 totals = %+v, want %+v", r10.Totals, want)
	}
	if len(r10.Vaults) != 3 || r10.Vaults[2].Vault.Code != "V301" || r10.Vaults[0].Vault.Address != "Jl. Vault" || r10.Vaults[0].Branch.VendorName != "V2" {
		t.Fatalf("replenish 10 sees every vault with address + branch: %+v", r10.Vaults)
	}
	v20 := got[partyKey{20, partyRoleVault}]
	if len(v20.Atms) != 0 || len(v20.Vaults) != 2 || v20.Vaults[0].TicketNumber != "V201_20261014_001" || v20.Vaults[1].TicketNumber != "V202_20261014_001" {
		t.Fatalf("vault 20 = %+v, want its two vaults with two tickets, no ATMs", v20)
	}
	if want := []DenomAmount{{50000, 100}, {100000, 800}}; !equalDenoms(v20.Totals, want) {
		t.Fatalf("vault 20 totals = %+v, want %+v", v20.Totals, want)
	}
	if len(v20.ReplenishBranches) != 2 || v20.ReplenishBranches[0].ID != 10 || v20.ReplenishBranches[1].ID != 11 {
		t.Fatalf("vault 20 replenish branches = %+v, want 10, 11", v20.ReplenishBranches)
	}
	if v30 := got[partyKey{30, partyRoleVault}]; len(v30.Vaults) != 1 || v30.Vaults[0].Vault.ID != 301 {
		t.Fatalf("vault 30 = %+v", v30)
	}
	// FR7.4: nothing internal in the snapshot.
	b, _ := json.Marshal(got[partyKey{10, partyRoleReplenish}])
	for _, leak := range []string{"saldo", "capacity", "tier", "urgent", "buffer"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("party content leaks %q: %s", leak, b)
		}
	}
}

func equalDenoms(a, b []DenomAmount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// jsonb reorders keys: the comparison must survive that, and detect a real change.
func TestSameContent(t *testing.T) {
	c := buildPartyContents([]db.ListRequestPartyAtmRowsRow{atmRow("A1", 100000, 200, 10)},
		[]db.ListRequestPartyVaultRowsRow{vaultRow(201, 20, 100000, 200)})[partyKey{20, partyRoleVault}]
	fresh, _ := json.Marshal(c)
	var m map[string]any
	_ = json.Unmarshal(fresh, &m)
	reordered, _ := json.Marshal(m) // map keys come out sorted, unlike the struct
	if !sameContent(reordered, fresh) {
		t.Fatal("same content with reordered keys reported as changed")
	}
	c.Vaults[0].Denoms[0].Amount = 201
	changed, _ := json.Marshal(c)
	if sameContent(reordered, changed) {
		t.Fatal("changed amount reported as same")
	}
	if sameContent([]byte("not json"), fresh) {
		t.Fatal("invalid stored content reported as same")
	}
}

func TestReapprovedSinceSent(t *testing.T) {
	t0 := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	ts := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	parties := []db.VendorRequestVendorParty{{SentAt: ts(t0)}, {SentAt: ts(t0.Add(time.Minute))}}
	cases := []struct {
		name     string
		approved pgtype.Timestamptz
		parties  []db.VendorRequestVendorParty
		want     bool
	}{
		{"first send", ts(t0), nil, false},
		{"vault reject resend", ts(t0.Add(-time.Hour)), parties, false},
		{"re-approved after edit", ts(t0.Add(time.Hour)), parties, true},
		{"approved between sends", ts(t0.Add(30 * time.Second)), parties, false},
		{"no approved_at", pgtype.Timestamptz{}, parties, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reapprovedSinceSent(db.VendorRequest{ApprovedAt: tc.approved}, tc.parties); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
