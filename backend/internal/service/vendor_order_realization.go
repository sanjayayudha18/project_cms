package service

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// Laporan realisasi vendor (.claude/sdlc/replenishment-realisasi/spec.md FR2): the replenish party
// of a request that went through the vendor flow (vendor_sent) reports date + amount per denom per
// ATM. It replaces the internal laporan selesai for those requests (legacy requests keep the
// ATM-USER path, decision 2026-10-14) and runs the same state machine (runCompletionTx): maker =
// vendor user, checker = ATM-SPV/BRANCH-ATM-SPV, approve counts the visits as before.

// RealizationView is one ATM's vendor report as shown on both portals.
type RealizationView struct {
	TerminalID      string        `json:"terminal_id"`
	TicketNumber    string        `json:"ticket_number"`
	Result          *string       `json:"result"`
	RealizedDate    *time.Time    `json:"realized_date"`
	Amounts         []DenomAmount `json:"amounts"`
	SubmittedAt     *time.Time    `json:"submitted_at"`
	SubmittedByName string        `json:"submitted_by_name"`
}

// realizationViews folds the (ATM, denom) rows of ListVendorRealizationsForRequest per terminal.
func realizationViews(rows []db.ListVendorRealizationsForRequestRow) map[string]RealizationView {
	out := make(map[string]RealizationView, len(rows))
	for _, r := range rows {
		v, ok := out[r.TerminalID]
		if !ok {
			v = RealizationView{TerminalID: r.TerminalID, TicketNumber: r.TicketNumber, Result: r.Result,
				RealizedDate: dateToPtr(r.RealizedDate), Amounts: []DenomAmount{},
				SubmittedAt: timestamptzToPtr(r.SubmittedAt), SubmittedByName: r.SubmittedByName}
		}
		if r.Denom != nil && r.Amount != nil {
			v.Amounts = append(v.Amounts, DenomAmount{Denom: *r.Denom, Amount: *r.Amount})
		}
		out[r.TerminalID] = v
	}
	return out
}

func sortedRealizations(m map[string]RealizationView) []RealizationView {
	out := make([]RealizationView, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		out = append(out, m[k])
	}
	return out
}

// SubmitRealization: POST /api/v1/vendor/replenish-orders/{partyID}/realization (FR2.2-2.3).
func (s *VendorOrderService) SubmitRealization(ctx context.Context, va VendorActor, partyID int64, rows []RealizationRowInput) (*VendorOrderDetail, error) {
	q := db.New(s.pool)
	sc, err := s.scope(ctx, q, va)
	if err != nil {
		return nil, err
	}
	party, err := s.getScoped(ctx, q, sc, partyID)
	if err != nil {
		return nil, err
	}
	if party.Role != partyRoleReplenish || party.Status != "accepted" {
		return nil, ErrInvalidTransition // the vault party does not report (Q6d); withdrawn parties neither
	}
	err = runCompletionTx(ctx, s.pool, va.Actor, party.VendorRequestID, actionSubmitCompletion,
		func(q *db.Queries, req db.VendorRequest, newStatus string) (map[string]any, error) {
			if !req.VendorSent {
				return nil, ErrInvalidTransition
			}
			if err := checkReportsWholeRequest(ctx, q, sc, req.ID); err != nil {
				return nil, err
			}
			return s.applyRealization(ctx, q, va, req, newStatus, rows)
		})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, va, partyID)
}

// checkReportsWholeRequest: one submit covers every ATM of the request, so the user may only report
// when every live replenish party of the request is inside its scope (its vendor, and its branch for
// a branch-scoped user) — never ATMs another vendor or branch manages.
// ponytail: a multi-branch request needs a vendor-wide user; per-branch partial reports if that ever matters.
func checkReportsWholeRequest(ctx context.Context, q *db.Queries, sc vendorScope, requestID int64) error {
	parties, err := q.ListVendorPartiesForRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("list parties of request %d: %w", requestID, err)
	}
	for _, p := range parties {
		if p.Role != partyRoleReplenish || p.Status == "withdrawn" {
			continue
		}
		if p.VendorID != sc.vendorID || (sc.branchID != nil && p.VendorBranchID != *sc.branchID) {
			return &ValidationError{Field: "realizations", Message: "request ini mencakup ATM di luar cakupan user Anda; laporkan dengan user vendor tanpa batas branch dari vendor pengisi"}
		}
	}
	return nil
}

