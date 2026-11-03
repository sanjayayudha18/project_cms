package exportjob

import (
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

var safeFileName = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.csv$`)

// Feature: vendor-data-export, Property 9: file name is safe, follows the
// Kind's pattern and carries the WIB date.
func FuzzFileName(f *testing.F) {
	f.Add("ABC", "BR1", int64(1700000000), uint8(0), false)
	f.Add("a b/c\"d;e", "x y", int64(0), uint8(1), false)
	f.Add("", "", int64(-1), uint8(2), true)
	f.Add("Vé ndör/日本", "../..", int64(1735689599), uint8(2), false) // 16:59 UTC -> next WIB day edge
	f.Add("v", "b", int64(1735682400), uint8(3), false)

	kinds := []Kind{KindVendor, KindCabang, KindHargaPaket, KindMasterDataGlobal}

	f.Fuzz(func(t *testing.T, vendorCode, branchCode string, unix int64, kindIdx uint8, withBranch bool) {
		kind := kinds[int(kindIdx)%len(kinds)]
		ents := entitiesFor(kind)
		p := Params{Kind: kind, Entity: ents[0]}
		if withBranch {
			id := int64(1)
			p.BranchID = &id
		}
		now := time.Unix(unix%(1<<36), 0) // keep within a sane year range
		d := now.UTC().Add(7 * time.Hour).Format("20060102")

		name := FileName(p, vendorCode, branchCode, now)

		if !safeFileName.MatchString(name) {
			t.Fatalf("unsafe name %q", name)
		}
		if !strings.HasSuffix(name, "-"+d+".csv") {
			t.Fatalf("name %q lacks WIB date %s", name, d)
		}
		switch kind {
		case KindVendor:
			if !strings.HasPrefix(name, "vendor-") {
				t.Fatalf("bad vendor name %q", name)
			}
			if !strings.Contains(name, "-"+p.Entity+"-") {
				t.Fatalf("vendor name %q lacks entity", name)
			}
		case KindCabang:
			if !strings.HasPrefix(name, "vendor-") || !strings.Contains(name, "-cabang-") || !strings.Contains(name, "-"+p.Entity+"-") {
				t.Fatalf("bad cabang name %q", name)
			}
		case KindHargaPaket:
			if !strings.HasPrefix(name, "harga-paket-") {
				t.Fatalf("bad harga name %q", name)
			}
			if withBranch && !strings.Contains(name, "-cabang-") {
				t.Fatalf("harga name %q lacks cabang segment", name)
			}
		case KindMasterDataGlobal:
			if !strings.HasPrefix(name, "master-data-"+p.Entity+"-") {
				t.Fatalf("bad master-data name %q", name)
			}
		}
	})
}

// Feature: vendor-data-export, Property 17 (pure part): ParseParams returns
// valid enum-bound Params or a typed validation error; never panics; idempotent.
func FuzzParseParams(f *testing.F) {
	f.Add("vendor", "branches", int64(1), true, int64(0), false, "", "")
	f.Add("cabang", "vaults", int64(1), true, int64(2), true, "all", "")
	f.Add("harga_paket", "package-prices", int64(3), true, int64(0), false, "disabled", "branch")
	f.Add("harga_paket", "package-prices", int64(3), true, int64(4), true, "", "")
	f.Add("master_data_global", "atms", int64(0), false, int64(0), false, "active", "")
	f.Add("bogus", "", int64(-5), true, int64(-1), true, "x;y", "z")
	f.Add("vendor", "vendor", int64(1), true, int64(0), false, "ACTIVE", "all")

	f.Fuzz(func(t *testing.T, kind, entity string, vid int64, hasV bool, bid int64, hasB bool, status, priceLevel string) {
		var vp, bp *int64
		if hasV {
			vp = &vid
		}
		if hasB {
			bp = &bid
		}

		p, err := ParseParams(Kind(kind), entity, vp, bp, status, priceLevel)
		if err != nil {
			if !errors.Is(err, ErrValidation) && !errors.Is(err, ErrUnknownEntity) {
				t.Fatalf("unexpected error type: %v", err)
			}
			return
		}

		if !slices.Contains(entitiesFor(p.Kind), p.Entity) {
			t.Fatalf("entity %q not valid for kind %q", p.Entity, p.Kind)
		}
		if !slices.Contains([]string{"active", "disabled", "all"}, p.StatusFilter) {
			t.Fatalf("bad status %q", p.StatusFilter)
		}
		if status == "" && p.StatusFilter != "active" {
			t.Fatalf("status default = %q", p.StatusFilter)
		}
		if p.VendorID != nil && *p.VendorID <= 0 || p.BranchID != nil && *p.BranchID <= 0 {
			t.Fatalf("non-positive id in %+v", p)
		}
		pl := ""
		if p.Kind == KindHargaPaket {
			if p.PriceLevel == nil || !slices.Contains([]string{"vendor", "branch", "atm", "all"}, *p.PriceLevel) {
				t.Fatalf("bad price level in %+v", p)
			}
			if priceLevel == "" && *p.PriceLevel != "all" {
				t.Fatalf("price_level default = %q", *p.PriceLevel)
			}
			pl = *p.PriceLevel
		} else if p.PriceLevel != nil {
			t.Fatalf("price level set for kind %q", p.Kind)
		}

		again, err := ParseParams(p.Kind, p.Entity, p.VendorID, p.BranchID, p.StatusFilter, pl)
		if err != nil || !reflect.DeepEqual(again, p) {
			t.Fatalf("not idempotent: %+v -> %+v (%v)", p, again, err)
		}
	})
}
