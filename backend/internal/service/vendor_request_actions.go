package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

const pgUniqueViolation = "23505"
const pgCheckViolation = "23514"

var validVendorRequestStatuses = []string{
	"draft", "pending_approval", "approved", "rejected", "processing", "completed", "cancelled",
}

// --- Reads (no transaction needed) --------------------------------------

// BrowseForecast lists dmaa_atm_forecast rows for one forecast_date (Req 3).
// CIT-2 (Req 1): FLMVendor/FLMVendorRegion are required (server-side backstop
// for the frontend's block-fetch behavior, Req 1.4); Brand stays optional.
// All three are length-bound to <=255 chars (Req 1.14).
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
	if params.FLMVendor == "" {
		return nil, &ValidationError{Field: "flm_vendor", Message: "wajib dipilih"}
	}
	if params.FLMVendorRegion == "" {
		return nil, &ValidationError{Field: "flm_vendor_region", Message: "wajib dipilih"}
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
		Page:            int32(params.Page),
		PageSize:        int32(params.PageSize),
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
			Paket:           notesOrEmpty(r.Paket),
			Escrow:          escrow,
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
		Page:            int32(params.Page),
		PageSize:        int32(params.PageSize),
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

	return mapDetail(header, items), nil
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

// --- Mutations (each runs inside one DB transaction) --------------------

// Create validates the payload, generates a request_number, and inserts the
// draft header + items (Req 4, 15). CIT-2 (Req 2, 3) adds replenish_date
// validation (applies to every create), Manual_Request request_category
// handling, branching item resolution (DMAA match vs. manual accept) and
// the Q2 single-vendor defence-in-depth check.
// replenishment-request-enhancements (Req 1): request_category is now
// required and date-validated on EVERY create (5.1), item resolution
// branches on the category instead of IsManual (5.2), and the category is
// persisted + audited for all creates (5.3) — see the numbered comments
// below.
func (s *VendorRequestService) Create(ctx context.Context, actor Actor, in CreateVendorRequestInput) (*VendorRequestDetail, error) {
	if err := validateItemsPayload(in.Items, in.Notes, 0); err != nil {
		return nil, err
	}
	if in.VendorID <= 0 {
		return nil, &ValidationError{Field: "vendor_id", Message: "wajib diisi"}
	}
	if err := validateReplenishDate(in.ReplenishDate); err != nil {
		return nil, err
	}
	// replenishment-request-enhancements (Req 1.10, 1.11): category→date
	// consistency applies to EVERY create, manual or DMAA-backed — the
	// default branch rejects an unknown/empty category, so a create can no
	// longer reach the tx without one of {planned, emergency, additional}.
	if err := validateCategoryDateConsistency(in.RequestCategory, in.ReplenishDate); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	var resolved []resolvedItem
	if in.RequestCategory == "emergency" || in.RequestCategory == "additional" {
		// Req 1.7, 1.8: Emergency/Additional accept items without requiring a
		// dmaa_atm_forecast match — manual and Forecast-Browser flow alike.
		resolved, err = acceptItems(in.Items, in.ReplenishDate)
	} else {
		// Planned (manual or Forecast-Browser) still validates against DMAA
		// like a normal create (Req 1.6, Req 3.8). The empty-category case
		// cannot reach here: validateCategoryDateConsistency rejected it
		// above (Req 1.11). UpdateItems bypasses this branch entirely and
		// keeps its own direct resolveItems call.
		resolved, err = resolveItems(ctx, q, in.Items)
	}
	if err != nil {
		return nil, err
	}
	if err := validateItemsSingleVendor(ctx, q, in.VendorID, resolved); err != nil {
		return nil, err
	}

	forecastDate := toPgDate(in.ForecastDate)
	requestID, err := createWithRetryingNumber(ctx, tx, q, in, forecastDate, actor)
	if err != nil {
		return nil, err
	}

	var overrides []string
	for _, r := range resolved {
		if _, err := q.InsertVendorRequestItem(ctx, db.InsertVendorRequestItemParams{
			VendorRequestID: requestID,
			TerminalID:      r.input.TerminalID,
			PeriodePred:     toPgDate(r.input.PeriodePred),
			Denom:           r.input.Denom,
			AmountReplenish: r.input.AmountReplenish,
			AmountRefund:    r.amountRefund,
			Brand:           stringPtrOrNil(r.input.Brand),
			LokasiAtm:       stringPtrOrNil(r.input.LokasiATM),
		}); err != nil {
			return nil, fmt.Errorf("insert item %s: %w", r.input.TerminalID, err)
		}
		if r.overridden {
			overrides = append(overrides, r.input.TerminalID)
		}
	}

	// Req 3.13 + replenishment-request-enhancements Req 1.13: every create
	// audit carries the category (validation above guarantees one of the
	// three values), state, actor; the audit.Writer stamps the UTC timestamp.
	metadata := map[string]any{
		"state": "draft", "is_manual": in.IsManual, "request_category": in.RequestCategory,
	}
	if len(overrides) > 0 {
		metadata["amount_overrides"] = overrides // Req 4.5: manual override discrepancy
	}
	if err := newAuditWriter(tx).Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: "create", EntityType: "vendor_request", EntityID: requestID,
		After: metadata, IP: actor.IP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create vendor request: %w", err)
	}
	return s.Get(ctx, requestID)
}

