package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// --- Reads (no transaction needed) --------------------------------------

// BrowseForecast lists dmaa_atm_forecast rows for one forecast_date (Req 3).
// Brand/FLMVendor/FLMVendorRegion are optional filters, each length-bound to
// <=255 chars (CIT-2 Req 1.14). forecast-browser-summary (FR2) dropped the
// CIT-2 Req 1.4 "vendor + region required" rule so the page can show every
// ATM; the one-vendor-per-request rule is still enforced at Create
// (GetActiveVendorForTerminal). Unassigned is mutually exclusive with a
// vendor/region filter (it selects rows whose vendor is NULL); NoRegion (review
// R1) selects vendor rows whose branch region is NULL, so it excludes a region.
func (s *VendorRequestService) BrowseForecast(ctx context.Context, params BrowseForecastParams) (*BrowseForecastResult, error) {
	if params.ForecastDate == "" {
		return nil, &ValidationError{Field: "forecast_date", Message: "wajib diisi"}
	}
	if err := validateDateBound("forecast_date", params.ForecastDate); err != nil {
		return nil, err
	}
	if params.Page < 1 {
		return nil, &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		return nil, &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	if params.Unassigned && (params.FLMVendor != "" || params.FLMVendorRegion != "" || params.NoRegion) {
		return nil, &ValidationError{Field: "unassigned", Message: "tidak bisa digabung dengan filter vendor/region"}
	}
	if params.NoRegion && params.FLMVendorRegion != "" {
		return nil, &ValidationError{Field: "no_region", Message: "tidak bisa digabung dengan filter region"}
	}
	if len(params.Brand) > 255 {
		return nil, &ValidationError{Field: "brand", Message: "maksimal 255 karakter"}
	}
	if len(params.FLMVendor) > 255 {
		return nil, &ValidationError{Field: "flm_vendor", Message: "maksimal 255 karakter"}
	}
	if len(params.FLMVendorRegion) > 255 {
		return nil, &ValidationError{Field: "flm_vendor_region", Message: "maksimal 255 karakter"}
	}

	forecastDate, err := time.Parse("2006-01-02", params.ForecastDate)
	if err != nil {
		return nil, &ValidationError{Field: "forecast_date", Message: "harus berformat YYYY-MM-DD"}
	}

	rows, err := s.read.ListForecastForDate(ctx, db.ListForecastForDateParams{
		ForecastDate:    toPgDate(forecastDate),
		TerminalID:      params.TerminalID,
		Brand:           params.Brand,
		FlmVendor:       params.FLMVendor,
		FlmVendorRegion: params.FLMVendorRegion,
		Unassigned:      params.Unassigned,
		NoRegion:        params.NoRegion,
		Page:            int32(params.Page),     //nolint:gosec // G115: page <= maxPage, page_size <= 100 (handler)
		PageSize:        int32(params.PageSize), //nolint:gosec // G115: page <= maxPage, page_size <= 100 (handler)
	})
	if err != nil {
		return nil, fmt.Errorf("listing forecast for %s: %w", params.ForecastDate, err)
	}
	total, err := s.read.CountForecastForDate(ctx, db.CountForecastForDateParams{
		ForecastDate:    toPgDate(forecastDate),
		TerminalID:      params.TerminalID,
		Brand:           params.Brand,
		FlmVendor:       params.FLMVendor,
		FlmVendorRegion: params.FLMVendorRegion,
		Unassigned:      params.Unassigned,
		NoRegion:        params.NoRegion,
	})
	if err != nil {
		return nil, fmt.Errorf("counting forecast for %s: %w", params.ForecastDate, err)
	}

	data := make([]ForecastRow, len(rows))
	for i, r := range rows {
		// Req 5.10/5.11: escrow travels as an exact decimal string (never
		// float); NULL (no itm_replenish row) maps to nil, which the wire and
		// UI render as "-", never "0.00".
		escrow, err := numericToDecimalStringPtr(r.Escrow)
		if err != nil {
			return nil, fmt.Errorf("converting escrow for terminal %s: %w", r.TerminalID, err)
		}
		data[i] = ForecastRow{
			TerminalID:      r.TerminalID,
			PeriodePred:     r.PeriodePred.Time,
			Denom:           r.Denom,
			AmountReplenish: r.AmountReplenish,
			AmountRefund:    r.AmountRefund,
			DmaaFileID:      r.DmaaFileID,
			LokasiATM:       notesOrEmpty(r.LokasiAtm),
			Brand:           notesOrEmpty(r.Brand),
			FLMVendor:       notesOrEmpty(r.FlmVendor),
			FLMVendorRegion: notesOrEmpty(r.FlmVendorRegion),
			PriorityClass:   notesOrEmpty(r.PriorityClass),
			Paket:           notesOrEmpty(packageLabel(r.Paket, r.PaketVendor)),
			Escrow:          escrow,
			IsRequested:     r.IsRequested,
			VisitRemaining:  r.VisitRemaining,
			VisitQuotaTotal: r.VisitQuotaTotal,
		}
	}

	return &BrowseForecastResult{
		Data:       data,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages(total, params.PageSize),
	}, nil
}

// ForecastSummary recaps one forecast_date per (FLM vendor, region)
// (forecast-browser-summary FR1): ATM counts requested vs not, and amounts.
// Totals are summed here from the groups rather than by a second query, so
// they can never disagree with the rows shown. Reads the primary pool (plan
// D1): the page is reopened right after a Create, so replica lag would hide
// the drop in "belum".
func (s *VendorRequestService) ForecastSummary(ctx context.Context, forecastDate string) (*ForecastSummaryResult, error) {
	if forecastDate == "" {
		return nil, &ValidationError{Field: "forecast_date", Message: "wajib diisi"}
	}
	if err := validateDateBound("forecast_date", forecastDate); err != nil {
		return nil, err
	}
	date, err := time.Parse("2006-01-02", forecastDate)
	if err != nil {
		return nil, &ValidationError{Field: "forecast_date", Message: "harus berformat YYYY-MM-DD"}
	}

	rows, err := s.read.SummarizeForecastForDate(ctx, toPgDate(date))
	if err != nil {
		return nil, fmt.Errorf("summarizing forecast for %s: %w", forecastDate, err)
	}

	result := &ForecastSummaryResult{ForecastDate: date, Groups: make([]ForecastSummaryGroup, len(rows))}
	for i, r := range rows {
		g := ForecastSummaryGroup{
			VendorID:                   r.VendorID,
			FLMVendor:                  r.FlmVendor,
			FLMVendorRegion:            r.FlmVendorRegion,
			ATMCount:                   r.AtmCount,
			RequestedATMCount:          r.RequestedAtmCount,
			UnrequestedATMCount:        r.AtmCount - r.RequestedAtmCount,
			AmountReplenish:            r.AmountReplenish,
			UnrequestedAmountReplenish: r.UnrequestedAmountReplenish,
		}
		result.Groups[i] = g
		t := &result.Totals
		t.ATMCount += g.ATMCount
		t.RequestedATMCount += g.RequestedATMCount
		t.UnrequestedATMCount += g.UnrequestedATMCount
		t.AmountReplenish += g.AmountReplenish
		t.UnrequestedAmountReplenish += g.UnrequestedAmountReplenish
		if g.FLMVendor == "" {
			t.UnassignedATMCount += g.ATMCount
		}
	}
	return result, nil
}

// ListVendorOptions returns the active-vendor and distinct-region option
// lists backing the Forecast Browser's required FLM Vendor / FLM Vendor
// Region selects (Req 1.2, 1.3) and the manual-request vendor select (Req 3,
// Q2). No filters/pagination: master data, expected to be small.
func (s *VendorRequestService) ListVendorOptions(ctx context.Context) (*VendorOptionsResult, error) {
	vendors, err := s.read.ListActiveVendors(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing active vendors: %w", err)
	}
	regionRows, err := s.read.ListDistinctVendorBranchRegions(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing distinct vendor branch regions: %w", err)
	}

	result := &VendorOptionsResult{
		Vendors: make([]VendorOption, len(vendors)),
		Regions: make([]string, 0, len(regionRows)),
	}
	for i, v := range vendors {
		result.Vendors[i] = VendorOption{ID: v.ID, Name: v.Name}
	}
	for _, r := range regionRows {
		if r != nil {
			result.Regions = append(result.Regions, *r)
		}
	}
	return result, nil
}

// List returns a paginated, filtered vendor_requests summary (Req 9.1-9.5).
func (s *VendorRequestService) List(ctx context.Context, params ListVendorRequestParams) (*ListVendorRequestResult, error) {
	if params.Page < 1 {
		return nil, &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		return nil, &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	for _, st := range params.Status {
		if !strings.Contains(","+strings.Join(validVendorRequestStatuses, ",")+",", ","+st+",") {
			return nil, &ValidationError{Field: "status", Message: "status tidak valid: " + st}
		}
	}
	if params.ForecastDate != "" {
		if err := validateDateBound("forecast_date", params.ForecastDate); err != nil {
			return nil, err
		}
	}
	if len(params.RequestNumber) > 50 {
		return nil, &ValidationError{Field: "request_number", Message: "maksimal 50 karakter"}
	}

	arg := db.ListVendorRequestsParams{
		Status:          strings.Join(params.Status, ","),
		ForecastDate:    params.ForecastDate,
		CreatedBy:       params.CreatedBy,
		RequestNumber:   params.RequestNumber,
		IncludeCanceled: params.IncludeCanceled,
		Page:            int32(params.Page),     //nolint:gosec // G115: page <= maxPage, page_size <= 100 (handler)
		PageSize:        int32(params.PageSize), //nolint:gosec // G115: page <= maxPage, page_size <= 100 (handler)
	}
	rows, err := s.read.ListVendorRequests(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("listing vendor requests: %w", err)
	}
	total, err := s.read.CountVendorRequests(ctx, db.CountVendorRequestsParams{
		Status:          arg.Status,
		ForecastDate:    arg.ForecastDate,
		CreatedBy:       arg.CreatedBy,
		RequestNumber:   arg.RequestNumber,
		IncludeCanceled: arg.IncludeCanceled,
	})
	if err != nil {
		return nil, fmt.Errorf("counting vendor requests: %w", err)
	}

	data := make([]VendorRequestSummary, len(rows))
	for i, r := range rows {
		summary := VendorRequestSummary{
			ID:                 r.ID,
			RequestNumber:      r.RequestNumber,
			ForecastDate:       r.ForecastDate.Time,
			Status:             r.Status,
			Notes:              notesOrEmpty(r.Notes),
			ItemCount:          r.ItemCount,
			TotalAmount:        r.TotalAmount,
			CreatedBy:          UserRef{ID: r.CreatedByID, FullName: r.CreatedByName},
			CreatedAt:          r.CreatedAt.Time,
			SubmittedAt:        timestamptzToPtr(r.SubmittedAt),
			ApprovedAt:         timestamptzToPtr(r.ApprovedAt),
			RejectedAt:         timestamptzToPtr(r.RejectedAt),
			ReplenishDate:      dateToPtr(r.ReplenishDate),
			RequestCategory:    r.RequestCategory,
			IsCanceled:         r.IsCanceled,
			IsManual:           r.IsManual,
			CancellationReason: r.CancellationReason,
		}
		if r.ApprovedByID != nil {
			summary.ApprovedBy = &UserRef{ID: *r.ApprovedByID, FullName: notesOrEmpty(r.ApprovedByName)}
		}
		data[i] = summary
	}

	return &ListVendorRequestResult{
		Data:       data,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages(total, params.PageSize),
	}, nil
}

// Get returns the full detail of one vendor request (Req 9.6).
func (s *VendorRequestService) Get(ctx context.Context, id int64) (*VendorRequestDetail, error) {
	header, err := s.read.GetVendorRequestDetail(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get vendor request %d: %w", id, err)
	}

	items, err := s.read.ListVendorRequestItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list items for vendor request %d: %w", id, err)
	}

	tickets, err := s.read.ListActiveVendorRequestTickets(ctx, header.RequestNumber)
	if err != nil {
		return nil, fmt.Errorf("list tickets for vendor request %d: %w", id, err)
	}
	detail := mapDetail(header, items)
	ticketNumbers := make(map[string]string, len(tickets))
	for _, t := range tickets {
		ticketNumbers[t.TerminalID] = t.TicketNumber
	}
	for i := range detail.Items {
		detail.Items[i].TicketNumber = ticketNumbers[detail.Items[i].TerminalID]
	}
	if detail.Atms, err = s.requestAtmStatuses(ctx, id, tickets); err != nil {
		return nil, err
	}
	return detail, nil
}

// AuditLog returns the append-only audit trail for one vendor request,
// oldest first (Req 16.4). 404s (via ErrNotFound) if the request doesn't
// exist, matching Req 16.7.
func (s *VendorRequestService) AuditLog(ctx context.Context, id int64) ([]AuditEntry, error) {
	if _, err := s.read.GetVendorRequest(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("check vendor request %d exists: %w", id, err)
	}

	rows, err := s.read.ListAuditLogsByEntity(ctx, db.ListAuditLogsByEntityParams{EntityType: "vendor_request", EntityID: id})
	if err != nil {
		return nil, fmt.Errorf("list audit log for vendor request %d: %w", id, err)
	}

	entries := make([]AuditEntry, len(rows))
	for i, r := range rows {
		entries[i] = AuditEntry{
			ID:            r.ID,
			EntityType:    r.EntityType,
			EntityID:      r.EntityID,
			Action:        r.Action,
			PerformedBy:   r.ActorID,
			PerformedAt:   r.CreatedAt.Time,
			PreviousState: extractState(r.Before),
			NewState:      extractState(r.After),
			Metadata:      extractMetadata(r.After),
		}
	}
	return entries, nil
}
