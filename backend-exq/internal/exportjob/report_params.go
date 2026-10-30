package exportjob

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cimb-niaga/cms/pkg/reports"
)

// Report kinds reuse the pkg/reports values (export_jobs.kind).
const (
	KindDsrLateDaily                Kind = reports.DsrLateDaily
	KindDsrLateSummary              Kind = reports.DsrLateSummary
	KindReplenishmentAmountMismatch Kind = reports.ReplenishmentAmountMismatch
	KindReplenishmentTrips          Kind = reports.ReplenishmentTrips
)

// ReportParams is the validated filter of one report export (stored in export_jobs.report_params).
// Fields that do not apply to the kind stay nil.
type ReportParams struct {
	Month          *string `json:"month,omitempty"`            // YYYY-MM (DSR)
	DateFrom       *string `json:"date_from,omitempty"`        // YYYY-MM-DD (replenishment)
	DateTo         *string `json:"date_to,omitempty"`          // YYYY-MM-DD (replenishment)
	VendorID       *int64  `json:"vendor_id,omitempty"`        // all report kinds
	VendorBranchID *int64  `json:"vendor_branch_id,omitempty"` // DSR
	RegionCode     *string `json:"region_code,omitempty"`      // replenishment
	Status         *string `json:"status,omitempty"`           // dsr_late_daily
	Classification *string `json:"classification,omitempty"`   // amount mismatch
}

// reportEntities maps each report kind to the legacy `report` value stored as export_jobs.entity.
var reportEntities = map[Kind]string{
	KindDsrLateDaily:                "daily",
	KindDsrLateSummary:              "summary",
	KindReplenishmentAmountMismatch: "amount_mismatch",
	KindReplenishmentTrips:          "trips",
}

// IsReportKind reports whether k is one of the four report kinds.
func IsReportKind(k Kind) bool { _, ok := reportEntities[k]; return ok }

// ParseReportParams validates a legacy-style query for a report kind with the pkg/reports
// validators, so field and message equal the legacy 400s. Parameters that do not apply to the
// kind are ignored. Errors wrap ErrValidation and a *reports.ValidationError (errors.As).
func ParseReportParams(kind Kind, q url.Values, now time.Time) (Params, error) {
	entity, ok := reportEntities[kind]
	if !ok {
		return Params{}, fmt.Errorf("%w: kind %q is not a report", ErrValidation, kind)
	}
	var r ReportParams
	var err error
	switch kind {
	case KindDsrLateDaily, KindDsrLateSummary:
		err = r.parseDsr(kind, q, now)
	default:
		err = r.parseReplenishment(kind, q)
	}
	if err != nil {
		return Params{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return Params{Kind: kind, Entity: entity, Report: &r}, nil
}

func (r *ReportParams) parseDsr(kind Kind, q url.Values, now time.Time) error {
	month, err := reports.ParseDsrLateMonth(q.Get("month"))
	if err != nil {
		return err
	}
	if err := reports.ValidateDsrLateMonth(month, now); err != nil {
		return err
	}
	m := month.Format("2006-01")
	r.Month = &m
	if r.VendorID, err = optionalID(q, "vendor_id"); err != nil {
		return err
	}
	if r.VendorBranchID, err = optionalID(q, "vendor_branch_id"); err != nil {
		return err
	}
	if s := q.Get("status"); kind == KindDsrLateDaily && s != "" {
		if !reports.DsrLateStatuses[s] {
			return &reports.ValidationError{Field: "status", Message: "tidak dikenal"}
		}
		r.Status = &s
	}
	return nil
}

func (r *ReportParams) parseReplenishment(kind Kind, q url.Values) error {
	var from, to time.Time
	for _, f := range []struct {
		field string
		t     *time.Time
		s     **string
	}{{"date_from", &from, &r.DateFrom}, {"date_to", &to, &r.DateTo}} {
		raw := q.Get(f.field)
		if raw == "" {
			continue
		}
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return &reports.ValidationError{Field: f.field, Message: "format tanggal YYYY-MM-DD"}
		}
		s := d.Format("2006-01-02")
		*f.t, *f.s = d, &s
	}
	if err := reports.ValidateReplenishmentRange(from, to); err != nil {
		return err
	}
	var err error
	if r.VendorID, err = optionalID(q, "vendor_id"); err != nil {
		return err
	}
	if rc := q.Get("region_code"); rc != "" {
		r.RegionCode = &rc
	}
	if c := q.Get("classification"); kind == KindReplenishmentAmountMismatch && c != "" {
		if !reports.ReplenishmentClasses[c] {
			return &reports.ValidationError{Field: "classification", Message: "tidak dikenal"}
		}
		r.Classification = &c
	}
	return nil
}

func optionalID(q url.Values, field string) (*int64, error) {
	raw := q.Get(field)
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, &reports.ValidationError{Field: field, Message: "tidak valid"}
	}
	return &id, nil
}

// reportHashInput is the canonical hash input: fixed field order, nil filters omitted, kind and
// entity included so it never equals a master-data hash or another kind's.
type reportHashInput struct {
	Kind   Kind         `json:"kind"`
	Entity string       `json:"entity"`
	Report ReportParams `json:"report"`
}

func reportHashBytes(p Params) []byte {
	// Marshal of plain strings/ints cannot fail.
	b, _ := json.Marshal(reportHashInput{Kind: p.Kind, Entity: p.Entity, Report: *p.Report})
	return b
}

// legacyFilter is the audit `filter` of a report job: the keys the legacy backend/ export wrote
// (absent = null), plus status (daily) and classification (amount mismatch), which it did not (Req 5.4).
func (r ReportParams) legacyFilter(kind Kind) map[string]any {
	switch kind {
	case KindDsrLateDaily, KindDsrLateSummary:
		m := map[string]any{"month": r.Month, "vendor_id": r.VendorID, "vendor_branch_id": r.VendorBranchID}
		if kind == KindDsrLateDaily {
			m["status"] = r.Status
		}
		return m
	}
	m := map[string]any{"date_from": r.DateFrom, "date_to": r.DateTo, "vendor_id": r.VendorID, "region_code": r.RegionCode}
	if kind == KindReplenishmentAmountMismatch {
		m["classification"] = r.Classification
	}
	return m
}

// reportFileName is the legacy file name of the report (pkg/reports.FileName).
func reportFileName(kind Kind, r ReportParams) string {
	day := func(s *string, layout string) time.Time {
		if s == nil {
			return time.Time{}
		}
		t, _ := time.Parse(layout, *s) // stored by ParseReportParams in this layout
		return t
	}
	return reports.FileName(string(kind), day(r.Month, "2006-01"), day(r.DateFrom, "2006-01-02"), day(r.DateTo, "2006-01-02"))
}

// reportParamsJSON is the export_jobs.report_params value; nil for master-data kinds.
func reportParamsJSON(r *ReportParams) []byte {
	if r == nil {
		return nil
	}
	b, _ := json.Marshal(r) // plain strings/ints cannot fail
	return b
}

// reportParamsFromJSON decodes export_jobs.report_params; NULL (master data) = nil.
func reportParamsFromJSON(b []byte) *ReportParams {
	if b == nil {
		return nil
	}
	var r ReportParams
	if err := json.Unmarshal(b, &r); err != nil {
		return nil // export_jobs_params_kind_chk guarantees an object; the report producer fails a job without params
	}
	return &r
}
