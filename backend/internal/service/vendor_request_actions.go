package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

const pgUniqueViolation = "23505"
const pgCheckViolation = "23514"

var validVendorRequestStatuses = []string{
	"draft", "pending_approval", "approved", "rejected", "processing", "completed", "cancelled",
	"completion_pending",                        // atm-visit-quota (migration 021)
	"vault_assignment", "vault_review", "ready", // cit-acm-plan (migration 027)
	"sent_to_vendor", "vendor_accepted", "vendor_rejected", // cit-send-vendor (migration 029)
}

// --- Mutations (each runs inside one DB transaction) --------------------

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
	// replenish-ticket FR10.4: a request with a region code keeps it; old
	// requests (region_code NULL) are not checked, as before.
	if req.RegionCode != nil && req.VendorID != nil {
		atms, err := validateItemsSingleVendor(ctx, q, *req.VendorID, resolved)
		if err != nil {
			return nil, err
		}
		if _, err := singleRegion(atms, *req.RegionCode); err != nil {
			return nil, err
		}
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

	existing, err := q.ListVendorRequestTickets(ctx, req.RequestNumber)
	if err != nil {
		return nil, fmt.Errorf("list tickets for %d: %w", id, err)
	}
	tickets, err := applyTicketPlan(ctx, q, req.RequestNumber, ticketDate(req), planTickets(existing, wantedCodes(resolved)))
	if err != nil {
		return nil, err
	}

	metadata := map[string]any{"state": req.Status, "items_added": len(resolved), "items_removed": 0, "items_modified": 0}
	if len(overrides) > 0 {
		metadata["amount_overrides"] = overrides
	}
	tickets.addTo(metadata)
	// cit-vault-select: vault allocations are per request, not per ATM; an edit
	// keeps them and Submit re-checks the total against the new order.
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

// Approve: pending_approval -> vault_assignment (Req 6; cit-acm-plan FR4.1:
// also creates the ACM area plans and notifies ACM-USER, same tx).
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

// VendorReturn: vendor_rejected -> rejected (cit-send-vendor FR6.1), same
// reason bounds and four-eyes as Reject; the maker then revises and edits.
func (s *VendorRequestService) VendorReturn(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error) {
	trimmed, err := checkReason(reason, "rejection_reason")
	if err != nil {
		return nil, err
	}
	return s.transition(ctx, actor, id, transitionOpts{action: actionVendorReturn, auditAction: "vendor_return", rejectionReason: trimmed})
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
	// cit-acm-plan FR5.5: the ACM area plans die with the request.
	if err := q.CancelVaultPlansForRequest(ctx, id); err != nil {
		return nil, fmt.Errorf("cancel vault plans of %d: %w", id, err)
	}
	// cit-send-vendor FR6.4: vendors already reached are told; parties keep their status (history).
	if err := cancelVendorParties(ctx, q, s.notifier, actor, req); err != nil {
		return nil, err
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
	if opts.action == actionReject || opts.action == actionVendorReturn {
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
	if opts.action == actionReject || opts.action == actionVendorReturn {
		after["rejection_reason"] = opts.rejectionReason // Req 16.2
	}
	aw := newAuditWriter(tx)
	if err := aw.Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: opts.auditAction, EntityType: "vendor_request", EntityID: id,
		Before: map[string]string{"state": req.Status}, After: after, IP: actor.IP,
	}); err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}
	if newStatus == "vault_assignment" && opts.action == actionApprove {
		// cit-send-vendor FR6.2: a request re-approved after an edit keeps its
		// area plans; they go back to draft for ACM to review again.
		if err := resetVaultPlansForReapproval(ctx, q, aw, s.notifier, actor, req); err != nil {
			return nil, err
		}
		if err := createVaultPlans(ctx, q, aw, s.notifier, actor, &id); err != nil {
			return nil, err
		}
	}
	if opts.action == actionVendorReturn && s.notifier != nil {
		if err := s.notifier.Send(ctx, q, notification.Message{
			Type: "vendor_request.vendor_returned", Title: "Request dikembalikan untuk revisi",
			Body: fmt.Sprintf("Request %s ditolak branch replenish dan dikembalikan: %s", req.RequestNumber, opts.rejectionReason),
			Link: fmt.Sprintf("/replenishment/vendor-requests/%d", id), EntityType: "vendor_request", EntityID: &id, Email: true,
		}, notification.Recipients{UserIDs: []int64{req.CreatedBy}}); err != nil {
			return nil, fmt.Errorf("notify vendor return: %w", err)
		}
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
		RegionCode:         h.RegionCode,
	}
	if h.ApprovedBy != nil {
		detail.ApprovedBy = &UserRef{ID: *h.ApprovedBy, FullName: notesOrEmpty(h.ApprovedByName)}
	}
	if h.RejectedBy != nil {
		detail.RejectedBy = &UserRef{ID: *h.RejectedBy, FullName: notesOrEmpty(h.RejectedByName)}
	}
	detail.CompletionSubmittedBy = userRefOrNil(h.CompletionSubmittedBy, h.CompletionSubmittedByName)
	detail.CompletionSubmittedAt = timestamptzToPtr(h.CompletionSubmittedAt)
	detail.CompletionApprovedBy = userRefOrNil(h.CompletionApprovedBy, h.CompletionApprovedByName)
	detail.CompletionApprovedAt = timestamptzToPtr(h.CompletionApprovedAt)
	detail.CompletionRejectedBy = userRefOrNil(h.CompletionRejectedBy, h.CompletionRejectedByName)
	detail.CompletionRejectedAt = timestamptzToPtr(h.CompletionRejectedAt)
	detail.CompletionRejectionReason = h.CompletionRejectionReason

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
