package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Package sources of an ATM assignment (migration 023,
// .claude/sdlc/atm-package-source/spec.md). "branch" points at a
// vendor_packages_branch row; "vendor" stores vendor + managing branch + the
// vendor-wide package label (vendor_package_prices.package).
const (
	AssignmentSourceBranch = "branch"
	AssignmentSourceVendor = "vendor"
)

// ErrATMAssignmentSourceInvalid is a vendor-wide assignment that no longer
// passes the tariff/vendor/branch checks when it is applied (FR9). The approval
// handler maps it to 409 like the other apply-on-approve failures.
var ErrATMAssignmentSourceInvalid = errors.New("kelolaan ATM vendor-wide tidak lagi valid")

// vendorSourceQueries is the read surface checkVendorSource needs. *db.Queries
// (used inside the apply transaction) and the repository both satisfy it.
type vendorSourceQueries interface {
	GetATMPriceGroup(ctx context.Context, id int64) (db.GetATMPriceGroupRow, error)
	CheckAssignmentVendorBranch(ctx context.Context, arg db.CheckAssignmentVendorBranchParams) (int64, error)
	VendorTariffExistsForATM(ctx context.Context, arg db.VendorTariffExistsForATMParams) (bool, error)
}

// source returns the normalised source of the payload ("" means branch, which
// keeps payloads of requests staged before migration 023 valid).
func (p ATMAssignmentUpdatePayload) source() string {
	if p.Source == AssignmentSourceVendor {
		return AssignmentSourceVendor
	}
	return AssignmentSourceBranch
}

// dbColumns maps the payload onto the nullable atm_vendor_packages source
// columns; exactly one mode is populated (avp_source_chk).
func (p ATMAssignmentUpdatePayload) dbColumns() (pkgID, vendorID, branchID *int64, label *string) {
	if p.source() == AssignmentSourceVendor {
		return nil, &p.VendorID, &p.VendorBranchID, &p.Package
	}
	return &p.VendorPackageID, nil, nil, nil
}

// normalizeSource validates the shape of the payload (FR3): branch needs
// vendor_package_id and none of the vendor-wide fields, vendor needs vendor_id,
// vendor_branch_id and package and no vendor_package_id. It trims the label and
// canonicalises Source in place.
func normalizeSource(p *ATMAssignmentUpdatePayload) error {
	switch p.Source {
	case "", AssignmentSourceBranch:
		p.Source = AssignmentSourceBranch
		if p.VendorPackageID == 0 {
			return &ValidationError{Field: "vendor_package_id", Message: "wajib diisi"}
		}
		if p.VendorID != 0 || p.VendorBranchID != 0 || strings.TrimSpace(p.Package) != "" {
			return &ValidationError{Field: "source", Message: "paket cabang tidak boleh memuat vendor_id, vendor_branch_id atau package"}
		}
	case AssignmentSourceVendor:
		p.Package = strings.TrimSpace(p.Package)
		switch {
		case p.VendorPackageID != 0:
			return &ValidationError{Field: "source", Message: "paket vendor-wide tidak boleh memuat vendor_package_id"}
		case p.VendorID == 0:
			return &ValidationError{Field: "vendor_id", Message: "wajib diisi"}
		case p.VendorBranchID == 0:
			return &ValidationError{Field: "vendor_branch_id", Message: "wajib diisi"}
		case p.Package == "":
			return &ValidationError{Field: "package", Message: "wajib diisi"}
		}
	default:
		return &ValidationError{Field: "source", Message: "harus branch atau vendor"}
	}
	return nil
}

// checkVendorSource is the FR9 check for a vendor-wide assignment, run at submit
// and again at apply (the situation can change in between): the ATM has a price
// group, the vendor is an active FLM vendor owning an active branch, and the
// vendor has a tariff for the label that matches the ATM and is effective on asOf.
func checkVendorSource(ctx context.Context, q vendorSourceQueries, atmID int64, p ATMAssignmentUpdatePayload, asOf time.Time) error {
	grp, err := q.GetATMPriceGroup(ctx, atmID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssignmentATMNotFound
	}
	if err != nil {
		return fmt.Errorf("checking atm price group: %w", err)
	}
	if grp.PriceMachineGroup == nil || grp.PriceClass == nil {
		return &ValidationError{Field: "package", Message: "jenis mesin/kelas prioritas ATM belum terpetakan ke tarif"}
	}
	_, err = q.CheckAssignmentVendorBranch(ctx, db.CheckAssignmentVendorBranchParams{VendorID: p.VendorID, VendorBranchID: p.VendorBranchID})
	if errors.Is(err, pgx.ErrNoRows) {
		return &ValidationError{Field: "vendor_branch_id", Message: "cabang tidak ditemukan, nonaktif, atau bukan milik vendor FLM yang dipilih"}
	}
	if err != nil {
		return fmt.Errorf("checking vendor branch: %w", err)
	}
	ok, err := q.VendorTariffExistsForATM(ctx, db.VendorTariffExistsForATMParams{
		VendorID: p.VendorID, Package: p.Package, AtmID: atmID, AsOf: pgtype.Date{Time: asOf, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("checking vendor tariff: %w", err)
	}
	if !ok {
		return &ValidationError{Field: "package", Message: "vendor tidak punya tarif aktif untuk paket ini pada jenis mesin ATM ini"}
	}
	return nil
}

// assignmentAsOf is the date the period starts: the explicit start date, or the
// approval day (WIB) for an automatic period.
func assignmentAsOf(p ATMAssignmentUpdatePayload, now time.Time) time.Time {
	if p.EffectiveStartDate != "" {
		if d, err := time.Parse(assignmentDateLayout, p.EffectiveStartDate); err == nil {
			return d
		}
	}
	y, m, d := now.In(wibZone).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// validateSourceAtApply re-runs the FR9 check inside the apply transaction and
// reports a failure as ErrATMAssignmentSourceInvalid (409 for the checker).
func validateSourceAtApply(ctx context.Context, q vendorSourceQueries, atmID int64, p ATMAssignmentUpdatePayload, now time.Time) error {
	if p.source() != AssignmentSourceVendor {
		return nil
	}
	if err := checkVendorSource(ctx, q, atmID, p, assignmentAsOf(p, now)); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			return fmt.Errorf("%w: %s", ErrATMAssignmentSourceInvalid, ve.Message)
		}
		return err
	}
	return nil
}
