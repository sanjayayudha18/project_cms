package service

import (
	"errors"
	"testing"
)

// Tiers are stored as int4 and appliers convert them with int32(...); values
// above maxTier must be rejected at validation, never silently truncated.
func TestPackageAndPriceGrain_TierUpperBound(t *testing.T) {
	cases := []struct {
		name      string
		min       int64
		max       *int64
		wantField string // "" = valid
	}{
		{"open range ok", 1, nil, ""},
		{"max at sentinel ok", 1, int64ptr(maxTier), ""},
		{"min at sentinel ok", maxTier, nil, ""},
		{"min above sentinel", maxTier + 1, nil, "tier_min"},
		{"min overflows int32", 1 << 31, nil, "tier_min"},
		{"max above sentinel", 1, int64ptr(maxTier + 1), "tier_max"},
		{"max overflows int32", 1, int64ptr(1 << 32), "tier_max"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkg := baseVendorPackageCreatePayload(9)
			pkg.TierMin, pkg.TierMax = tc.min, tc.max
			_, pkgErr := validatePackageGrain(&pkg)

			price := VendorPackagePriceCreatePayload{
				Package: "PAKET 3", MachineGroup: "ATM", PriceClass: "REGULAR",
				TierMin: tc.min, TierMax: tc.max, EffectiveStartDate: "2026-01-01",
			}
			_, priceErr := validatePriceGrain(&price)

			for name, err := range map[string]error{"package": pkgErr, "price": priceErr} {
				var ve *ValidationError
				if tc.wantField == "" {
					if errors.As(err, &ve) && (ve.Field == "tier_min" || ve.Field == "tier_max") {
						t.Fatalf("%s: unexpected tier error %v", name, err)
					}
					continue
				}
				if !errors.As(err, &ve) || ve.Field != tc.wantField {
					t.Fatalf("%s: want %s ValidationError, got %v", name, tc.wantField, err)
				}
			}
		})
	}
}
