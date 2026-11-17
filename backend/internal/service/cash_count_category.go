package service

import "math/big"

// CashCountVaultCategory is the H/M/L class of a vault for cash count (AC-5.1-02).
type CashCountVaultCategory string

const (
	CashCountVaultHigh   CashCountVaultCategory = "HIGH"
	CashCountVaultMedium CashCountVaultCategory = "MEDIUM"
	CashCountVaultLow    CashCountVaultCategory = "LOW"
)

// CashCountCategoryThresholds holds the Medium band of the active
// cash_count_category_params set, in full IDR precision (numeric(20,2)).
type CashCountCategoryThresholds struct {
	MediumMin *big.Rat // inclusive
	MediumMax *big.Rat // inclusive; above = HIGH
}

// ClassifyCashCountVault classifies a vault's total balance. Exact big.Rat
// comparison, never float. Below MediumMin (negative included) = LOW.
func ClassifyCashCountVault(total *big.Rat, p CashCountCategoryThresholds) CashCountVaultCategory {
	switch {
	case total.Cmp(p.MediumMin) < 0:
		return CashCountVaultLow
	case total.Cmp(p.MediumMax) <= 0:
		return CashCountVaultMedium
	default:
		return CashCountVaultHigh
	}
}
