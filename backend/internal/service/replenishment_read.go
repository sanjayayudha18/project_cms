package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Replenishment read side (.claude/sdlc/replenishment-realisasi/spec.md FR4, FR6): realisasi vs order
// classifications written by the Python job replenish_classify. Reads on the replica (Sec 6).
// Roles = the Vendor Request viewers (FR4.3); roles, classes, range and export limit live in
// pkg/reports (report-export-migration 3.1). CSV export moved to backend-exq.

const replenishmentMaxPageSize = 100

type ReplenishmentFilter struct {
	From, To       time.Time
	VendorID       *int64
	VendorBranchID *int64 // VendorPortal scope only
	RegionCode     *string
	Classification *string
	OnlyDiff       bool
	FinalOnly      bool
	Page, PageSize int
}

type ClassificationAmount struct {
	Denom  int32  `json:"denom"`
	Order  int64  `json:"order_amount"`
	Itm    int64  `json:"itm_amount"`
	Vendor *int64 `json:"vendor_amount"`
	Diff   int64  `json:"diff"`
}

type ClassificationView struct {
	ID               int64                  `json:"id"`
	TicketNumber     *string                `json:"ticket_number"`
	RequestID        *int64                 `json:"request_id"`
	RequestNumber    *string                `json:"request_number"`
	TerminalID       string                 `json:"terminal_id"`
	VendorID         *int64                 `json:"vendor_id"`
	VendorName       *string                `json:"vendor_name"`
	BranchCode       *string                `json:"branch_code"`
	BranchName       *string                `json:"branch_name"`
	RegionCode       *string                `json:"region_code"`
	ScheduledDate    *time.Time             `json:"scheduled_date"`
	RealizedDate     *time.Time             `json:"realized_date"`
	RealizedTime     *string                `json:"realized_time"`
	WorkdayOffset    *int16                 `json:"workday_offset"`
	Classification   string                 `json:"classification"`
	IsFinal          bool                   `json:"is_final"`
	HasAmountDiff    bool                   `json:"has_amount_diff"`
	VendorMismatch   bool                   `json:"vendor_mismatch"`
	VendorReportLate bool                   `json:"vendor_report_late"`
	ClassifiedAt     *time.Time             `json:"classified_at"`
	Amounts          []ClassificationAmount `json:"amounts"`
	Currency         string                 `json:"currency"`
}

