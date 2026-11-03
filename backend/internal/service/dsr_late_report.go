package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// DSR late report (.claude/sdlc/dsr-late-report/spec.md FR4, FR7): monthly late / missing DSR per
// vendor branch. Status is computed on read by the repository queries (replica, Sec 6). CSV export
// moved to backend-exq (report-export-migration).

const (
	dsrLateMaxPageSize = 200
	dsrLateDefaultPage = 50
)

// Roles, statuses, WIB and the month/start-date rules live in pkg/reports (shared with backend-exq,
// report-export-migration 3.1).

type DsrLateFilter struct {
	Month          time.Time // first day of the month
	VendorID       *int64
	VendorBranchID *int64
	Status         *string // daily only
	Page, PageSize int
}

type DsrLateDailyView struct {
	ReportDate      string     `json:"report_date"`
	VendorID        int64      `json:"vendor_id"`
	VendorCode      string     `json:"vendor_code"`
	VendorName      string     `json:"vendor_name"`
	VendorBranchID  int64      `json:"vendor_branch_id"`
	BranchCode      string     `json:"branch_code"`
	BranchName      string     `json:"branch_name"`
	RegionCode      *string    `json:"region_code"`
	Deadline        time.Time  `json:"deadline"`
	FirstReceivedAt *time.Time `json:"first_received_at"`
	ReceiptChecksum string     `json:"receipt_checksum"`
	MatchedLabels   []string   `json:"matched_locations"`
	Status          string     `json:"status"`
	LateMinutes     int32      `json:"late_minutes"`
}

type DsrLateDailyList struct {
	Items     []DsrLateDailyView `json:"items"`
	Total     int64              `json:"total"`
	Page      int                `json:"page"`
	PageSize  int                `json:"page_size"`
	StartDate *string            `json:"start_date"`
}

type DsrLateSummaryView struct {
	VendorID         int64   `json:"vendor_id"`
	VendorCode       string  `json:"vendor_code"`
	VendorName       string  `json:"vendor_name"`
	VendorBranchID   int64   `json:"vendor_branch_id"`
	BranchCode       string  `json:"branch_code"`
	BranchName       string  `json:"branch_name"`
	RegionCode       *string `json:"region_code"`
	ObligedDays      int32   `json:"obliged_days"`
	OkDays           int32   `json:"ok_days"`
	LateDays         int32   `json:"late_days"`
	NotSentDays      int32   `json:"not_sent_days"`
	PendingDays      int32   `json:"pending_days"`
	LateMinutesTotal int32   `json:"late_minutes_total"`
}

type DsrLateSummary struct {
	Items     []DsrLateSummaryView `json:"items"`
	StartDate *string              `json:"start_date"`
}

type DsrLateUnmappedVault struct {
	VendorID       int64  `json:"vendor_id"`
	VendorName     string `json:"vendor_name"`
	VendorBranchID int64  `json:"vendor_branch_id"`
	BranchCode     string `json:"branch_code"`
	BranchName     string `json:"branch_name"`
	VendorVaultID  int64  `json:"vendor_vault_id"`
	VaultCode      string `json:"vault_code"`
	BranchExcluded bool   `json:"branch_excluded"`
}

type DsrLateUnmappedLabel struct {
	VendorID        int64   `json:"vendor_id"`
	VendorName      string  `json:"vendor_name"`
	DsrLocation     string  `json:"dsr_location"`
	ReceiptCount    int32   `json:"receipt_count"`
	FirstReportDate *string `json:"first_report_date"`
	LastReportDate  *string `json:"last_report_date"`
}

type DsrLateUnresolvedVendor struct {
	VendorLabel     string  `json:"vendor_label"`
	ReceiptCount    int32   `json:"receipt_count"`
	FirstReportDate *string `json:"first_report_date"`
	LastReportDate  *string `json:"last_report_date"`
}

type DsrLateWarnings struct {
	UnmappedVaults    []DsrLateUnmappedVault    `json:"unmapped_vaults"`
	UnmappedLabels    []DsrLateUnmappedLabel    `json:"unmapped_labels"`
	UnresolvedVendors []DsrLateUnresolvedVendor `json:"unresolved_vendors"`
	StartDate         *string                   `json:"start_date"`
}

