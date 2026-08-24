package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ValidationError represents a structured field-level validation error.
// Mirrors auth.ValidationError's shape so handlers can format both the
// same way.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ListATMsParams holds the validated request parameters for listing ATMs.
// validate tags document the intended rules (enforced by hand in validate()
// below, matching this codebase's existing convention — see
// auth.LoginRequest / auth.Service.validateInput). Callers (the HTTP
// handler) are responsible for applying API-contract defaults — page=1,
// page_size=25, status="all", sort_by="terminal_id", sort_order="asc" —
// before calling ListATMs; this struct does not default zero values itself.
type ListATMsParams struct {
	Page           int    `json:"page" validate:"gte=1"`
	PageSize       int    `json:"page_size" validate:"gte=1,lte=100"`
	Search         string `json:"search" validate:"max=100"`
	Status         string `json:"status" validate:"omitempty,oneof=all low critical normal unconfigured no_data"`
	MachineType    string `json:"machine_type"`
	Brand          string `json:"brand"`
	DeploymentType string `json:"deployment_type"`
	Region         string `json:"region"`
	DateFrom       string `json:"date_from"`
	DateTo         string `json:"date_to"`
	SortBy         string `json:"sort_by" validate:"omitempty,oneof=terminal_id location last_replenish_date refund_total replenish_total status"`
	SortOrder      string `json:"sort_order" validate:"omitempty,oneof=asc desc"`
}

var allowedSortBy = []string{
	"terminal_id", "location", "last_replenish_date", "refund_total", "replenish_total", "status",
}

// validate checks ListATMsParams against the rules documented in its
// validate tags, returning a *ValidationError for the first failing field.
func (p ListATMsParams) validate() error {
	if p.Page < 1 {
		return &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		return &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	if len(p.Search) > 100 {
		return &ValidationError{Field: "search", Message: "maksimal 100 karakter"}
	}
	if err := validateStatus(p.Status); err != nil {
		return err
	}
	if err := validateDateBound("date_from", p.DateFrom); err != nil {
		return err
	}
	if err := validateDateBound("date_to", p.DateTo); err != nil {
		return err
	}
	if p.DateFrom != "" && p.DateTo != "" && p.DateFrom > p.DateTo {
		return &ValidationError{Field: "date_from", Message: "tidak boleh lebih besar dari date_to"}
	}
	if p.SortBy != "" && !slices.Contains(allowedSortBy, p.SortBy) {
		return &ValidationError{Field: "sort_by", Message: "harus salah satu dari: terminal_id, location, last_replenish_date, refund_total, replenish_total, status"}
	}
	if p.SortOrder != "" && !slices.Contains([]string{"asc", "desc"}, p.SortOrder) {
		return &ValidationError{Field: "sort_order", Message: "harus salah satu dari: asc, desc"}
	}
	return nil
}

// validateDateBound checks an optional YYYY-MM-DD date query param.
func validateDateBound(field, value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return &ValidationError{Field: field, Message: "harus berformat YYYY-MM-DD"}
	}
	return nil
}

// validateStatus checks status against the same rules as the other oneof
// fields, except status is comma-separated multi-select (requirements.md
// Req 89.3, same pattern as MachineType/Brand): "" or "all" means no
// filter, otherwise every comma-separated segment must be one of the five
// real status values.
func validateStatus(status string) error {
	if status == "" || status == "all" {
		return nil
	}
	allowed := []string{"low", "critical", "normal", "unconfigured", "no_data"}
	for _, s := range strings.Split(status, ",") {
		if !slices.Contains(allowed, s) {
			return &ValidationError{Field: "status", Message: "harus salah satu dari: all, low, critical, normal, unconfigured, no_data"}
		}
	}
	return nil
}

// AtmWithCashPos is a single ATM row joined with its latest itm_replenish
// record. TerminalID/LocationName/Address/MachineType/Brand/DeploymentType
// and Status are plain strings (not pointers) because the underlying
// atms/locations columns they come from are NOT NULL — verified live
// against the cms database (see tasks.md Task 1.1). The cashpos-derived
// fields are pointers because a LEFT JOIN LATERAL leaves them NULL when a
// terminal has no itm_replenish record yet (status "no_data").
type AtmWithCashPos struct {
	TerminalID              string
	LocationName            string
	Address                 string
	MachineType             string
	Brand                   string
	DeploymentType          string
	LowThresholdAmount      *float64
	CriticalThresholdAmount *float64
	LastReplenishDate       *time.Time
	LastReplenishTime       *string
	RefundTotal             *float64
	ReplenishTotal          *float64
	Escrow                  *float64
	Status                  string
}

