package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func partyRow(terminal string, denom int32, amount, replenish, vault int64) db.ListRequestPartyRowsRow {
	ticket := terminal + "_T"
	return db.ListRequestPartyRowsRow{
		TerminalID: terminal, LokasiAtm: "Lokasi " + terminal, TicketNumber: &ticket, Denom: denom, AmountReplenish: amount,
		ReplenishBranchID: replenish, ReplenishBranchName: "R", ReplenishVendorID: 1, ReplenishVendorName: "V1",
		VaultBranchID: vault, VaultBranchName: "C", VaultVendorID: 2, VaultVendorName: "V2", VaultAddress: "Jl. Vault",
	}
}

// FR1/FR3: one content per (branch, role); a vault party only sees its own
// ATMs; totals per denom; counterpart is the other role's branch.
func TestBuildPartyContents(t *testing.T) {
	rows := []db.ListRequestPartyRowsRow{
		partyRow("A1", 50000, 100, 10, 20),
		partyRow("A1", 100000, 200, 10, 20),
		partyRow("A2", 100000, 300, 10, 21),
		partyRow("A3", 100000, 400, 11, 20),
	}
	got := buildPartyContents(rows)
	if len(got) != 4 {
		t.Fatalf("parties = %d, want 4 (2 replenish + 2 vault)", len(got))
	}
	r10 := got[partyKey{10, partyRoleReplenish}]
	if len(r10.Atms) != 2 || r10.Atms[0].TerminalID != "A1" || len(r10.Atms[0].Denoms) != 2 || r10.Atms[1].Counterpart.ID != 21 {
		t.Fatalf("replenish 10 = %+v", r10)
	}
	if r10.Atms[0].Counterpart.Address != "Jl. Vault" || r10.Currency != "IDR" {
		t.Fatalf("replenish sees vault address + IDR: %+v", r10)
	}
	v20 := got[partyKey{20, partyRoleVault}]
	if len(v20.Atms) != 2 || v20.Atms[0].TerminalID != "A1" || v20.Atms[1].TerminalID != "A3" {
		t.Fatalf("vault 20 ATMs = %+v, want A1, A3 only", v20.Atms)
	}
	if want := []DenomAmount{{50000, 100}, {100000, 600}}; !equalDenoms(v20.Totals, want) {
		t.Fatalf("vault 20 totals = %+v, want %+v", v20.Totals, want)
	}
	if v20.Atms[1].Counterpart.ID != 11 || v20.Atms[1].Counterpart.Address != "" {
		t.Fatalf("vault counterpart = %+v, want replenish 11 without address", v20.Atms[1].Counterpart)
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
	c := buildPartyContents([]db.ListRequestPartyRowsRow{partyRow("A1", 100000, 200, 10, 20)})[partyKey{20, partyRoleVault}]
	fresh, _ := json.Marshal(c)
	var m map[string]any
	_ = json.Unmarshal(fresh, &m)
	reordered, _ := json.Marshal(m) // map keys come out sorted, unlike the struct
	if !sameContent(reordered, fresh) {
		t.Fatal("same content with reordered keys reported as changed")
	}
	c.Atms[0].Denoms[0].Amount = 201
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