// DsrLateReportService reads the report; startDate nil (DSR_LATE_START_DATE unset) = always empty.
type DsrLateReportService struct {
	repo      *repository.DsrLateReportRepository
	startDate *time.Time
	now       func() time.Time
}

// NewDsrLateReportService: read-only, all queries on readPool (replica).
func NewDsrLateReportService(readPool db.DBTX, startDate *time.Time) *DsrLateReportService {
	return &DsrLateReportService{
		repo: repository.NewDsrLateReportRepository(readPool), startDate: startDate, now: time.Now,
	}
}

// ParseDsrLateMonth parses month=YYYY-MM into the first day of that month (reports.ParseDsrLateMonth).
func ParseDsrLateMonth(raw string) (time.Time, error) {
	m, err := reports.ParseDsrLateMonth(raw)
	return m, reportErr(err)
}

func (s *DsrLateReportService) authorize(actor Actor) error {
	if !roleIn(actor.Role, reports.DsrLateViewerRoles) {
		return ErrNotAuthorized
	}
	return nil
}

// validateMonth: the month must not be after the current month (WIB), FR4.1.
func (s *DsrLateReportService) validateMonth(month time.Time) error {
	return reportErr(reports.ValidateDsrLateMonth(month, s.now()))
}

// reportErr maps a pkg/reports validation error onto ValidationError, which the handlers turn into 400.
func reportErr(err error) error {
	var ve *reports.ValidationError
	if errors.As(err, &ve) {
		return &ValidationError{Field: ve.Field, Message: ve.Message}
	}
	return err
}

// prepare = authorize + validate; returns the shared query window. ok=false means "no start date: empty".
func (s *DsrLateReportService) prepare(actor Actor, month time.Time) (monthStart, startDate pgtype.Date, asOf pgtype.Timestamptz, ok bool, err error) {
	if err = s.authorize(actor); err != nil {
		return
	}
	if err = s.validateMonth(month); err != nil {
		return
	}
	if s.startDate == nil {
		return
	}
	return toPgDate(month), toPgDate(*s.startDate), pgtype.Timestamptz{Time: s.now(), Valid: true}, true, nil
}

func (s *DsrLateReportService) startDateText() *string {
	if s.startDate == nil {
		return nil
	}
	t := s.startDate.Format("2006-01-02")
	return &t
}

// Daily: GET /daily (FR4.2), paged.
func (s *DsrLateReportService) Daily(ctx context.Context, actor Actor, f DsrLateFilter) (*DsrLateDailyList, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = dsrLateDefaultPage
	}
	if f.PageSize > dsrLateMaxPageSize {
		f.PageSize = dsrLateMaxPageSize
	}
	limit := int64(f.PageSize)
	items, total, err := s.daily(ctx, actor, f, limit, int64((f.Page-1)*f.PageSize))
	if err != nil {
		return nil, err
	}
	return &DsrLateDailyList{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize, StartDate: s.startDateText()}, nil
}

