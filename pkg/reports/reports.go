// Package reports is the report-CSV contract shared by backend (screens) and backend-exq (async
// export): Report_Kind values, viewer roles, parameter validation and the cell format. Moved as-is
// from backend/internal/{handler,service}; the golden files in backend/internal/handler/testdata/report_csv
// pin the output.
package reports

import (
	"fmt"
	"time"
)

// Report_Kind values (export_jobs.kind).
const (
	DsrLateDaily                = "dsr_late_daily"
	DsrLateSummary              = "dsr_late_summary"
	ReplenishmentAmountMismatch = "replenishment_amount_mismatch"
	ReplenishmentTrips          = "replenishment_trips"

	ReplenishmentMaxRangeDays = 31 // K5: screen + export
	ReplenishmentExportLimit  = 100000
)

// Viewer roles of each report family; route guard and service check must share these.
var (
	DsrLateViewerRoles       = []string{"ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ADMIN"}
	ReplenishmentViewerRoles = []string{"ADMIN", "ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"}
)

// DsrLateStatuses are the status codes a filter may name.
var DsrLateStatuses = map[string]bool{"ok": true, "late": true, "not_sent": true, "pending": true}

// ReplenishmentClasses are the classification values a filter may name.
var ReplenishmentClasses = map[string]bool{
	"pending": true, "on_schedule": true, "early": true, "late": true, "not_done": true, "no_order": true, "void": true,
}

// WIB is fixed (no DST) so callers never depend on tzdata being installed.
var WIB = time.FixedZone("WIB", 7*60*60)

// ValidationError is a bad request parameter; callers map it to 400 with Field/Message.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

// ParseDsrLateStartDate parses DSR_LATE_START_DATE (YYYY-MM-DD); empty = feature off (nil).
func ParseDsrLateStartDate(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, fmt.Errorf("DSR_LATE_START_DATE must be YYYY-MM-DD: %w", err)
	}
	return &d, nil
}

// ParseDsrLateMonth parses month=YYYY-MM into the first day of that month.
func ParseDsrLateMonth(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, &ValidationError{Field: "month", Message: "month wajib diisi (YYYY-MM)"}
	}
	m, err := time.Parse("2006-01", raw)
	if err != nil {
		return time.Time{}, &ValidationError{Field: "month", Message: "format month YYYY-MM"}
	}
	return m, nil
}

// ValidateDsrLateMonth: the month must not be after the current month (WIB), FR4.1.
func ValidateDsrLateMonth(month, now time.Time) error {
	if month.IsZero() {
		return &ValidationError{Field: "month", Message: "month wajib diisi (YYYY-MM)"}
	}
	n := now.In(WIB)
	if month.After(time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)) {
		return &ValidationError{Field: "month", Message: "bulan tidak boleh setelah bulan berjalan"}
	}
	return nil
}

// ValidateReplenishmentRange: both dates set, from <= to, at most 31 days (K5).
func ValidateReplenishmentRange(from, to time.Time) error {
	if from.IsZero() || to.IsZero() {
		return &ValidationError{Field: "date_from", Message: "date_from dan date_to wajib diisi"}
	}
	if to.Before(from) {
		return &ValidationError{Field: "date_to", Message: "date_to tidak boleh sebelum date_from"}
	}
	if civilDate(to).Sub(civilDate(from)) > (ReplenishmentMaxRangeDays-1)*24*time.Hour {
		return &ValidationError{Field: "date_to", Message: fmt.Sprintf("rentang maksimal %d hari", ReplenishmentMaxRangeDays)}
	}
	return nil
}

func civilDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
