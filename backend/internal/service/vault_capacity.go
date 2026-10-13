package service

import (
	"encoding/json"
	"strconv"
)

// Shared money helpers for vault saldo/kapasitas (cit-vault-select FR5). All
// money is whole IDR in int64 -- never float; JSON carries amounts as decimal strings.

// denomAmounts maps a denomination (100000, 50000, ...) to an IDR amount.
type denomAmounts map[int32]int64

// capacityWarning (FR4.4): saldo unknown, or remaining capacity below the
// line's amount for any denomination it carries.
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

// coverage sums capacity over the denominations the request ordered -- the
// sort key for "sisa kapasitas terbesar" within a tier (FR3.4).
func coverage(capacity denomAmounts, need denomAmounts) int64 {
	var sum int64
	for denom := range need {
		sum += capacity[denom]
	}
	return sum
}

// vaultTier is FR3.2: 1 = same vendor + same region as the request, 2 = other
// vendor same region, 3 = other region (urgent only). No request region = tier 3 everywhere.
func vaultTier(candVendorID int64, candRegion string, reqVendorID int64, reqRegion *string) int16 {
	if reqRegion == nil || *reqRegion != candRegion {
		return 3
	}
	if candVendorID == reqVendorID {
		return 1
	}
	return 2
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
