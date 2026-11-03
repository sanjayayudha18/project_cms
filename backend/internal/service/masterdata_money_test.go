package service

import (
	"math/big"
	"testing"
)

// be-tech-debt C4.4 money math: a package price is a decimal string with at
// most 18 integer digits and 2 decimals, stored exactly (never float); blank
// means "inherit" and becomes SQL NULL.
func TestValidatePriceContent_BasePrice(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // normalized value after validation
		wantErr bool
	}{
		{"integer", "1500000", "1500000", false},
		{"one decimal", "1500000.5", "1500000.5", false},
		{"two decimals", "1500000.55", "1500000.55", false},
		{"surrounding spaces trimmed", "  250000  ", "250000", false},
		{"zero is allowed", "0", "0", false},
		{"18 integer digits", "999999999999999999.99", "999999999999999999.99", false},
		{"blank stays blank (inherit)", "   ", "   ", false},
		{"negative", "-1", "", true},
		{"three decimals", "1.234", "", true},
		{"19 integer digits", "1000000000000000000", "", true},
		{"thousands separator", "1,500,000", "", true},
		{"exponent", "1e6", "", true},
		{"text", "abc", "", true},
		{"trailing dot", "100.", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.in
			err := validatePriceContent(&VendorPackagePriceContentPayload{BasePrice: &v})
			if tt.wantErr {
				ve, ok := err.(*ValidationError)
				if !ok || ve.Field != "base_price" {
					t.Fatalf("err = %v, want ValidationError on base_price", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v != tt.want {
				t.Fatalf("normalized = %q, want %q", v, tt.want)
			}
		})
	}
}

func TestValidatePriceContent_OptionalFields(t *testing.T) {
	blank, note, badDate, goodDate := "  ", "  SLA 4 jam  ", "15-10-2026", " 2026-12-31 "
	p := VendorPackagePriceContentPayload{SlaNote: &blank}
	if err := validatePriceContent(&p); err != nil || p.SlaNote != nil {
		t.Fatalf("blank SLA note: err %v, note %v, want nil note", err, p.SlaNote)
	}
	p = VendorPackagePriceContentPayload{SlaNote: &note, EffectiveEndDate: &goodDate}
	if err := validatePriceContent(&p); err != nil || *p.SlaNote != "SLA 4 jam" || *p.EffectiveEndDate != "2026-12-31" {
		t.Fatalf("trim: err %v, note %q, end %q", err, *p.SlaNote, *p.EffectiveEndDate)
	}
	p = VendorPackagePriceContentPayload{EffectiveEndDate: &badDate}
	if ve, ok := validatePriceContent(&p).(*ValidationError); !ok || ve.Field != "effective_end_date" {
		t.Fatalf("bad date: want ValidationError on effective_end_date")
	}
	p = VendorPackagePriceContentPayload{EffectiveEndDate: &blank}
	if err := validatePriceContent(&p); err != nil || p.EffectiveEndDate != nil {
		t.Fatalf("blank end date: err %v, end %v, want nil", err, p.EffectiveEndDate)
	}
}

// numericFromDecimalPtr is how every validated decimal string reaches the DB:
// exact digits, blank/nil -> NULL, garbage -> error (never a silent zero).
func TestNumericFromDecimalPtr(t *testing.T) {
	str := func(s string) *string { return &s }
	tests := []struct {
		name      string
		in        *string
		wantNull  bool
		wantExact string // rational form of the stored value
		wantErr   bool
	}{
		{"nil is NULL", nil, true, "", false},
		{"blank is NULL", str("  "), true, "", false},
		{"cents kept exactly", str("1000000.55"), false, "100000055/100", false},
		{"large amount exact", str("999999999999999999.99"), false, "99999999999999999999/100", false},
		{"not a number", str("12abc"), false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := numericFromDecimalPtr(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNull {
				if n.Valid {
					t.Fatalf("want NULL, got %+v", n)
				}
				return
			}
			r, err := numericToRat(n)
			if err != nil {
				t.Fatalf("numericToRat: %v", err)
			}
			want, _ := new(big.Rat).SetString(tt.wantExact)
			if r.Cmp(want) != 0 {
				t.Fatalf("stored %s, want %s", r.RatString(), want.RatString())
			}
		})
	}
}

func TestTierMaxInt32(t *testing.T) {
	if tierMaxInt32(nil) != nil {
		t.Fatal("nil tier max must stay nil (open-ended tier)")
	}
	v := int64(25)
	if got := tierMaxInt32(&v); got == nil || *got != 25 {
		t.Fatalf("tierMaxInt32(25) = %v", got)
	}
}