type ClassificationList struct {
	Items    []ClassificationView `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type ClassCount struct {
	Classification   string `json:"classification"`
	Total            int64  `json:"total"`
	AmountDiff       int64  `json:"amount_diff"`
	VendorMismatch   int64  `json:"vendor_mismatch"`
	VendorReportLate int64  `json:"vendor_report_late"`
}

type DailyInstruction struct {
	Date        *time.Time `json:"date"`
	Terminals   int64      `json:"terminals"`
	OrderAmount int64      `json:"order_amount"`
}

type ReplenishmentSummary struct {
	ByClass      []ClassCount       `json:"by_class"`
	Instructions []DailyInstruction `json:"instructions"`
	Currency     string             `json:"currency"`
}

type TripRow struct {
	VendorID   *int64     `json:"vendor_id"`
	VendorName *string    `json:"vendor_name"`
	TerminalID string     `json:"terminal_id"`
	Trips      int64      `json:"trips"`
	FirstDate  *time.Time `json:"first_date"`
	LastDate   *time.Time `json:"last_date"`
}

type ReplenishmentReadService struct {
	read *db.Queries
}

// NewReplenishmentReadService: read-only, all queries on readPool (replica).
func NewReplenishmentReadService(readPool *pgxpool.Pool) *ReplenishmentReadService {
	return &ReplenishmentReadService{read: db.New(readPool)}
}

// validateReplenishmentRange: both dates set, from <= to, at most 31 days (K5).
func validateReplenishmentRange(from, to time.Time) error {
	return reportErr(reports.ValidateReplenishmentRange(from, to))
}

func (s *ReplenishmentReadService) authorize(actor Actor) error {
	if !roleIn(actor.Role, reports.ReplenishmentViewerRoles) {
		return ErrNotAuthorized
	}
	return nil
}

// List: GET /api/v1/replenishment/classifications (FR4.1).
func (s *ReplenishmentReadService) List(ctx context.Context, actor Actor, f ReplenishmentFilter) (*ClassificationList, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}
	return listClassifications(ctx, s.read, f)
}

// listClassifications is shared with the VendorPortal page (FR5.2), which sets the vendor scope.
func listClassifications(ctx context.Context, q *db.Queries, f ReplenishmentFilter) (*ClassificationList, error) {
	if err := validateReplenishmentRange(f.From, f.To); err != nil {
		return nil, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > replenishmentMaxPageSize {
		f.PageSize = 20
	}
	items, err := classificationViews(ctx, q, f, int64(f.PageSize), int64((f.Page-1)*f.PageSize))
	if err != nil {
		return nil, err
	}
	total, err := q.CountReplenishClassifications(ctx, db.CountReplenishClassificationsParams{
		DateFrom: toPgDate(f.From), DateTo: toPgDate(f.To), VendorID: f.VendorID, VendorBranchID: f.VendorBranchID,
		RegionCode: f.RegionCode, Classification: f.Classification, OnlyDiff: f.OnlyDiff, FinalOnly: f.FinalOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("count classifications: %w", err)
	}
	return &ClassificationList{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func classificationViews(ctx context.Context, q *db.Queries, f ReplenishmentFilter, limit, offset int64) ([]ClassificationView, error) {
	rows, err := q.ListReplenishClassifications(ctx, db.ListReplenishClassificationsParams{
		DateFrom: toPgDate(f.From), DateTo: toPgDate(f.To), VendorID: f.VendorID, VendorBranchID: f.VendorBranchID,
		RegionCode: f.RegionCode, Classification: f.Classification, OnlyDiff: f.OnlyDiff, FinalOnly: f.FinalOnly,
		PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list classifications: %w", err)
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	amountRows, err := q.ListReplenishClassificationAmounts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list classification amounts: %w", err)
	}
	amounts := make(map[int64][]ClassificationAmount, len(rows))
	for _, a := range amountRows {
		amounts[a.ClassificationID] = append(amounts[a.ClassificationID], ClassificationAmount{
			Denom: a.Denom, Order: a.OrderAmount, Itm: a.ItmAmount, Vendor: a.VendorAmount, Diff: derefInt64(a.Diff),
		})
	}
	out := make([]ClassificationView, len(rows))
	for i, r := range rows {
		var realizedTime *string
		if r.RealizedTime.Valid {
			t := time.UnixMicro(r.RealizedTime.Microseconds).UTC().Format("15:04:05")
			realizedTime = &t
		}
		am := amounts[r.ID]
		if am == nil {
			am = []ClassificationAmount{}
		}
		out[i] = ClassificationView{
			ID: r.ID, TicketNumber: r.TicketNumber, RequestID: r.RequestID, RequestNumber: r.RequestNumber,
			TerminalID: r.TerminalID, VendorID: r.VendorID, VendorName: r.VendorName, BranchCode: r.BranchCode,
			BranchName: r.BranchName, RegionCode: r.RegionCode, ScheduledDate: dateToPtr(r.ScheduledDate),
			RealizedDate: dateToPtr(r.RealizedDate), RealizedTime: realizedTime, WorkdayOffset: r.WorkdayOffset,
			Classification: r.Classification, IsFinal: r.IsFinal, HasAmountDiff: r.HasAmountDiff,
			VendorMismatch: r.VendorMismatch, VendorReportLate: r.VendorReportLate,
			ClassifiedAt: timestamptzToPtr(r.ClassifiedAt), Amounts: am, Currency: currencyIDR,
		}
	}
	return out, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// Summary: GET /api/v1/replenishment/summary (FR4.2 + FNC 003).
func (s *ReplenishmentReadService) Summary(ctx context.Context, actor Actor, from, to time.Time, vendorID *int64) (*ReplenishmentSummary, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}
	if err := validateReplenishmentRange(from, to); err != nil {
		return nil, err
	}
	classes, err := s.read.SummarizeReplenishClassifications(ctx, db.SummarizeReplenishClassificationsParams{
		DateFrom: toPgDate(from), DateTo: toPgDate(to), VendorID: vendorID})
	if err != nil {
		return nil, fmt.Errorf("summarize classifications: %w", err)
	}
	daily, err := s.read.SummarizeReplenishInstructions(ctx, db.SummarizeReplenishInstructionsParams{
		DateFrom: toPgDate(from), DateTo: toPgDate(to), VendorID: vendorID})
	if err != nil {
		return nil, fmt.Errorf("summarize instructions: %w", err)
	}
	out := &ReplenishmentSummary{ByClass: make([]ClassCount, len(classes)), Instructions: make([]DailyInstruction, len(daily)), Currency: currencyIDR}
	for i, c := range classes {
		out.ByClass[i] = ClassCount(c)
	}
	for i, d := range daily {
		out.Instructions[i] = DailyInstruction{Date: dateToPtr(d.ReplenishDate), Terminals: d.Terminals, OrderAmount: d.OrderAmount}
	}
	return out, nil
}

// AmountMismatch: report "amount != order" (FR6.1) -- final rows with a selisih.
func (s *ReplenishmentReadService) AmountMismatch(ctx context.Context, actor Actor, f ReplenishmentFilter) ([]ClassificationView, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}
	if err := validateReplenishmentRange(f.From, f.To); err != nil {
		return nil, err
	}
	f.OnlyDiff, f.FinalOnly = true, true
	return classificationViews(ctx, s.read, f, reports.ReplenishmentExportLimit, 0)
}

// Trips: report trip / realisasi per vendor per ATM (FR6.2).
func (s *ReplenishmentReadService) Trips(ctx context.Context, actor Actor, from, to time.Time, vendorID *int64) ([]TripRow, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}
	if err := validateReplenishmentRange(from, to); err != nil {
		return nil, err
	}
	rows, err := s.read.ReportReplenishTrips(ctx, db.ReportReplenishTripsParams{DateFrom: toPgDate(from), DateTo: toPgDate(to), VendorID: vendorID})
	if err != nil {
		return nil, fmt.Errorf("report trips: %w", err)
	}
	out := make([]TripRow, len(rows))
	for i, r := range rows {
		out[i] = TripRow{VendorID: r.VendorID, VendorName: r.VendorName, TerminalID: r.TerminalID, Trips: r.Trips,
			FirstDate: dateToPtr(r.FirstDate), LastDate: dateToPtr(r.LastDate)}
	}
	return out, nil
}

// ListRealizations: GET /api/v1/vendor/replenish-realizations (FR5.2) -- the vendor's own
// classifications only, narrowed to its branch when the user is pinned to one.
func (s *VendorOrderService) ListRealizations(ctx context.Context, va VendorActor, f ReplenishmentFilter) (*ClassificationList, error) {
	sc, err := s.scope(ctx, db.New(s.pool), va)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(sc.partyRoles, partyRoleReplenish) {
		return nil, ErrNotAuthorized // vendor-party-scope Req 4.2: a vault-only account has no realisasi
	}
	f.VendorID, f.VendorBranchID, f.RegionCode = &sc.vendorID, sc.branchID, nil
	return listClassifications(ctx, s.read, f)
}
