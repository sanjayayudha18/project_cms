package service

import (
	"math/big"
	"testing"
)

func mustRat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("bad decimal %q", s)
	}
	return r
}

// Rows = phase 5.1 "Gerbang Sec 4 #7" table, seed set 5.000.000.000,00 / 15.000.000.000,00.
func TestClassifyCashCountVault(t *testing.T) {
	p := CashCountCategoryThresholds{
		MediumMin: mustRat(t, "5000000000.00"),
		MediumMax: mustRat(t, "15000000000.00"),
	}
	tests := []struct {
		name  string
		total string
		want  CashCountVaultCategory
	}{
		{"zero", "0.00", CashCountVaultLow},
		{"one cent below min", "4999999999.99", CashCountVaultLow},
		{"min inclusive", "5000000000.00", CashCountVaultMedium},
		{"max inclusive", "15000000000.00", CashCountVaultMedium},
		{"one cent above max", "15000000000.01", CashCountVaultHigh},
		{"FSD L737 Abacus Medan", "97822619068.00", CashCountVaultHigh},
		// FSD example says Medium; by the rule it is LOW (risk noted in phase file).
		{"FSD L737 Abacus Sukabumi", "1716024483.00", CashCountVaultLow},
		{"negative", "-1.00", CashCountVaultLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyCashCountVault(mustRat(t, tt.total), p); got != tt.want {
				t.Errorf("ClassifyCashCountVault(%s) = %s; want %s", tt.total, got, tt.want)
			}
		})
	}
}