// validateReplenishDate enforces Req 2.6 (required calendar date -- the
// zero time.Time value signals "missing"; string-format validation happens
// in the handler, same as ForecastDate) and Req 2.7 (not earlier than today
// in Asia/Jakarta).
func validateReplenishDate(d time.Time) error {
	if d.IsZero() {
		return &ValidationError{Field: "replenish_date", Message: "wajib diisi"}
	}
	if d.Before(jakartaCalendarDate(0)) {
		return &ValidationError{Field: "replenish_date", Message: "tidak boleh sebelum hari ini"}
	}
	return nil
}

// validateCategoryDateConsistency enforces Req 3.11 (category must be one of
// the three) and Req 3.3-3.5/3.10 (the replenish_date each category allows).
// replenishment-request-enhancements (Req 1.10, 1.11): called for EVERY
// create, manual or DMAA-backed — the default branch makes the category
// mandatory at the service layer.
func validateCategoryDateConsistency(category string, replenishDate time.Time) error {
	switch category {
	case "planned":
		if !replenishDate.Equal(jakartaCalendarDate(1)) {
			return &ValidationError{Field: "replenish_date", Message: "kategori planned wajib H+1"}
		}
	case "emergency":
		if !replenishDate.Equal(jakartaCalendarDate(0)) {
			return &ValidationError{Field: "replenish_date", Message: "kategori emergency wajib H+0"}
		}
	case "additional":
		if !replenishDate.Equal(jakartaCalendarDate(0)) &&
			!replenishDate.Equal(jakartaCalendarDate(1)) &&
			!replenishDate.Equal(jakartaCalendarDate(2)) {
			return &ValidationError{Field: "replenish_date", Message: "kategori additional wajib H+0, H+1, atau H+2"}
		}
	default:
		return &ValidationError{Field: "request_category", Message: "harus salah satu dari: planned, emergency, additional"}
	}
	return nil
}

// acceptItems validates Emergency/Additional items without requiring a
// dmaa_atm_forecast match (Req 1.7, 1.8) — manual and Forecast-Browser flow
// alike: terminal_id 1-64 chars, denom > 0, amount_replenish > 0 (Req 3.6,
// 3.14). amount_refund is always 0 (no DMAA match required, so there is no
// row to copy from). periode_pred: items drawn from the Forecast Browser
// already carry the DMAA periode — keep it to preserve the (terminal_id,
// periode_pred, denom) identity (Req 5.12); anchor to replenish_date only
// when the caller sent none (a manual item has no DMAA periode analogue).
// Operator-entered brand/lokasi_atm pass through on the item itself so
// Create can persist them (Q4).
func acceptItems(items []ItemInput, replenishDate time.Time) ([]resolvedItem, error) {
	resolved := make([]resolvedItem, 0, len(items))
	for _, it := range items {
		if len(it.TerminalID) < 1 || len(it.TerminalID) > 64 {
			return nil, &ValidationError{Field: "terminal_id", Message: "harus 1-64 karakter"}
		}
		if it.Denom <= 0 {
			return nil, &ValidationError{Field: "denom", Message: "harus lebih besar dari 0"}
		}
		if it.AmountReplenish <= 0 {
			return nil, &ValidationError{Field: "amount_replenish", Message: "harus lebih besar dari 0"}
		}
		if it.PeriodePred.IsZero() {
			it.PeriodePred = replenishDate
		}
		resolved = append(resolved, resolvedItem{input: it, amountRefund: 0})
	}
	return resolved, nil
}