// ATMSummary holds global replenishment-status counts, independent of any
// list filters (Property 10).
type ATMSummary struct {
	Total        int64
	Critical     int64
	Low          int64
	Normal       int64
	Unconfigured int64
	NoData       int64
}

// ListATMsResult is the full result of a ListATMs call.
type ListATMsResult struct {
	Data        []AtmWithCashPos
	Summary     ATMSummary
	Total       int64
	Page        int
	PageSize    int
	LastUpdated *time.Time
}

// AtmPortalRepository is the small, service-defined interface the
// sqlc-generated *db.Queries satisfies. Defined here (where it's used),
// per Go convention, so the service can be tested against a fake.
// Cashpos methods live on AtmPortalCashposRepository; *db.Queries
// implements both and is stored here as AtmPortalRepository (ListATMs
// path). ListCashpos type-asserts to AtmPortalCashposRepository.
type AtmPortalRepository interface {
	ListATMsWithCashPos(ctx context.Context, arg db.ListATMsWithCashPosParams) ([]db.ListATMsWithCashPosRow, error)
	CountATMsWithCashPos(ctx context.Context, arg db.CountATMsWithCashPosParams) (int64, error)
	GetATMSummary(ctx context.Context) (db.GetATMSummaryRow, error)
	GetLastUpdated(ctx context.Context) (pgtype.Timestamp, error)
}

// AtmPortalServicer is the interface AtmPortalHandler depends on.
type AtmPortalServicer interface {
	ListATMs(ctx context.Context, params ListATMsParams) (*ListATMsResult, error)
	ListCashpos(ctx context.Context, params ListCashposParams) (*ListCashposResult, error)
}

// AtmPortalService implements AtmPortalServicer.
type AtmPortalService struct {
	repo AtmPortalRepository
}

// NewAtmPortalService creates a new AtmPortalService wrapping the given repository.
func NewAtmPortalService(repo AtmPortalRepository) *AtmPortalService {
	return &AtmPortalService{repo: repo}
}

// ListATMs validates params, delegates to the repository, and assembles
// the paginated list, global summary, total count, and data-freshness
// timestamp into a single result.
func (s *AtmPortalService) ListATMs(ctx context.Context, params ListATMsParams) (*ListATMsResult, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	listArg := db.ListATMsWithCashPosParams{
		Search:         params.Search,
		Status:         params.Status,
		MachineType:    params.MachineType,
		Brand:          params.Brand,
		DeploymentType: params.DeploymentType,
		Region:         params.Region,
		DateFrom:       params.DateFrom,
		DateTo:         params.DateTo,
		SortBy:         params.SortBy,
		SortOrder:      params.SortOrder,
		Page:           int32(params.Page),
		PageSize:       int32(params.PageSize),
	}
	rows, err := s.repo.ListATMsWithCashPos(ctx, listArg)
	if err != nil {
		return nil, fmt.Errorf("listing atms: %w", err)
	}

	countArg := db.CountATMsWithCashPosParams{
		Search:         params.Search,
		Status:         params.Status,
		MachineType:    params.MachineType,
		Brand:          params.Brand,
		DeploymentType: params.DeploymentType,
		Region:         params.Region,
		DateFrom:       params.DateFrom,
		DateTo:         params.DateTo,
	}
	total, err := s.repo.CountATMsWithCashPos(ctx, countArg)
	if err != nil {
		return nil, fmt.Errorf("counting atms: %w", err)
	}

	summaryRow, err := s.repo.GetATMSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting atm summary: %w", err)
	}

	// GetLastUpdated is a `:one` query with no matching row when
	// itm_replenish is empty — that's pgx.ErrNoRows, not a NULL value inside
	// a row, so it needs its own not-found branch rather than a Valid
	// check (see design.md's Error Handling table: "No itm_replenish
	// records globally" -> "last_updated: null", not a 500).
	var lastUpdated *time.Time
	lastUpdatedRow, err := s.repo.GetLastUpdated(ctx)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("getting last updated: %w", err)
		}
	} else {
		lastUpdated = timestampToTimePtr(lastUpdatedRow)
	}

	data := make([]AtmWithCashPos, len(rows))
	for i, row := range rows {
		atm, err := rowToAtmWithCashPos(row)
		if err != nil {
			return nil, fmt.Errorf("converting atm row %q: %w", row.TerminalID, err)
		}
		data[i] = atm
	}

	return &ListATMsResult{
		Data:  data,
		Total: total,
		Summary: ATMSummary{
			Total:        summaryRow.Total,
			Critical:     summaryRow.Critical,
			Low:          summaryRow.Low,
			Normal:       summaryRow.Normal,
			Unconfigured: summaryRow.Unconfigured,
			NoData:       summaryRow.NoData,
		},
		Page:        params.Page,
		PageSize:    params.PageSize,
		LastUpdated: lastUpdated,
	}, nil
}

