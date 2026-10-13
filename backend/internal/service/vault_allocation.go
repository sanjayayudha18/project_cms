package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Pembagian nominal request ke vault (cit-vault-select spec v3 FR3-FR5).
// Pure functions; VaultPlanService does the reads/writes. All money is whole
// IDR in int64 -- never float; JSON carries amounts as decimal strings.

// allocationDenoms are the banknotes an allocation may carry (FR4.2; same set
// as the CHECK on vendor_request_vault_allocation_amounts).
var allocationDenoms = []int32{100000, 50000, 20000, 10000, 5000, 2000, 1000}

const (
	reasonMinLen = 10
	reasonMaxLen = 500
)

// AllocationLineInput is one vault row the ACM-USER saves (FR4.1).
// Amounts: {"100000": "500000000"} in full IDR.
type AllocationLineInput struct {
	VendorVaultID int64             `json:"vendor_vault_id"`
	Amounts       map[string]string `json:"amounts"`
	IsUrgent      bool              `json:"is_urgent"`
	UrgentReason  *string           `json:"urgent_reason"`
}

// allocationLine is a validated input row, ready to insert.
type allocationLine struct {
	vault        db.ListVaultCandidateVaultsRow
	tier         int16
	amounts      denomAmounts
	isUrgent     bool
	urgentReason *string
}

// requestScope is what tiering needs about the request (FR3.2).
type requestScope struct {
	vendorID   int64
	regionCode *string
}

// validateAllocationLines checks every FR4.2 rule and returns the rows to
// insert. eligible holds the currently eligible vaults by id (FR3.1).
func validateAllocationLines(in []AllocationLineInput, eligible map[int64]db.ListVaultCandidateVaultsRow, req requestScope) ([]allocationLine, error) {
	seen := map[int64]bool{}
	out := make([]allocationLine, 0, len(in))
	for _, l := range in {
		v, ok := eligible[l.VendorVaultID]
		if !ok {
			return nil, allocErr("vault %d tidak aktif / bukan CASH-ATM_CASH / branch tidak aktif atau tanpa kode region", l.VendorVaultID)
		}
		if seen[l.VendorVaultID] {
			return nil, allocErr("vault %s dipilih lebih dari sekali", v.VaultCode)
		}
		seen[l.VendorVaultID] = true
		amounts, err := parseAllocationAmounts(v.VaultCode, l.Amounts)
		if err != nil {
			return nil, err
		}
		reason, err := urgentReason(v.VaultCode, l.IsUrgent, l.UrgentReason)
		if err != nil {
			return nil, err
		}
		tier := vaultTier(v.VendorID, v.RegionCode, req.vendorID, req.regionCode)
		if tier == 3 && !l.IsUrgent {
			return nil, allocErr("vault %s beda region hanya boleh sebagai urgent", v.VaultCode)
		}
		out = append(out, allocationLine{vault: v, tier: tier, amounts: amounts, isUrgent: l.IsUrgent, urgentReason: reason})
	}
	return out, nil
}

// parseAllocationAmounts: whole rupiah, > 0, a known denom, a multiple of it,
// at least one amount (FR4.2, V12).
func parseAllocationAmounts(vaultCode string, in map[string]string) (denomAmounts, error) {
	out := denomAmounts{}
	for key, raw := range in {
		d, err := strconv.ParseInt(key, 10, 32)
		if err != nil || !isAllocationDenom(int32(d)) {
			return nil, allocErr("vault %s: pecahan %q tidak dikenal", vaultCode, key)
		}
		amt, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, allocErr("vault %s: nominal %s harus bilangan bulat rupiah", vaultCode, key)
		}
		if amt <= 0 {
			return nil, allocErr("vault %s: nominal %s harus lebih dari 0", vaultCode, key)
		}
		if amt%d != 0 {
			return nil, allocErr("vault %s: nominal %s harus kelipatan Rp%d", vaultCode, key, d)
		}
		out[int32(d)] = amt
	}
	if len(out) == 0 {
		return nil, allocErr("vault %s: isi minimal satu nominal", vaultCode)
	}
	return out, nil
}

func isAllocationDenom(d int32) bool {
	for _, x := range allocationDenoms {
		if x == d {
			return true
		}
	}
	return false
}

// urgentReason: required 10-500 chars when urgent, absent otherwise.
func urgentReason(vaultCode string, isUrgent bool, reason *string) (*string, error) {
	r := strings.TrimSpace(deref(reason))
	if !isUrgent {
		if r != "" {
			return nil, allocErr("vault %s: alasan urgent hanya diisi bila urgent", vaultCode)
		}
		return nil, nil
	}
	if n := len([]rune(r)); n < reasonMinLen || n > reasonMaxLen {
		return nil, allocErr("vault %s: alasan urgent wajib %d-%d karakter", vaultCode, reasonMinLen, reasonMaxLen)
	}
	return &r, nil
}