// validateItemsSingleVendor enforces Q2's defence-in-depth: every item's ATM
// must resolve, as of its own periode_pred, to the same vendor as the
// request's chosen vendor_id -- catching a stale Forecast Browser selection
// or a manual entry against the wrong vendor's ATM. An item whose ATM has no
// resolvable active vendor package also counts as a mismatch: the request
// cannot be confirmed to belong to the chosen vendor.
func validateItemsSingleVendor(ctx context.Context, q *db.Queries, vendorID int64, resolved []resolvedItem) error {
	var mismatched []string
	for _, r := range resolved {
		vendor, err := q.GetActiveVendorForTerminal(ctx, db.GetActiveVendorForTerminalParams{
			TerminalID: r.input.TerminalID,
			AsOfDate:   toPgDate(r.input.PeriodePred),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				mismatched = append(mismatched, r.input.TerminalID)
				continue
			}
			return fmt.Errorf("resolve active vendor for %s: %w", r.input.TerminalID, err)
		}
		if vendor != vendorID {
			mismatched = append(mismatched, r.input.TerminalID)
		}
	}
	if len(mismatched) > 0 {
		return &ValidationError{Field: "items", Message: "item tidak sesuai dengan vendor yang dipilih: " + strings.Join(mismatched, ", ")}
	}
	return nil
}

// createWithRetryingNumber generates a REP-<prefix>-<YYYYMMDD>-<seq> request
// number (Req 4.1; replenishment-request-enhancements Req 4 inserts the
// hyphen separators — prefix/date/seq sources are unchanged, Req 4.2) and
// inserts the header, retrying up to 5 times on a request_number
// unique-constraint race (Req 4.6). The date segment and the
// atomic per-scope sequence both key off replenish_date, not forecast_date
// (Q5, Req 4.4): the number is an operational identifier for the day cash is
// delivered, not the DMAA periode it was drawn from. Legacy VR-YYYYMMDD-NNNN
// rows are never rewritten (Req 4.9) -- this only affects new creates.
func createWithRetryingNumber(ctx context.Context, tx pgx.Tx, q *db.Queries, in CreateVendorRequestInput, forecastDate pgtype.Date, actor Actor) (int64, error) {
	prefix, err := resolveVendorPrefix(ctx, q, in.VendorID)
	if err != nil {
		return 0, err
	}
	replenishDate := toPgDate(in.ReplenishDate)
	dateSeg := in.ReplenishDate.Format("20060102")

	const maxAttempts = 5
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		requestID, err := attemptCreateWithNumber(ctx, tx, in, prefix, dateSeg, replenishDate, forecastDate, actor)
		if err == nil {
			return requestID, nil
		}
		if errors.Is(err, ErrNumberExhausted) {
			return 0, err
		}
		if !isUniqueViolation(err) {
			return 0, fmt.Errorf("create vendor request: %w", err)
		}
		lastErr = err
	}
	return 0, fmt.Errorf("%w: %v", ErrNumberGeneration, lastErr)
}

