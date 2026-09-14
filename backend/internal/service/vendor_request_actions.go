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

var validVendorRequestStatuses = []string{
	"draft", "pending_approval", "approved", "rejected", "processing", "completed", "cancelled",
}

// --- Reads (no transaction needed) --------------------------------------

// BrowseForecast lists dmaa_atm_forecast rows for one forecast_date (Req 3).
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

	forecastDate, err := time.Parse("2006-01-02", params.ForecastDate)
	if err != nil {
		return nil, &ValidationError{Field: "forecast_date", Message: "harus berformat YYYY-MM-DD"}
	}

	rows, err := s.read.ListForecastForDate(ctx, db.ListForecastForDateParams{
		ForecastDate: toPgDate(forecastDate),
		TerminalID:   params.TerminalID,
		Page:         int32(params.Page),
		PageSize:     int32(params.PageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("listing forecast for %s: %w", params.ForecastDate, err)
	}
	total, err := s.read.CountForecastForDate(ctx, db.CountForecastForDateParams{
		ForecastDate: toPgDate(forecastDate),
		TerminalID:   params.TerminalID,
	})
	if err != nil {
		return nil, fmt.Errorf("counting forecast for %s: %w", params.ForecastDate, err)
	}

	data := make([]ForecastRow, len(rows))
	for i, r := range rows {
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
		Status:        strings.Join(params.Status, ","),
		ForecastDate:  params.ForecastDate,
		CreatedBy:     params.CreatedBy,
		RequestNumber: params.RequestNumber,
		Page:          int32(params.Page),
		PageSize:      int32(params.PageSize),
	}
	rows, err := s.read.ListVendorRequests(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("listing vendor requests: %w", err)
	}
	total, err := s.read.CountVendorRequests(ctx, db.CountVendorRequestsParams{
		Status:        arg.Status,
		ForecastDate:  arg.ForecastDate,
		CreatedBy:     arg.CreatedBy,
		RequestNumber: arg.RequestNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("counting vendor requests: %w", err)
	}

	data := make([]VendorRequestSummary, len(rows))
	for i, r := range rows {
		summary := VendorRequestSummary{
			ID:            r.ID,
			RequestNumber: r.RequestNumber,
			ForecastDate:  r.ForecastDate.Time,
			Status:        r.Status,
			Notes:         notesOrEmpty(r.Notes),
			ItemCount:     r.ItemCount,
			TotalAmount:   r.TotalAmount,
			CreatedBy:     UserRef{ID: r.CreatedByID, FullName: r.CreatedByName},
			CreatedAt:     r.CreatedAt.Time,
			SubmittedAt:   timestamptzToPtr(r.SubmittedAt),
			ApprovedAt:    timestamptzToPtr(r.ApprovedAt),
			RejectedAt:    timestamptzToPtr(r.RejectedAt),
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

	rows, err := s.read.ListAuditLogsByEntity(ctx, "vendor_request", id)
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
// draft header + items (Req 4, 15).
func (s *VendorRequestService) Create(ctx context.Context, actor Actor, in CreateVendorRequestInput) (*VendorRequestDetail, error) {
	if err := validateItemsPayload(in.Items, in.Notes, 0); err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	resolved, err := resolveItems(ctx, q, in.Items)
	if err != nil {
		return nil, err
	}

	forecastDate := toPgDate(in.ForecastDate)
	requestID, err := createWithRetryingNumber(ctx, q, in, forecastDate, actor)
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
		}); err != nil {
			return nil, fmt.Errorf("insert item %s: %w", r.input.TerminalID, err)
		}
		if r.overridden {
			overrides = append(overrides, r.input.TerminalID)
		}
	}

	metadata := map[string]any{"state": "draft"}
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

// createWithRetryingNumber generates VR-YYYYMMDD-NNNN and inserts the
// header, retrying up to 3 times on a request_number unique-constraint race
// (Req 15.2-15.5).
func createWithRetryingNumber(ctx context.Context, q *db.Queries, in CreateVendorRequestInput, forecastDate pgtype.Date, actor Actor) (int64, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		maxSeq, err := q.MaxRequestNumberSeqForDate(ctx, forecastDate)
		if err != nil {
			return 0, fmt.Errorf("get max request number seq: %w", err)
		}
		seq := maxSeq + int32(attempt) + 1
		if seq > 9999 {
			return 0, ErrNumberExhausted
		}
		requestNumber := fmt.Sprintf("VR-%s-%04d", in.ForecastDate.Format("20060102"), seq)

		request, err := q.CreateVendorRequest(ctx, db.CreateVendorRequestParams{
			RequestNumber: requestNumber,
			ForecastDate:  forecastDate,
			Notes:         stringPtrOrNil(in.Notes),
			CreatedBy:     actor.UserID,
		})
		if err == nil {
			return request.ID, nil
		}
		if !isUniqueViolation(err) {
			return 0, fmt.Errorf("create vendor request: %w", err)
		}
		lastErr = err
	}
	return 0, fmt.Errorf("%w: %v", ErrNumberGeneration, lastErr)
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

// Cancel: draft|pending_approval -> cancelled (Req 10, union rule — see
// checkActor's doc comment).
func (s *VendorRequestService) Cancel(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error) {
	return s.transition(ctx, actor, id, transitionOpts{action: actionCancel, auditAction: "cancel"})
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
		ID:              h.ID,
		RequestNumber:   h.RequestNumber,
		ForecastDate:    h.ForecastDate.Time,
		Status:          h.Status,
		Notes:           notesOrEmpty(h.Notes),
		CreatedBy:       UserRef{ID: h.CreatedBy, FullName: h.CreatedByName},
		RejectionReason: notesOrEmpty(h.RejectionReason),
		CreatedAt:       h.CreatedAt.Time,
		UpdatedAt:       h.UpdatedAt.Time,
		SubmittedAt:     timestamptzToPtr(h.SubmittedAt),
		ApprovedAt:      timestamptzToPtr(h.ApprovedAt),
		RejectedAt:      timestamptzToPtr(h.RejectedAt),
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
