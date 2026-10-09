package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func TestDigitsOrFallback(t *testing.T) {
	cases := map[string]string{"PAKET 3": "3", "PAKET 12A": "12", "PAKET KHUSUS": "0", "": "0"}
	for in, want := range cases {
		if got := digitsOrFallback(in); got != want {
			t.Errorf("digitsOrFallback(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapPriceAndPackageDBError(t *testing.T) {
	exclusion := &pgconn.PgError{Code: pgExclusionViolation}
	codeDup := &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: "vendor_package_prices_package_code_key"}
	otherDup := &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: "some_other_key"}
	plain := errors.New("boom")

	if err := mapPriceDBError("create", exclusion); !errors.Is(err, ErrVendorPackagePriceOverlap) {
		t.Errorf("price exclusion: got %v", err)
	}
	if err := mapPriceDBError("create", codeDup); !errors.Is(err, ErrVendorPackageCodeConflict) {
		t.Errorf("price package_code dup: got %v", err)
	}
	for _, in := range []error{otherDup, plain} {
		err := mapPriceDBError("update", in)
		if errors.Is(err, ErrVendorPackagePriceOverlap) || errors.Is(err, ErrVendorPackageCodeConflict) || !errors.Is(err, in) {
			t.Errorf("price passthrough of %v: got %v", in, err)
		}
	}

	if err := mapPackageDBError("create", exclusion); !errors.Is(err, ErrVendorPackageOverlap) {
		t.Errorf("package exclusion: got %v", err)
	}
	if err := mapPackageDBError("create", plain); errors.Is(err, ErrVendorPackageOverlap) || !errors.Is(err, plain) {
		t.Errorf("package passthrough: got %v", err)
	}
}

// Error paths that must fail before any SQL runs (nil tx is never touched).
func TestVendorPackagePriceApplier_RejectsBadChanges(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		change db.MasterDataChangeRequest
	}{
		{"unknown op", db.MasterDataChangeRequest{Op: "enable", Payload: []byte(`{}`)}},
		{"update without entity_id", db.MasterDataChangeRequest{Op: "update", Payload: []byte(`{}`)}},
		{"disable without entity_id", db.MasterDataChangeRequest{Op: "disable", Payload: []byte(`{}`)}},
		{"create bad json", db.MasterDataChangeRequest{Op: "create", Payload: []byte(`{`)}},
		{"create bad base_price", db.MasterDataChangeRequest{Op: "create", Payload: []byte(`{"base_price":"abc","effective_start_date":"2027-01-01"}`)}},
		{"create bad start date", db.MasterDataChangeRequest{Op: "create", Payload: []byte(`{"effective_start_date":"01-01-2027"}`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := (VendorPackagePriceApplier{}).Apply(ctx, nil, tc.change); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}