// attemptCreateWithNumber runs one number-generation + insert attempt inside
// a pgx pseudo-nested transaction (SAVEPOINT under the hood, pgx.Tx.Begin).
// Without this, a unique-violation on q.CreateVendorRequest aborts the whole
// *outer* Create transaction, and every subsequent retry's
// NextRequestNumberSeq/CreateVendorRequest call in the loop above would fail
// with "current transaction is aborted" (Postgres 25P02) instead of actually
// retrying -- silently breaking the "≤5 retries on unique-violation"
// guarantee (Req 4.6) the very first time two creates ever raced. Scoping
// each attempt to its own savepoint means only that attempt rolls back on
// conflict, leaving the outer tx healthy for the next one.
func attemptCreateWithNumber(ctx context.Context, tx pgx.Tx, in CreateVendorRequestInput, prefix, dateSeg string, replenishDate, forecastDate pgtype.Date, actor Actor) (int64, error) {
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin savepoint: %w", err)
	}
	defer func() { _ = savepoint.Rollback(ctx) }()
	q := db.New(savepoint)

	seq, err := q.NextRequestNumberSeq(ctx, db.NextRequestNumberSeqParams{
		VendorID: in.VendorID,
		SeqDate:  replenishDate,
	})
	if err != nil {
		if isSequenceExhausted(err) {
			return 0, ErrNumberExhausted
		}
		return 0, fmt.Errorf("next request number seq: %w", err)
	}
	// Belt-and-suspenders (design.md): the vendor_request_number_seq_last_chk
	// CHECK is the primary guard (surfaced above via isSequenceExhausted),
	// but this catches exhaustion even if that CHECK were ever relaxed.
	if seq > 999 {
		return 0, ErrNumberExhausted
	}
	requestNumber := fmt.Sprintf("REP-%s-%s-%03d", prefix, dateSeg, seq)

	request, err := q.CreateVendorRequest(ctx, db.CreateVendorRequestParams{
		RequestNumber:   requestNumber,
		ForecastDate:    forecastDate,
		ReplenishDate:   replenishDate,
		RequestCategory: categoryOrNil(in),
		IsManual:        in.IsManual,
		VendorID:        in.VendorID,
		Notes:           stringPtrOrNil(in.Notes),
		CreatedBy:       actor.UserID,
	})
	if err != nil {
		return 0, err
	}
	if err := savepoint.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit savepoint: %w", err)
	}
	return request.ID, nil
}

// isSequenceExhausted reports whether err is the
// vendor_request_number_seq_last_chk CHECK violation (Req 4.7): the atomic
// upsert would have pushed last_seq past 999 for this (vendor, date) scope.
func isSequenceExhausted(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgCheckViolation &&
		pgErr.ConstraintName == "vendor_request_number_seq_last_chk"
}

// categoryOrNil persists request_category for every create whose category is
// one of the three valid values (replenishment-request-enhancements Req 1.9):
// validateCategoryDateConsistency already rejected anything else by the time
// Create reaches the insert, so this switch is defence-in-depth — the
// vendor_requests_category_chk CHECK (migration 034) is the second net.
func categoryOrNil(in CreateVendorRequestInput) *string {
	switch in.RequestCategory {
	case "planned", "emergency", "additional":
		return &in.RequestCategory
	}
	return nil
}

// resolveVendorPrefix returns the Vendor_Prefix for vendorID (Req 4.2, 4.3,
// design.md Q1): vendors.request_prefix if set, else the deterministic
// fallback derived from vendors.code -- never empty, truncated, or
// non-3-char. Takes the tx-scoped *db.Queries directly, matching
// resolveItems below rather than going through VendorRequestRepository.
func resolveVendorPrefix(ctx context.Context, q *db.Queries, vendorID int64) (string, error) {
	vendor, err := q.GetVendorForRequestNumber(ctx, vendorID)
	if err != nil {
		return "", fmt.Errorf("get vendor %d for request number: %w", vendorID, err)
	}
	if vendor.RequestPrefix != nil {
		return *vendor.RequestPrefix, nil
	}
	return fallbackVendorPrefix(vendor.Code), nil
}

// fallbackVendorPrefix derives a deterministic 3-uppercase-char prefix from
// a vendor's code when vendors.request_prefix is NULL (Req 4.3, design.md
// Q1): uppercase, strip everything but A-Z/0-9, take the first 3 characters,
// or right-pad with 'X' to 3 if shorter. Always returns exactly 3 characters.
func fallbackVendorPrefix(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	stripped := b.String()
	if len(stripped) >= 3 {
		return stripped[:3]
	}
	return stripped + strings.Repeat("X", 3-len(stripped))
}

