package service

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Saldo/kapasitas branch vault (cit-acm-plan FR3). All money is whole IDR in
// int64 -- never float; JSON carries amounts as decimal strings.

// denomAmounts maps a denomination (100000, 50000, ...) to an IDR amount.
type denomAmounts map[int32]int64

// branchSaldo is FR3.1 for one branch and date; unknown = no usable DSR data.
type branchSaldo struct {
	unknown bool
	amounts denomAmounts
}

func saldoFromRow(r db.ListVaultBranchSaldoRow) branchSaldo {
	return branchSaldo{unknown: r.Unknown, amounts: denomAmounts{
		100000: r.D100000, 50000: r.D50000, 20000: r.D20000, 10000: r.D10000,
		5000: r.D5000, 2000: r.D2000, 1000: r.D1000,
	}}
}

// atmKey identifies one ATM order (one ATM can sit on several requests for the same date).
type atmKey struct {
	requestID  int64
	terminalID string
}

// vaultDemand holds the FR3.2 + FR3.3 load per branch for one replenish date,
// precomputed once so capacity per (ATM, candidate) is O(1) (NFR1).
type vaultDemand struct {
	needs   map[atmKey]denomAmounts // order of each ATM
	charged map[atmKey]int64        // branch each ATM's order is charged to (0 = none)
	total   map[int64]denomAmounts  // per branch: sum of orders charged to it
}

// newVaultDemand charges every ATM order to its assigned vault branch (FR3.3)
// or, while unassigned, to its own replenish branch (FR3.2).
func newVaultDemand(rows []db.ListVaultDemandRow) *vaultDemand {
	d := &vaultDemand{needs: map[atmKey]denomAmounts{}, charged: map[atmKey]int64{}, total: map[int64]denomAmounts{}}
	for _, r := range rows {
		k := atmKey{r.VendorRequestID, r.TerminalID}
		if d.needs[k] == nil {
			d.needs[k] = denomAmounts{}
		}
		d.needs[k][r.Denom] += r.Amount

		var branch int64
		switch {
		case r.VaultBranchID != nil:
			branch = *r.VaultBranchID
		case r.ReplenishBranchID != nil:
			branch = *r.ReplenishBranchID
		}
		d.charged[k] = branch
		if branch == 0 {
			continue
		}
		if d.total[branch] == nil {
			d.total[branch] = denomAmounts{}
		}
		d.total[branch][r.Denom] += r.Amount
	}
	return d
}

// capacity is FR3.4: saldo - everything charged to branch, not counting the
// ATM being edited (k). ok=false when the saldo is unknown.
func (d *vaultDemand) capacity(saldo branchSaldo, branch int64, k atmKey) (denomAmounts, bool) {
	if saldo.unknown {
		return nil, false
	}
	out := denomAmounts{}
	for denom, amt := range saldo.amounts {
		out[denom] = amt
	}
	for denom, amt := range d.total[branch] {
		out[denom] -= amt
	}
	if d.charged[k] == branch {
		for denom, amt := range d.needs[k] {
			out[denom] += amt
		}
	}
	return out, true
}

// capacityWarning (FR4.3): saldo unknown, or remaining capacity below the
// ATM's order for any denomination it needs.
func capacityWarning(capacity denomAmounts, known bool, need denomAmounts) bool {
	if !known {
		return true
	}
	for denom, amt := range need {
		if amt > 0 && capacity[denom] < amt {
			return true
		}
	}
	return false
}

// coverage sums capacity over the denominations the ATM needs -- the sort key
// for "sisa kapasitas terbesar" within a tier (FR4.2).
func coverage(capacity denomAmounts, need denomAmounts) int64 {
	var sum int64
	for denom := range need {
		sum += capacity[denom]
	}
	return sum
}

// vaultTier is F6/FR4.2: 1 = same vendor + same region, 2 = other vendor same
// region, 3 = other region (urgent only). An ATM without a region code is tier 3 everywhere.
func vaultTier(candVendorID int64, candRegion string, atmVendorID int64, atmRegion *string) int16 {
	if atmRegion == nil || *atmRegion != candRegion {
		return 3
	}
	if candVendorID == atmVendorID {
		return 1
	}
	return 2
}

// VaultCandidate is one branch vault option for an ATM (FR4.2).
type VaultCandidate struct {
	VendorBranchID int64             `json:"vendor_branch_id"`
	BranchCode     string            `json:"branch_code"`
	BranchName     string            `json:"branch_name"`
	VendorName     string            `json:"vendor_name"`
	RegionCode     string            `json:"region_code"`
	Category       string            `json:"category"`
	Tier           int16             `json:"tier"`
	SaldoKnown     bool              `json:"saldo_known"`
	Saldo          map[string]string `json:"saldo"`
	Capacity       map[string]string `json:"capacity"`
	Warning        bool              `json:"capacity_warning"`
	coverage       int64
}

// sortCandidates orders by tier, then known saldo before unknown, then
// largest remaining capacity, then branch code (stable for the UI).
func sortCandidates(c []VaultCandidate) {
	sort.SliceStable(c, func(i, j int) bool {
		a, b := c[i], c[j]
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		if a.SaldoKnown != b.SaldoKnown {
			return a.SaldoKnown
		}
		if a.coverage != b.coverage {
			return a.coverage > b.coverage
		}
		return a.BranchCode < b.BranchCode
	})
}

// amountsJSON renders amounts as {"100000":"123"} (nil -> nil = unknown).
func amountsJSON(a denomAmounts) map[string]string {
	if a == nil {
		return nil
	}
	out := make(map[string]string, len(a))
	for denom, amt := range a {
		out[strconv.Itoa(int(denom))] = strconv.FormatInt(amt, 10)
	}
	return out
}

// snapshotBytes marshals a snapshot for a jsonb column; nil stays SQL NULL.
func snapshotBytes(a denomAmounts) []byte {
	if a == nil {
		return nil
	}
	b, _ := json.Marshal(amountsJSON(a)) // map[string]string cannot fail to marshal
	return b
}
