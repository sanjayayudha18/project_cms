package exportjob

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

func id(n int64) *int64 { return &n }

func TestParseParams(t *testing.T) {
	tests := []struct {
		name      string
		kind      Kind
		entity    string
		v, b      *int64
		status    string
		pl        string
		wantErr   error
		wantStat  string
		wantLevel string // "" = nil
	}{
		{"vendor defaults", KindVendor, "vendor", id(1), nil, "", "", nil, "active", ""},
		{"vendor disabled", KindVendor, "pics", id(1), nil, "disabled", "", nil, "disabled", ""},
		{"vendor all", KindVendor, "branches", id(1), nil, "all", "", nil, "all", ""},
		{"vendor dsr maps", KindVendor, "dsr-location-maps", id(1), nil, "", "", nil, "active", ""},
		{"vendor bad status", KindVendor, "vendor", id(1), nil, "ACTIVE", "", ErrValidation, "", ""},
		{"vendor missing id", KindVendor, "vendor", nil, nil, "", "", ErrValidation, "", ""},
		{"vendor with branch", KindVendor, "vendor", id(1), id(2), "", "", ErrValidation, "", ""},
		{"vendor zero id", KindVendor, "vendor", id(0), nil, "", "", ErrValidation, "", ""},
		{"vendor negative id", KindVendor, "vendor", id(-3), nil, "", "", ErrValidation, "", ""},
		{"vendor price_level", KindVendor, "vendor", id(1), nil, "", "all", ErrValidation, "", ""},
		{"vendor unknown entity", KindVendor, "atms", id(1), nil, "", "", ErrUnknownEntity, "", ""},
		{"cabang ok", KindCabang, "vaults", id(1), id(2), "", "", nil, "active", ""},
		{"cabang no branch", KindCabang, "vaults", id(1), nil, "", "", ErrValidation, "", ""},
		{"cabang no vendor", KindCabang, "vaults", nil, id(2), "", "", ErrValidation, "", ""},
		{"cabang zero branch", KindCabang, "vaults", id(1), id(0), "", "", ErrValidation, "", ""},
		{"cabang entity vendor", KindCabang, "vendor", id(1), id(2), "", "", ErrUnknownEntity, "", ""},
		{"cabang entity branches", KindCabang, "branches", id(1), id(2), "", "", ErrUnknownEntity, "", ""},
		{"harga default level", KindHargaPaket, "package-prices", id(1), nil, "", "", nil, "active", "all"},
		{"harga with branch", KindHargaPaket, "package-prices", id(1), id(2), "", "atm", nil, "active", "atm"},
		{"harga level vendor", KindHargaPaket, "package-prices", id(1), nil, "all", "vendor", nil, "all", "vendor"},
		{"harga level branch", KindHargaPaket, "package-prices", id(1), nil, "", "branch", nil, "active", "branch"},
		{"harga bad level", KindHargaPaket, "package-prices", id(1), nil, "", "x", ErrValidation, "", ""},
		{"harga no vendor", KindHargaPaket, "package-prices", nil, nil, "", "", ErrValidation, "", ""},
		{"harga bad entity", KindHargaPaket, "vaults", id(1), nil, "", "", ErrUnknownEntity, "", ""},
		{"global ok", KindMasterDataGlobal, "atms", nil, nil, "", "", nil, "active", ""},
		{"global with vendor", KindMasterDataGlobal, "atms", id(1), nil, "", "", ErrValidation, "", ""},
		{"global with branch", KindMasterDataGlobal, "atms", nil, id(1), "", "", ErrValidation, "", ""},
		{"global price_level", KindMasterDataGlobal, "atms", nil, nil, "", "all", ErrValidation, "", ""},
		{"global export-only entity", KindMasterDataGlobal, "package-prices", nil, nil, "", "", ErrUnknownEntity, "", ""},
		{"unknown kind", Kind("zzz"), "vendor", id(1), nil, "", "", ErrValidation, "", ""},
		{"empty kind", Kind(""), "", nil, nil, "", "", ErrValidation, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseParams(tc.kind, tc.entity, tc.v, tc.b, tc.status, tc.pl)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err %v", err)
			}
			if p.StatusFilter != tc.wantStat {
				t.Errorf("status = %q, want %q", p.StatusFilter, tc.wantStat)
			}
			switch {
			case tc.wantLevel == "" && p.PriceLevel != nil:
				t.Errorf("PriceLevel = %q, want nil", *p.PriceLevel)
			case tc.wantLevel != "" && (p.PriceLevel == nil || *p.PriceLevel != tc.wantLevel):
				t.Errorf("PriceLevel = %v, want %q", p.PriceLevel, tc.wantLevel)
			}
		})
	}
}