// UpdateItems replaces all items of a draft request (Req 8, full-replacement
// strategy).
func (s *VendorRequestService) UpdateItems(ctx context.Context, actor Actor, id int64, items []ItemInput) (*VendorRequestDetail, error) {
	if err := validateItemsPayload(items, "", 1000); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	req, err := q.GetVendorRequestForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load vendor request %d: %w", id, err)
	}
	if req.Status != "draft" {
		return nil, ErrInvalidTransition
	}
	if actor.UserID != req.CreatedBy {
		return nil, ErrNotCreator
	}

	resolved, err := resolveItems(ctx, q, items)
	if err != nil {
		return nil, err
	}

	if err := q.DeleteVendorRequestItems(ctx, id); err != nil {
		return nil, fmt.Errorf("delete existing items for %d: %w", id, err)
	}
	var overrides []string
	for _, r := range resolved {
		if _, err := q.InsertVendorRequestItem(ctx, db.InsertVendorRequestItemParams{
			VendorRequestID: id,
			TerminalID:      r.input.TerminalID,
			PeriodePred:     toPgDate(r.input.PeriodePred),
			Denom:           r.input.Denom,
			AmountReplenish: r.input.AmountReplenish,
			AmountRefund:    r.amountRefund,
		}); err != nil {
			return nil, fmt.Errorf("insert item %s: %w", r.input.TerminalID, err)
		}
		if r.overridden {
			overrides = append(overrides, r.input.TerminalID)
		}
	}

	metadata := map[string]any{"state": req.Status, "items_added": len(resolved), "items_removed": 0, "items_modified": 0}
	if len(overrides) > 0 {
		metadata["amount_overrides"] = overrides
	}
	if err := newAuditWriter(tx).Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: "update_items", EntityType: "vendor_request", EntityID: id,
		Before: map[string]string{"state": req.Status}, After: metadata, IP: actor.IP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update items for %d: %w", id, err)
	}
	return s.Get(ctx, id)
}

// Submit: draft -> pending_approval (Req 5).
func (s *VendorRequestService) Submit(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error) {
	return s.transition(ctx, actor, id, transitionOpts{action: actionSubmit, auditAction: "submit", requireItems: true})
}

// Approve: pending_approval -> approved (Req 6).
func (s *VendorRequestService) Approve(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error) {
	return s.transition(ctx, actor, id, transitionOpts{action: actionApprove, auditAction: "approve"})
}

// Reject: pending_approval -> rejected (Req 6). reason is required, 1-500
// non-whitespace chars after trim (Req 6.2/6.12; design.md's API contract
// and the frontend spec (Req 14.6) both say 500, so that value is used here
// — Req 2.11's "maximum 1000 characters" is the outlier of the two and is
// treated as the likely typo).
func (s *VendorRequestService) Reject(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil, ErrRejectReasonEmpty
	}
	if len(trimmed) > 500 {
		return nil, &ValidationError{Field: "rejection_reason", Message: "maksimal 500 karakter"}
	}
	return s.transition(ctx, actor, id, transitionOpts{action: actionReject, auditAction: "reject", rejectionReason: trimmed})
}

// Revise: rejected -> draft (Req 7).
func (s *VendorRequestService) Revise(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error) {
	return s.transition(ctx, actor, id, transitionOpts{action: actionRevise, auditAction: "revise"})
}