func (s *DsrLateReportService) daily(ctx context.Context, actor Actor, f DsrLateFilter, limit, offset int64) ([]DsrLateDailyView, int64, error) {
	monthStart, startDate, asOf, ok, err := s.prepare(actor, f.Month)
	if err != nil {
		return nil, 0, err
	}
	if f.Status != nil && !reports.DsrLateStatuses[*f.Status] {
		return nil, 0, &ValidationError{Field: "status", Message: "tidak dikenal"}
	}
	if !ok {
		return []DsrLateDailyView{}, 0, nil
	}
	rows, err := s.repo.ListDaily(ctx, db.ListDsrLateDailyParams{
		VendorID: f.VendorID, VendorBranchID: f.VendorBranchID, Status: f.Status,
		PageOffset: offset, PageLimit: &limit, MonthStart: monthStart, StartDate: startDate, AsOf: asOf,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list dsr late daily: %w", err)
	}
	total, err := s.repo.CountDaily(ctx, db.CountDsrLateDailyParams{
		VendorID: f.VendorID, VendorBranchID: f.VendorBranchID, Status: f.Status,
		MonthStart: monthStart, StartDate: startDate, AsOf: asOf,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count dsr late daily: %w", err)
	}
	out := make([]DsrLateDailyView, len(rows))
	for i, r := range rows {
		labels := r.MatchedLocations
		if labels == nil {
			labels = []string{}
		}
		out[i] = DsrLateDailyView{
			ReportDate: r.ReportDate.Time.Format("2006-01-02"), VendorID: r.VendorID, VendorCode: r.VendorCode,
			VendorName: r.VendorName, VendorBranchID: r.VendorBranchID, BranchCode: r.BranchCode, BranchName: r.BranchName,
			RegionCode: r.RegionCode, Deadline: r.Deadline.Time.In(reports.WIB), FirstReceivedAt: inWIB(r.FirstReceivedAt),
			ReceiptChecksum: r.ReceiptChecksum, MatchedLabels: labels, Status: r.Status, LateMinutes: r.LateMinutes,
		}
	}
	return out, total, nil
}

func inWIB(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time.In(reports.WIB)
	return &t
}

func dateText(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	t := d.Time.Format("2006-01-02")
	return &t
}

// Summary: GET /summary (FR4.3).
func (s *DsrLateReportService) Summary(ctx context.Context, actor Actor, f DsrLateFilter) (*DsrLateSummary, error) {
	monthStart, startDate, asOf, ok, err := s.prepare(actor, f.Month)
	if err != nil {
		return nil, err
	}
	out := &DsrLateSummary{Items: []DsrLateSummaryView{}, StartDate: s.startDateText()}
	if !ok {
		return out, nil
	}
	rows, err := s.repo.ListSummary(ctx, db.ListDsrLateSummaryParams{
		VendorID: f.VendorID, VendorBranchID: f.VendorBranchID, MonthStart: monthStart, StartDate: startDate, AsOf: asOf,
	})
	if err != nil {
		return nil, fmt.Errorf("list dsr late summary: %w", err)
	}
	for _, r := range rows {
		out.Items = append(out.Items, DsrLateSummaryView(r))
	}
	return out, nil
}

// Warnings: GET /warnings (FR3.4).
func (s *DsrLateReportService) Warnings(ctx context.Context, actor Actor, f DsrLateFilter) (*DsrLateWarnings, error) {
	monthStart, startDate, asOf, ok, err := s.prepare(actor, f.Month)
	if err != nil {
		return nil, err
	}
	out := &DsrLateWarnings{
		UnmappedVaults: []DsrLateUnmappedVault{}, UnmappedLabels: []DsrLateUnmappedLabel{},
		UnresolvedVendors: []DsrLateUnresolvedVendor{}, StartDate: s.startDateText(),
	}
	if !ok {
		return out, nil
	}
	vaults, err := s.repo.ListUnmappedVaults(ctx, db.ListDsrLateUnmappedVaultsParams{
		VendorID: f.VendorID, MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		return nil, fmt.Errorf("list unmapped vaults: %w", err)
	}
	labels, err := s.repo.ListUnmappedLabels(ctx, db.ListDsrLateUnmappedLabelsParams{
		VendorID: f.VendorID, MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		return nil, fmt.Errorf("list unmapped labels: %w", err)
	}
	vendors, err := s.repo.ListUnresolvedVendors(ctx, db.ListDsrLateUnresolvedVendorsParams{
		MonthStart: monthStart, StartDate: startDate, AsOf: asOf})
	if err != nil {
		return nil, fmt.Errorf("list unresolved vendors: %w", err)
	}
	for _, v := range vaults {
		out.UnmappedVaults = append(out.UnmappedVaults, DsrLateUnmappedVault(v))
	}
	for _, l := range labels {
		out.UnmappedLabels = append(out.UnmappedLabels, DsrLateUnmappedLabel{
			VendorID: l.VendorID, VendorName: l.VendorName, DsrLocation: l.DsrLocation, ReceiptCount: l.ReceiptCount,
			FirstReportDate: dateText(l.FirstReportDate), LastReportDate: dateText(l.LastReportDate)})
	}
	for _, v := range vendors {
		out.UnresolvedVendors = append(out.UnresolvedVendors, DsrLateUnresolvedVendor{
			VendorLabel: v.VendorLabel, ReceiptCount: v.ReceiptCount,
			FirstReportDate: dateText(v.FirstReportDate), LastReportDate: dateText(v.LastReportDate)})
	}
	return out, nil
}