func (s *VendorOrderService) applyRealization(ctx context.Context, q *db.Queries, va VendorActor, req db.VendorRequest,
	newStatus string, rows []RealizationRowInput) (map[string]any, error) {
	orders, err := q.ListRequestTicketOrders(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("list ticket orders of %d: %w", req.ID, err)
	}
	tickets := make(map[string]TicketOrder)
	for _, o := range orders {
		t, ok := tickets[o.TerminalID]
		if !ok {
			t = TicketOrder{TicketID: o.TicketID, ScheduledDate: o.ReplenishDate.Time, Orders: map[int32]int64{}}
		}
		t.Orders[o.Denom] += o.OrderAmount
		tickets[o.TerminalID] = t
	}
	if err := validateRealization(tickets, rows, jakartaCalendarDate(0)); err != nil {
		return nil, err
	}

	report := make(map[string]any, len(rows))
	for _, r := range rows {
		ticketID, err := q.SetVendorRequestTicketResult(ctx, db.SetVendorRequestTicketResultParams{
			RequestNumber: req.RequestNumber, TerminalID: r.TerminalID, Result: r.Result,
		})
		if err != nil {
			return nil, fmt.Errorf("save result %s: %w", r.TerminalID, err)
		}
		date := optDate(r.RealizedDate)
		realizationID, err := q.UpsertVendorRealization(ctx, db.UpsertVendorRealizationParams{
			TicketID: ticketID, VendorRequestID: req.ID, RealizedDate: date, SubmittedBy: va.UserID,
		})
		if err != nil {
			return nil, fmt.Errorf("save realization %s: %w", r.TerminalID, err)
		}
		if err := q.DeleteVendorRealizationAmounts(ctx, realizationID); err != nil {
			return nil, fmt.Errorf("reset realization amounts %s: %w", r.TerminalID, err)
		}
		for _, denom := range slices.Sorted(maps.Keys(r.Amounts)) {
			if err := q.InsertVendorRealizationAmount(ctx, db.InsertVendorRealizationAmountParams{
				RealizationID: realizationID, Denom: denom, Amount: r.Amounts[denom],
			}); err != nil {
				return nil, fmt.Errorf("save realization amount %s/%d: %w", r.TerminalID, denom, err)
			}
		}
		report[r.TerminalID] = map[string]any{"result": r.Result, "realized_date": dateToPtr(date), "amounts": r.Amounts}
	}
	if _, err := q.UpdateVendorRequestCompletion(ctx, db.UpdateVendorRequestCompletionParams{
		Status: newStatus, ActorID: va.UserID, ID: req.ID,
	}); err != nil {
		return nil, fmt.Errorf("update vendor request %d: %w", req.ID, err)
	}
	if s.notifier != nil {
		id := req.ID
		if err := s.notifier.Send(ctx, q, notification.Message{
			Type: "vendor_request.realization_submitted", Title: "Laporan realisasi vendor masuk",
			Body: fmt.Sprintf("Vendor melaporkan realisasi request %s. Mohon periksa dan setujui.", req.RequestNumber),
			Link: fmt.Sprintf("/replenishment/vendor-requests/%d", id), EntityType: "vendor_request", EntityID: &id, Email: true,
		}, notification.Recipients{Roles: []string{"ATM-SPV", "BRANCH-ATM-SPV"}}); err != nil {
			return nil, fmt.Errorf("notify realization submitted: %w", err)
		}
	}
	return map[string]any{"realizations": report}, nil
}

// notifyRealizationDecision tells the vendor user who reported that the report was approved
// (reason "") or rejected (FR2.8). No-op for legacy requests (internal maker) or without notifier.
func notifyRealizationDecision(ctx context.Context, q *db.Queries, n Notifier, req db.VendorRequest, reason string) error {
	if n == nil || !req.VendorSent || req.CompletionSubmittedBy == nil {
		return nil
	}
	msg := notification.Message{Type: "vendor_order.realization_approved", Title: "Laporan realisasi disetujui",
		Body: fmt.Sprintf("Laporan realisasi request %s disetujui CIMB Niaga.", req.RequestNumber), Email: true}
	if reason != "" {
		msg.Type, msg.Title = "vendor_order.realization_rejected", "Laporan realisasi ditolak"
		msg.Body = fmt.Sprintf("Laporan realisasi request %s ditolak: %s. Mohon lapor ulang.", req.RequestNumber, reason)
	}
	parties, err := q.ListVendorPartiesForRequest(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("list parties of request %d: %w", req.ID, err)
	}
	for _, p := range parties {
		if p.Role == partyRoleReplenish && p.Status != "withdrawn" {
			id := p.ID
			msg.Link, msg.EntityType, msg.EntityID = fmt.Sprintf("/orders/%d", id), vendorPartyEntity, &id
			break
		}
	}
	if err := n.Send(ctx, q, msg, notification.Recipients{UserIDs: []int64{*req.CompletionSubmittedBy}}); err != nil {
		return fmt.Errorf("notify realization decision: %w", err)
	}
	return nil
}

// loadRealization fills the replenish party's report + whether it may report now (FR5.1).
func loadRealization(ctx context.Context, q *db.Queries, d *VendorOrderDetail, requestID int64) error {
	d.Realizations = []RealizationView{}
	if d.Role != partyRoleReplenish {
		return nil
	}
	req, err := q.GetVendorRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("load vendor request %d: %w", requestID, err)
	}
	rows, err := q.ListVendorRealizationsForRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("list realizations of %d: %w", requestID, err)
	}
	d.Realizations = sortedRealizations(realizationViews(rows))
	d.CompletionRejectionReason = req.CompletionRejectionReason
	d.CanReport = req.VendorSent && !req.IsCanceled && req.Status == "vendor_accepted"
	return nil
}