// rowToAtmWithCashPos converts a generated sqlc row into the service's
// public AtmWithCashPos, unwrapping pgtype nullable values into plain Go
// pointer types.
func rowToAtmWithCashPos(row db.ListATMsWithCashPosRow) (AtmWithCashPos, error) {
	lowThreshold, err := numericToFloat64Ptr(row.LowThresholdAmount)
	if err != nil {
		return AtmWithCashPos{}, fmt.Errorf("low_threshold_amount: %w", err)
	}
	criticalThreshold, err := numericToFloat64Ptr(row.CriticalThresholdAmount)
	if err != nil {
		return AtmWithCashPos{}, fmt.Errorf("critical_threshold_amount: %w", err)
	}
	refundTotal, err := numericToFloat64Ptr(row.RefundTotal)
	if err != nil {
		return AtmWithCashPos{}, fmt.Errorf("refund_total: %w", err)
	}
	replenishTotal, err := numericToFloat64Ptr(row.ReplenishTotal)
	if err != nil {
		return AtmWithCashPos{}, fmt.Errorf("replenish_total: %w", err)
	}
	escrow, err := numericToFloat64Ptr(row.Escrow)
	if err != nil {
		return AtmWithCashPos{}, fmt.Errorf("escrow: %w", err)
	}

	return AtmWithCashPos{
		TerminalID:              row.TerminalID,
		LocationName:            row.LocationName,
		Address:                 row.Address,
		MachineType:             row.MachineType,
		Brand:                   row.Brand,
		DeploymentType:          row.DeploymentType,
		LowThresholdAmount:      lowThreshold,
		CriticalThresholdAmount: criticalThreshold,
		LastReplenishDate:       dateToTimePtr(row.LastReplenishDate),
		LastReplenishTime:       pgTimeToStringPtr(row.LastReplenishTime),
		RefundTotal:             refundTotal,
		ReplenishTotal:          replenishTotal,
		Escrow:                  escrow,
		Status:                  row.Status,
	}, nil
}

// numericToFloat64Ptr converts a pgtype.Numeric into a *float64, returning
// nil for SQL NULL.
func numericToFloat64Ptr(n pgtype.Numeric) (*float64, error) {
	if !n.Valid {
		return nil, nil
	}
	f, err := n.Float64Value()
	if err != nil {
		return nil, fmt.Errorf("converting numeric to float64: %w", err)
	}
	v := f.Float64
	return &v, nil
}

// dateToTimePtr converts a pgtype.Date into a *time.Time, returning nil for
// SQL NULL.
func dateToTimePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

// pgTimeToStringPtr converts a pgtype.Time (microseconds since midnight)
// into a "HH:MM:SS" *string, returning nil for SQL NULL.
func pgTimeToStringPtr(t pgtype.Time) *string {
	if !t.Valid {
		return nil
	}
	formatted := time.Time{}.Add(time.Duration(t.Microseconds) * time.Microsecond).Format("15:04:05")
	return &formatted
}

// timestampToTimePtr converts a pgtype.Timestamp into a *time.Time,
// returning nil for SQL NULL.
func timestampToTimePtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