func TestParseParams_AllGlobalEntitiesAndNoAliasing(t *testing.T) {
	names := csvexport.ImportableNames()
	if len(names) != 6 {
		t.Fatalf("want 6 importable entities, got %d", len(names))
	}
	for _, n := range names {
		if _, err := ParseParams(KindMasterDataGlobal, n, nil, nil, "", ""); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	v := int64(5)
	p, _ := ParseParams(KindVendor, "vendor", &v, nil, "", "")
	v = 9
	if *p.VendorID != 5 {
		t.Errorf("Params aliases caller's id: %d", *p.VendorID)
	}
}

func TestParamsHash(t *testing.T) {
	base, _ := ParseParams(KindVendor, "vendor", id(1), nil, "", "")
	same, _ := ParseParams(KindVendor, "vendor", id(1), nil, "active", "")
	if ParamsHash(base) != ParamsHash(same) || len(ParamsHash(base)) != 64 {
		t.Fatal("hash must be stable, 64 hex chars")
	}
	seen := map[string]bool{ParamsHash(base): true}
	for _, f := range []func() (Params, error){
		func() (Params, error) { return ParseParams(KindVendor, "pics", id(1), nil, "", "") },
		func() (Params, error) { return ParseParams(KindVendor, "vendor", id(2), nil, "", "") },
		func() (Params, error) { return ParseParams(KindVendor, "vendor", id(1), nil, "all", "") },
		func() (Params, error) { return ParseParams(KindCabang, "vaults", id(1), id(1), "", "") },
		func() (Params, error) { return ParseParams(KindHargaPaket, "package-prices", id(1), nil, "", "") },
		func() (Params, error) { return ParseParams(KindHargaPaket, "package-prices", id(1), nil, "", "atm") },
		func() (Params, error) { return ParseParams(KindHargaPaket, "package-prices", id(1), id(1), "", "") },
	} {
		p, err := f()
		if err != nil {
			t.Fatal(err)
		}
		h := ParamsHash(p)
		if seen[h] {
			t.Errorf("hash collision for %+v", p)
		}
		seen[h] = true
	}
}

func TestFileName(t *testing.T) {
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	vp, _ := ParseParams(KindVendor, "branches", id(1), nil, "", "")
	cp, _ := ParseParams(KindCabang, "pics", id(1), id(2), "", "")
	hp, _ := ParseParams(KindHargaPaket, "package-prices", id(1), nil, "", "")
	hb, _ := ParseParams(KindHargaPaket, "package-prices", id(1), id(2), "", "")
	gp, _ := ParseParams(KindMasterDataGlobal, "atms", nil, nil, "", "")

	tests := []struct {
		name string
		p    Params
		v, b string
		now  time.Time
		want string
	}{
		{"vendor", vp, "ABC-1", "", now, "vendor-ABC-1-branches-20261028.csv"},
		{"cabang", cp, "ABC", "JKT_01", now, "vendor-ABC-cabang-JKT_01-pics-20261028.csv"},
		{"harga vendor", hp, "ABC", "", now, "harga-paket-ABC-20261028.csv"},
		{"harga cabang", hb, "ABC", "B1", now, "harga-paket-ABC-cabang-B1-20261028.csv"},
		{"global", gp, "", "", now, "master-data-atms-20261028.csv"},
		{"sanitize space slash quote", vp, "A B/C\"D;E", "", now, "vendor-A_B_C_D_E-branches-20261028.csv"},
		{"sanitize unicode per rune", vp, "Ünï世", "", now, "vendor-_n__-branches-20261028.csv"},
		{"sanitize path traversal", vp, "../x", "", now, "vendor-___x-branches-20261028.csv"},
		{"WIB boundary before", vp, "V", "", time.Date(2026, 10, 28, 16, 59, 59, 0, time.UTC), "vendor-V-branches-20261028.csv"},
		{"WIB boundary after", vp, "V", "", time.Date(2026, 10, 28, 17, 0, 0, 0, time.UTC), "vendor-V-branches-20261029.csv"},
		{"non-UTC input", vp, "V", "", time.Date(2026, 10, 28, 23, 0, 0, 0, time.FixedZone("X", -5*3600)), "vendor-V-branches-20261029.csv"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FileName(tc.p, tc.v, tc.b, tc.now)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if strings.ContainsAny(got, " /\\\";") {
				t.Errorf("unsafe chars in %q", got)
			}
		})
	}
}