// Cancel: draft|pending_approval|approved -> cancelled, plus is_canceled=true
// (Req 10 union rule for pending_approval / Req 3.7 checker-only for
// approved — see checkActor's doc comment; Req 5 soft-cancel). reason is
// required, 1-500 non-whitespace chars after trim (Req 3.2/3.3, same bound
// as Reject), and is persisted to vendor_requests.cancellation_reason (Req
// 3.11 Opsi B, migration 038) as well as the audit entry's `after`. Unlike
// the other transitions, Cancel no longer shares the transition() helper: it
// needs the already-canceled guard (Req 5.7/3.8) and SoftCancelVendorRequest
// (which sets is_canceled alongside status in one statement, Req 5.2)
// instead of UpdateVendorRequestStatus.
func (s *VendorRequestService) Cancel(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil, ErrCancelReasonEmpty
	}
	if len(trimmed) > 500 {
		return nil, &ValidationError{Field: "cancellation_reason", Message: "maksimal 500 karakter"}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	req, err := q.GetVendorRequestForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load vendor request %d: %w", id, err)
	}

	// Order matches design.md: already-canceled guard first (Req 5.7, a
	// conflict distinct from an invalid-transition), then the state guard
	// (Req 5.9), then the actor guard (Req 5.3, 5.8, 3.7).
	if req.IsCanceled {
		return nil, ErrAlreadyCanceled
	}
	if _, ok := nextState(req.Status, actionCancel); !ok {
		return nil, ErrInvalidTransition
	}
	if err := checkActor(actor, req, actionCancel); err != nil {
		return nil, err
	}

	if _, err := q.SoftCancelVendorRequest(ctx, db.SoftCancelVendorRequestParams{ID: id, CancellationReason: trimmed}); err != nil {
		return nil, fmt.Errorf("soft cancel vendor request %d: %w", id, err)
	}

	if err := newAuditWriter(tx).Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: "cancel", EntityType: "vendor_request", EntityID: id,
		Before: map[string]any{"state": req.Status, "is_canceled": false},
		After:  map[string]any{"state": "cancelled", "is_canceled": true, "reason": trimmed},
		IP:     actor.IP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancel vendor request %d: %w", id, err)
	}
	return s.Get(ctx, id)
}

type transitionOpts struct {
	action          action
	auditAction     string
	rejectionReason string
	requireItems    bool // Req 5.4: submit requires >=1 item
}

// transition is the shared tx→lock→guard→update→audit flow behind Submit,
// Approve, Reject, Revise, and Cancel (design.md's per-method pseudocode
// collapsed into one function since all five share the same shape).
// Order matches design.md: state guard before actor guard.
func (s *VendorRequestService) transition(ctx context.Context, actor Actor, id int64, opts transitionOpts) (*VendorRequestDetail, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	req, err := q.GetVendorRequestForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load vendor request %d: %w", id, err)
	}

	newStatus, ok := nextState(req.Status, opts.action)
	if !ok {
		return nil, ErrInvalidTransition
	}
	if err := checkActor(actor, req, opts.action); err != nil {
		return nil, err
	}

	if opts.requireItems {
		items, err := q.ListVendorRequestItems(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("list items for %d: %w", id, err)
		}
		if len(items) == 0 {
			return nil, ErrEmptyItems
		}
	}

	var reasonParam *string
	if opts.action == actionReject {
		reasonParam = &opts.rejectionReason
	}
	if _, err := q.UpdateVendorRequestStatus(ctx, db.UpdateVendorRequestStatusParams{
		Status:          newStatus,
		ActorID:         &actor.UserID, // ignored by SQL CASE unless action is approve/reject
		RejectionReason: reasonParam,
		ID:              id,
	}); err != nil {
		return nil, fmt.Errorf("update vendor request %d status: %w", id, err)
	}

	after := map[string]any{"state": newStatus}
	if opts.action == actionReject {
		after["rejection_reason"] = opts.rejectionReason // Req 16.2
	}
	if err := newAuditWriter(tx).Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: opts.auditAction, EntityType: "vendor_request", EntityID: id,
		Before: map[string]string{"state": req.Status}, After: after, IP: actor.IP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit vendor request %d transition: %w", id, err)
	}
	return s.Get(ctx, id)
}

// --- Item validation / resolution ---------------------------------------

// validateItemsPayload checks the shape shared by Create and UpdateItems
// (Req 4.6, 8.1, 8.6): at least one item, no duplicate (terminal_id,
// periode_pred, denom) within the payload, and notes length. maxItems=0
// means no cap (Create has none; UpdateItems caps at 1000 per Req 8.1).
func validateItemsPayload(items []ItemInput, notes string, maxItems int) error {
	if len(items) == 0 {
		return ErrEmptyItems
	}
	if maxItems > 0 && len(items) > maxItems {
		return &ValidationError{Field: "items", Message: fmt.Sprintf("maksimal %d item", maxItems)}
	}
	if len(notes) > 500 {
		return &ValidationError{Field: "notes", Message: "maksimal 500 karakter"}
	}
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		key := it.TerminalID + "|" + it.PeriodePred.Format("2006-01-02") + "|" + fmt.Sprint(it.Denom)
		if _, dup := seen[key]; dup {
			return ErrDuplicateItems
		}
		seen[key] = struct{}{}
	}
	return nil
}

