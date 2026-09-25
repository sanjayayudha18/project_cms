package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VendorPackagePriceApplier is the Applier for entity_type="vendor_package_price"
// (vendor-pricing plan.md P20). The vpp_no_overlap exclusion constraint (009)
// is the authoritative overlap guard -- re-checked here inside the apply
// transaction, so two overlapping requests that were both pending cannot both
// land. A violation surfaces as ErrVendorPackagePriceOverlap (never the raw
// DB error); the approval handler maps it to a 409.
type VendorPackagePriceApplier struct{}

func priceDate(s string) (pgtype.Date, error) {
	t, err := time.Parse(priceDateLayout, s)
	if err != nil {
		return pgtype.Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func priceDatePtr(s *string) (pgtype.Date, error) {
	if s == nil {
		return pgtype.Date{}, nil
	}
	return priceDate(*s)
}

func tierMaxInt32(v *int64) *int32 {
	if v == nil {
		return nil
	}
	out := int32(*v)
	return &out
}

// mapPriceDBError translates the exclusion-violation and package_code
// unique-violation SQLSTATEs; other errors pass through wrapped with the
// operation name.
func mapPriceDBError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgExclusionViolation:
			return fmt.Errorf("%s vendor package price: %w", op, ErrVendorPackagePriceOverlap)
		case pgUniqueViolation:
			if pgErr.ConstraintName == "vendor_package_prices_package_code_key" {
				return fmt.Errorf("%s vendor package price: %w", op, ErrVendorPackageCodeConflict)
			}
		}
	}
	return fmt.Errorf("%s vendor package price: %w", op, err)
}

// packageDigitsRe extracts the numeric characters of a package label
// ("PAKET 3" -> "3") for embedding in a generated package_code.
var packageDigitsRe = regexp.MustCompile(`[^0-9]`)

// digitsOrFallback returns the numeric characters of label, or the
// deterministic fallback token "0" when label has none (Design Decision 1),
// keeping the generated code non-null and stable across re-runs.
func digitsOrFallback(label string) string {
	digits := packageDigitsRe.ReplaceAllString(label, "")
	if digits == "" {
		return "0"
	}
	return digits
}

// nextPackageCode derives the next per-vendor package_code on the SAME
// transaction as the insert (Req 5.4), so a rejected or still-pending change
// request never consumes a sequence value. Format:
// PKG<digits(label)>_<vendor.code>_<seq3>; seq = the max existing trailing
// 3-digit sequence for this vendor + 1 -- covers both migration-backfilled
// and previously-generated codes, since both share the format. The global
// UNIQUE(package_code) constraint (mapped in mapPriceDBError) is the final
// collision backstop (Req 5.3/5.5, Design Decision 4).
func nextPackageCode(ctx context.Context, tx pgx.Tx, vendorID int64, label string) (string, error) {
	// Serialize concurrent applies for the SAME vendor so two in-flight
	// approvals cannot read the same max seq; different vendors never block
	// each other (lock key is the vendor id).
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, vendorID); err != nil {
		return "", fmt.Errorf("lock vendor %d for package code: %w", vendorID, err)
	}

	var vendorCode string
	if err := tx.QueryRow(ctx, `SELECT code FROM vendors WHERE id = $1`, vendorID).Scan(&vendorCode); err != nil {
		return "", fmt.Errorf("load vendor code for package code: %w", err)
	}

	var maxSeq int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX((regexp_replace(package_code, '^.*_', ''))::int), 0)
		FROM vendor_package_prices WHERE vendor_id = $1`, vendorID).Scan(&maxSeq)
	if err != nil {
		return "", fmt.Errorf("load max package code sequence: %w", err)
	}

	return fmt.Sprintf("PKG%s_%s_%03d", digitsOrFallback(label), vendorCode, maxSeq+1), nil
}

// Apply implements Applier for entity_type="vendor_package_price".
func (VendorPackagePriceApplier) Apply(ctx context.Context, tx pgx.Tx, change db.MasterDataChangeRequest) (int64, any, error) {
	q := db.New(tx)

	switch change.Op {
	case "create":
		var p VendorPackagePricePayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor package price create payload: %w", err)
		}
		base, err := numericFromDecimalPtr(p.BasePrice)
		if err != nil {
			return 0, nil, err
		}
		start, err := priceDate(p.EffectiveStartDate)
		if err != nil {
			return 0, nil, err
		}
		end, err := priceDatePtr(p.EffectiveEndDate)
		if err != nil {
			return 0, nil, err
		}
		packageCode, err := nextPackageCode(ctx, tx, p.VendorID, p.Package)
		if err != nil {
			return 0, nil, fmt.Errorf("generate vendor package price code: %w", err)
		}
		created, err := q.CreateVendorPackagePriceAdmin(ctx, db.CreateVendorPackagePriceAdminParams{
			VendorID: p.VendorID, Package: p.Package, PackageCode: packageCode, MachineGroup: p.MachineGroup, PriceClass: p.PriceClass,
			TierMin: int32(p.TierMin), TierMax: tierMaxInt32(p.TierMax),
			BasePrice:      base,
			VendorBranchID: p.VendorBranchID, AtmID: p.AtmID, SlaNote: p.SlaNote, Currency: p.Currency,
			EffectiveStartDate: start, EffectiveEndDate: end,
		})
		if err != nil {
			return 0, nil, mapPriceDBError("create", err)
		}
		return created.ID, created, nil

	case "update":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("update requires entity_id")
		}
		var p VendorPackagePriceContentPayload
		if err := json.Unmarshal(change.Payload, &p); err != nil {
			return 0, nil, fmt.Errorf("unmarshal vendor package price update payload: %w", err)
		}
		base, err := numericFromDecimalPtr(p.BasePrice)
		if err != nil {
			return 0, nil, err
		}
		end, err := priceDatePtr(p.EffectiveEndDate)
		if err != nil {
			return 0, nil, err
		}
		updated, err := q.UpdateVendorPackagePriceAdmin(ctx, db.UpdateVendorPackagePriceAdminParams{
			ID: *change.EntityID, BasePrice: base,
			SlaNote: p.SlaNote, EffectiveEndDate: end,
		})
		if err != nil {
			return 0, nil, mapPriceDBError("update", err)
		}
		return *change.EntityID, updated, nil

	case "disable":
		if change.EntityID == nil {
			return 0, nil, fmt.Errorf("disable requires entity_id")
		}
		if err := q.DisableVendorPackagePriceAdmin(ctx, *change.EntityID); err != nil {
			return 0, nil, fmt.Errorf("disable vendor package price: %w", err)
		}
		return *change.EntityID, map[string]string{"effective_end_date": "closed"}, nil

	default:
		return 0, nil, fmt.Errorf("unsupported op=%s for entity_type=vendor_package_price", change.Op)
	}
}

// CurrentState implements Applier: same db.GetVendorPackagePriceAdminByIDRow
// shape VendorPackagePriceAdminService uses for SubmitRequest.Before.
func (VendorPackagePriceApplier) CurrentState(ctx context.Context, tx pgx.Tx, entityID int64) (any, error) {
	row, err := db.New(tx).GetVendorPackagePriceAdminByID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("load current vendor package price state: %w", err)
	}
	return row, nil
}
