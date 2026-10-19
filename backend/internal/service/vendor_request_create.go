package service

import (
	"context"
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
	atms, err := validateItemsSingleVendor(ctx, q, in.VendorID, resolved)
	if err != nil {
		return nil, err
	}
	regionCode, err := singleRegion(atms, "")
	if err != nil {
		return nil, err
	}

	forecastDate := toPgDate(in.ForecastDate)
	requestID, requestNumber, err := createWithRetryingNumber(ctx, tx, q, in, regionCode, forecastDate, actor)
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
	tickets, err := applyTicketPlan(ctx, q, requestNumber, toPgDate(in.ReplenishDate), planTickets(nil, wantedCodes(resolved)))
	if err != nil {
		return nil, err
	}

	// Req 3.13 + replenishment-request-enhancements Req 1.13: every create
	// audit carries the category (validation above guarantees one of the
	// three values), state, actor; the audit.Writer stamps the UTC timestamp.
	metadata := map[string]any{
		"state": "draft", "is_manual": in.IsManual, "request_category": in.RequestCategory,
		"region_code": regionCode, "tickets": tickets.Added, // replenish-ticket FR6.1
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
// cannot be confirmed to belong to the chosen vendor. Returns each ATM's
// managing cabang + region code for singleRegion (replenish-ticket FR10.1).
func validateItemsSingleVendor(ctx context.Context, q *db.Queries, vendorID int64, resolved []resolvedItem) (map[string]branchRegion, error) {
	var mismatched []string
	atms := make(map[string]branchRegion, len(resolved))
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
			return nil, fmt.Errorf("resolve active vendor for %s: %w", r.input.TerminalID, err)
		}
		if vendor.VendorID != vendorID {
			mismatched = append(mismatched, r.input.TerminalID)
		}
		atms[r.input.TerminalID] = branchRegion{branchCode: vendor.BranchCode, regionCode: vendor.RegionCode}
	}
	if len(mismatched) > 0 {
		return nil, &ValidationError{Field: "items", Message: "item tidak sesuai dengan vendor yang dipilih: " + strings.Join(mismatched, ", ")}
	}
	return atms, nil
}

// createWithRetryingNumber generates a REP-<prefix>-<region>-<YYYYMMDD>-<seq>
// request number (region segment + per-region counter: replenish-ticket
// FR10, migration 026) (Req 4.1; replenishment-request-enhancements Req 4 inserts the
// hyphen separators — prefix/date/seq sources are unchanged, Req 4.2) and
// inserts the header, retrying up to 5 times on a request_number
// unique-constraint race (Req 4.6). The date segment and the
// atomic per-scope sequence both key off replenish_date, not forecast_date
// (Q5, Req 4.4): the number is an operational identifier for the day cash is
// delivered, not the DMAA periode it was drawn from. Legacy VR-YYYYMMDD-NNNN
// rows are never rewritten (Req 4.9) -- this only affects new creates.
func createWithRetryingNumber(ctx context.Context, tx pgx.Tx, q *db.Queries, in CreateVendorRequestInput, regionCode string, forecastDate pgtype.Date, actor Actor) (int64, string, error) {
	prefix, err := resolveVendorPrefix(ctx, q, in.VendorID)
	if err != nil {
		return 0, "", err
	}
	replenishDate := toPgDate(in.ReplenishDate)
	dateSeg := in.ReplenishDate.Format("20060102")

	const maxAttempts = 5
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		request, err := attemptCreateWithNumber(ctx, tx, in, prefix, regionCode, dateSeg, replenishDate, forecastDate, actor)
		if err == nil {
			return request.ID, request.RequestNumber, nil
		}
		if errors.Is(err, ErrNumberExhausted) {
			return 0, "", err
		}
		if !isUniqueViolation(err) {
			return 0, "", fmt.Errorf("create vendor request: %w", err)
		}
		lastErr = err
	}
	return 0, "", fmt.Errorf("%w: %v", ErrNumberGeneration, lastErr)
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
func attemptCreateWithNumber(ctx context.Context, tx pgx.Tx, in CreateVendorRequestInput, prefix, regionCode, dateSeg string, replenishDate, forecastDate pgtype.Date, actor Actor) (db.VendorRequest, error) {
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return db.VendorRequest{}, fmt.Errorf("begin savepoint: %w", err)
	}
	defer func() { _ = savepoint.Rollback(ctx) }()
	q := db.New(savepoint)

	seq, err := q.NextRequestNumberSeq(ctx, db.NextRequestNumberSeqParams{
		VendorID:   in.VendorID,
		RegionCode: regionCode,
		SeqDate:    replenishDate,
	})
	if err != nil {
		if isSequenceExhausted(err) {
			return db.VendorRequest{}, ErrNumberExhausted
		}
		return db.VendorRequest{}, fmt.Errorf("next request number seq: %w", err)
	}
	// Belt-and-suspenders (design.md): the vendor_request_number_seq_last_chk
	// CHECK is the primary guard (surfaced above via isSequenceExhausted),
	// but this catches exhaustion even if that CHECK were ever relaxed.
	if seq > 999 {
		return db.VendorRequest{}, ErrNumberExhausted
	}
	requestNumber := fmt.Sprintf("REP-%s-%s-%s-%03d", prefix, regionCode, dateSeg, seq)

	request, err := q.CreateVendorRequest(ctx, db.CreateVendorRequestParams{
		RequestNumber:   requestNumber,
		ForecastDate:    forecastDate,
		ReplenishDate:   replenishDate,
		RequestCategory: categoryOrNil(in),
		IsManual:        in.IsManual,
		VendorID:        in.VendorID,
		RegionCode:      regionCode,
		Notes:           stringPtrOrNil(in.Notes),
		CreatedBy:       actor.UserID,
	})
	if err != nil {
		return db.VendorRequest{}, err
	}
	if err := savepoint.Commit(ctx); err != nil {
		return db.VendorRequest{}, fmt.Errorf("commit savepoint: %w", err)
	}
	return request, nil
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