type resolvedItem struct {
	input        ItemInput
	amountRefund int64
	overridden   bool
}

// resolveItems validates every item against dmaa_atm_forecast (Req 4.4,
// 8.6) and carries each matched row's amount_refund forward for
// InsertVendorRequestItem, plus whether amount_replenish was manually
// overridden (Req 4.5). See queries/vendor_request.sql's ForecastRowExists
// doc comment for why this is a per-item loop rather than one set-based
// query.
func resolveItems(ctx context.Context, q *db.Queries, items []ItemInput) ([]resolvedItem, error) {
	resolved := make([]resolvedItem, 0, len(items))
	var invalid []ItemInput
	for _, it := range items {
		row, err := q.ForecastRowExists(ctx, db.ForecastRowExistsParams{
			TerminalID:  it.TerminalID,
			PeriodePred: toPgDate(it.PeriodePred),
			Denom:       it.Denom,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				invalid = append(invalid, it)
				continue
			}
			return nil, fmt.Errorf("check forecast row for %s: %w", it.TerminalID, err)
		}
		resolved = append(resolved, resolvedItem{
			input:        it,
			amountRefund: row.AmountRefund,
			overridden:   it.AmountReplenish != row.AmountReplenish,
		})
	}
	if len(invalid) > 0 {
		return nil, &InvalidItemsError{Items: invalid}
	}
	return resolved, nil
}

// --- response mapping ----------------------------------------------------

func mapDetail(h db.GetVendorRequestDetailRow, itemRows []db.VendorRequestItem) *VendorRequestDetail {
	detail := &VendorRequestDetail{
		ID:                 h.ID,
		RequestNumber:      h.RequestNumber,
		ForecastDate:       h.ForecastDate.Time,
		Status:             h.Status,
		Notes:              notesOrEmpty(h.Notes),
		CreatedBy:          UserRef{ID: h.CreatedBy, FullName: h.CreatedByName},
		RejectionReason:    notesOrEmpty(h.RejectionReason),
		CreatedAt:          h.CreatedAt.Time,
		UpdatedAt:          h.UpdatedAt.Time,
		SubmittedAt:        timestamptzToPtr(h.SubmittedAt),
		ApprovedAt:         timestamptzToPtr(h.ApprovedAt),
		RejectedAt:         timestamptzToPtr(h.RejectedAt),
		ReplenishDate:      dateToPtr(h.ReplenishDate),
		RequestCategory:    h.RequestCategory,
		IsCanceled:         h.IsCanceled,
		IsManual:           h.IsManual,
		CancellationReason: h.CancellationReason,
	}
	if h.ApprovedBy != nil {
		detail.ApprovedBy = &UserRef{ID: *h.ApprovedBy, FullName: notesOrEmpty(h.ApprovedByName)}
	}
	if h.RejectedBy != nil {
		detail.RejectedBy = &UserRef{ID: *h.RejectedBy, FullName: notesOrEmpty(h.RejectedByName)}
	}

	items := make([]VendorRequestItemOut, len(itemRows))
	var total int64
	for i, it := range itemRows {
		items[i] = VendorRequestItemOut{
			ID:              it.ID,
			TerminalID:      it.TerminalID,
			PeriodePred:     it.PeriodePred.Time,
			Denom:           it.Denom,
			AmountReplenish: it.AmountReplenish,
			AmountRefund:    it.AmountRefund,
		}
		total += it.AmountReplenish
	}
	detail.Items = items
	detail.TotalAmount = total
	return detail
}

// extractState reads the "state" key out of an audit_logs before/after jsonb
// column (design.md's mapping: Before={"state":prev}, After={"state":new,
// ...metadata}). Returns nil for a nil column (e.g. before on a create) or
// a payload with no "state" key.
func extractState(raw []byte) *string {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	v, ok := m["state"].(string)
	if !ok {
		return nil
	}
	return &v
}

// extractMetadata returns an audit_logs.after payload minus its "state"
// key (Req 16.4's "metadata" field).
func extractMetadata(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}
	delete(m, "state")
	if m == nil {
		return map[string]any{}
	}
	return m
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

func totalPages(total int64, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