// allocationTotals sums the lines per denom.
func allocationTotals(lines []allocationLine) denomAmounts {
	out := denomAmounts{}
	for _, l := range lines {
		for d, a := range l.amounts {
			out[d] += a
		}
	}
	return out
}

// checkCoverage is the FR4.3 submit rule: every denom's allocation covers the
// order; any excess (incl. a denom not ordered) needs a 10-500 char buffer
// reason. Returns the reason to store (nil when there is no excess).
func checkCoverage(order, allocated denomAmounts, bufferReason *string) (*string, error) {
	for _, d := range allocationDenoms {
		if short := order[d] - allocated[d]; short > 0 {
			return nil, allocErr("alokasi %s kurang Rp%d dari order", denomLabel(d), short)
		}
	}
	excess := false
	for d, a := range allocated {
		if a > order[d] {
			excess = true
		}
	}
	if !excess {
		return nil, nil
	}
	r := strings.TrimSpace(deref(bufferReason))
	if n := len([]rune(r)); n < reasonMinLen || n > reasonMaxLen {
		return nil, allocErr("alokasi melebihi order (buffer): alasan wajib %d-%d karakter", reasonMinLen, reasonMaxLen)
	}
	return &r, nil
}

func denomLabel(d int32) string {
	if d >= 1000 && d%1000 == 0 {
		return strconv.Itoa(int(d/1000)) + "K"
	}
	return strconv.Itoa(int(d))
}

// vaultSaldo is FR5.1 for one vault and date; unknown = no usable DSR data.
type vaultSaldo struct {
	unknown bool
	amounts denomAmounts
}

func vaultSaldoFromRow(r db.ListVaultSaldoRow) vaultSaldo {
	return vaultSaldo{unknown: r.Unknown, amounts: denomAmounts{
		100000: r.D100000, 50000: r.D50000, 20000: r.D20000, 10000: r.D10000,
		5000: r.D5000, 2000: r.D2000, 1000: r.D1000,
	}}
}

// allocatedByVault indexes ListVaultAllocatedTotals (other plans, FR5.2).
func allocatedByVault(rows []db.ListVaultAllocatedTotalsRow) map[int64]denomAmounts {
	out := map[int64]denomAmounts{}
	for _, r := range rows {
		if out[r.VendorVaultID] == nil {
			out[r.VendorVaultID] = denomAmounts{}
		}
		out[r.VendorVaultID][r.Denom] += r.Amount
	}
	return out
}

// vaultCapacity is FR5.3: saldo - allocations of other plans. ok=false when
// the saldo is unknown.
func vaultCapacity(s vaultSaldo, allocated denomAmounts) (denomAmounts, bool) {
	if s.unknown {
		return nil, false
	}
	out := denomAmounts{}
	for d, a := range s.amounts {
		out[d] = a - allocated[d]
	}
	return out, true
}

// VaultOption is one eligible vault for the plan (FR3.3).
type VaultOption struct {
	VendorVaultID int64             `json:"vendor_vault_id"`
	VaultCode     string            `json:"vault_code"`
	Category      string            `json:"category"`
	Tier          int16             `json:"tier"`
	SaldoKnown    bool              `json:"saldo_known"`
	Saldo         map[string]string `json:"saldo"`
	Capacity      map[string]string `json:"capacity"`
	coverage      int64
}

// VaultBranchOption groups the vaults of one vendor branch (FR3.4).
type VaultBranchOption struct {
	VendorBranchID int64         `json:"vendor_branch_id"`
	BranchCode     string        `json:"branch_code"`
	BranchName     string        `json:"branch_name"`
	VendorID       int64         `json:"vendor_id"`
	VendorName     string        `json:"vendor_name"`
	RegionCode     string        `json:"region_code"`
	Tier           int16         `json:"tier"`
	Vaults         []VaultOption `json:"vaults"`
}

// groupVaultOptions orders vaults within each branch, then branches, by tier,
// known saldo first, largest coverage of the ordered denoms, then code (FR3.4).
// Every branch carries at least one vault (FR3.1).
func groupVaultOptions(branches map[int64]*VaultBranchOption) []VaultBranchOption {
	out := make([]VaultBranchOption, 0, len(branches))
	for _, b := range branches {
		sort.SliceStable(b.Vaults, func(i, j int) bool { return vaultLess(b.Vaults[i], b.Vaults[j]) })
		out = append(out, *b)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		if va, vb := a.Vaults[0], b.Vaults[0]; va.SaldoKnown != vb.SaldoKnown || va.coverage != vb.coverage {
			return vaultLess(va, vb)
		}
		return a.BranchCode < b.BranchCode
	})
	return out
}

func vaultLess(a, b VaultOption) bool {
	if a.SaldoKnown != b.SaldoKnown {
		return a.SaldoKnown
	}
	if a.coverage != b.coverage {
		return a.coverage > b.coverage
	}
	return a.VaultCode < b.VaultCode
}

func allocErr(format string, args ...any) error {
	return &ValidationError{Field: "allocations", Message: fmt.Sprintf(format, args...)}
}
