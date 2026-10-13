package service

import (
	"errors"
	"testing"
)

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
