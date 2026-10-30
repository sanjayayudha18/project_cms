package exportjob

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Kind is the export job type; values match export_jobs.kind.
type Kind string

const (
	KindVendor           Kind = "vendor"
	KindCabang           Kind = "cabang"
	KindHargaPaket       Kind = "harga_paket"
	KindMasterDataGlobal Kind = "master_data_global"
)

// Status is the export job state; values match export_jobs.status.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusRunning     Status = "running"
	StatusSucceeded   Status = "succeeded"
	StatusFailed      Status = "failed"
	StatusDownloading Status = "downloading"
	StatusDownloaded  Status = "downloaded"
	StatusExpired     Status = "expired"
)

const (
	statusActive  = "active"
	priceLevelAll = "all"
)

// Params is the validated, canonical input of one export job.
type Params struct {
	Kind         Kind
	Entity       string
	VendorID     *int64
	BranchID     *int64
	StatusFilter string        // active | disabled | all
	PriceLevel   *string       // only for KindHargaPaket
	Report       *ReportParams // only for report kinds
}

func entitiesFor(k Kind) []string {
	switch k {
	case KindVendor:
		return []string{"vendor", "branches", "vaults", "pics", "dsr-location-maps"}
	case KindCabang:
		return []string{"vaults", "pics", "dsr-location-maps"}
	case KindHargaPaket:
		return []string{"package-prices"}
	case KindMasterDataGlobal:
		return csvexport.ImportableNames()
	}
	return nil
}

// ParseParams validates raw input and applies defaults. Unknown kind, bad
// enums, ids <= 0 or a wrong scope shape give ErrValidation; an entity not
// valid for the kind gives ErrUnknownEntity.
func ParseParams(kind Kind, entity string, vendorID, branchID *int64, status, priceLevel string) (Params, error) {
	ents := entitiesFor(kind)
	if ents == nil {
		return Params{}, fmt.Errorf("%w: kind %q", ErrValidation, kind)
	}
	if !slices.Contains(ents, entity) {
		return Params{}, fmt.Errorf("%w: %q for kind %q", ErrUnknownEntity, entity, kind)
	}
	if status == "" {
		status = statusActive
	}
	if !slices.Contains([]string{statusActive, "disabled", "all"}, status) {
		return Params{}, fmt.Errorf("%w: status %q", ErrValidation, status)
	}
	if (vendorID != nil && *vendorID <= 0) || (branchID != nil && *branchID <= 0) {
		return Params{}, fmt.Errorf("%w: ids must be > 0", ErrValidation)
	}
	vendorNeeded := kind != KindMasterDataGlobal
	branchNeeded := kind == KindCabang
	branchAllowed := branchNeeded || kind == KindHargaPaket
	if vendorNeeded != (vendorID != nil) || (branchID != nil && !branchAllowed) || (branchNeeded && branchID == nil) {
		return Params{}, fmt.Errorf("%w: scope does not match kind %q", ErrValidation, kind)
	}

	p := Params{Kind: kind, Entity: entity, StatusFilter: status}
	if vendorID != nil {
		v := *vendorID
		p.VendorID = &v
	}
	if branchID != nil {
		b := *branchID
		p.BranchID = &b
	}
	if kind == KindHargaPaket {
		if priceLevel == "" {
			priceLevel = priceLevelAll
		}
		if !slices.Contains([]string{"vendor", "branch", "atm", priceLevelAll}, priceLevel) {
			return Params{}, fmt.Errorf("%w: price_level %q", ErrValidation, priceLevel)
		}
		p.PriceLevel = &priceLevel
	} else if priceLevel != "" {
		return Params{}, fmt.Errorf("%w: price_level only applies to harga_paket", ErrValidation)
	}
	return p, nil
}

func idStr(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

// ParamsHash is the hex sha256 of kind|entity|vendor_id|branch_id|status|price_level, or of the
// canonical JSON of kind, entity and ReportParams for a report.
func ParamsHash(p Params) string {
	if p.Report != nil {
		sum := sha256.Sum256(reportHashBytes(p))
		return hex.EncodeToString(sum[:])
	}
	pl := ""
	if p.PriceLevel != nil {
		pl = *p.PriceLevel
	}
	s := strings.Join([]string{string(p.Kind), p.Entity, idStr(p.VendorID), idStr(p.BranchID), p.StatusFilter, pl}, "|")
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var wib = time.FixedZone("WIB", 7*3600)

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		}
		return '_'
	}, s)
}

// FileName builds the download name (Req 5.2) with the WIB date of now.
func FileName(p Params, vendorCode, branchCode string, now time.Time) string {
	if p.Report != nil {
		return reportFileName(p.Kind, *p.Report)
	}
	d := now.In(wib).Format("20060102")
	v, b := sanitize(vendorCode), sanitize(branchCode)
	switch p.Kind {
	case KindVendor:
		return "vendor-" + v + "-" + p.Entity + "-" + d + ".csv"
	case KindCabang:
		return "vendor-" + v + "-cabang-" + b + "-" + p.Entity + "-" + d + ".csv"
	case KindHargaPaket:
		if p.BranchID != nil {
			return "harga-paket-" + v + "-cabang-" + b + "-" + d + ".csv"
		}
		return "harga-paket-" + v + "-" + d + ".csv"
	}
	return "master-data-" + p.Entity + "-" + d + ".csv"
}
