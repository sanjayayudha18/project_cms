package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// Laporan selesai replenish (.claude/sdlc/atm-visit-quota/spec.md FR1):
// approved -> completion_pending (maker reports per-ATM success/failed) ->
// completed (checker approves: each successful ATM counts one visit against
// its kuota) or back to approved (checker rejects, maker may resubmit).
// These don't go through transition(): checkActor's four-eyes rule compares
// against the order's creator, while here the checker must differ from the
// report's maker, and approve writes visits in the same tx (plan D2).

// completionMakerRoles: who may report a request as replenished (intent #4:
// internal ATM-USER, not the vendor). Mirrors handler.vendorRequestMakerRoles.
var completionMakerRoles = []string{"ADMIN", "ATM-USER", "BRANCH-ATM-USER"}

// ErrCompletionReasonEmpty: reject-report reason missing (body field "reason",
// distinct from the order's ErrRejectReasonEmpty / "rejection_reason").
var ErrCompletionReasonEmpty = errors.New("completion rejection reason is required")

const (
	completionSuccess = "success"
	completionFailed  = "failed"
)

// CompletionResultInput is one ATM's outcome in a laporan selesai.
type CompletionResultInput struct {
	TerminalID string
	Result     string // "success" | "failed"
}

func checkCompletionActor(actor Actor, req db.VendorRequest, a action) error {
	switch a {
	case actionSubmitCompletion:
		if !roleIn(actor.Role, completionMakerRoles) {
			return ErrNotAuthorized
		}
	case actionApproveCompletion, actionRejectCompletion:
		if !isChecker(actor.Role) {
			return ErrNotChecker
		}
		if req.CompletionSubmittedBy != nil && *req.CompletionSubmittedBy == actor.UserID {
			return ErrSelfApproval
		}
	}
	return nil
}

// validateCompletionResults requires exactly one result for every distinct
// terminal of the request and nothing else (FR1.1).
func validateCompletionResults(terminals []string, results []CompletionResultInput) error {
	want := make(map[string]bool, len(terminals))
	for _, t := range terminals {
		want[t] = true
	}
	seen := make(map[string]bool, len(results))
	for _, r := range results {
		if r.Result != completionSuccess && r.Result != completionFailed {
			return &ValidationError{Field: "results", Message: fmt.Sprintf("hasil ATM %s harus success atau failed", r.TerminalID)}
		}
		if !want[r.TerminalID] {
			return &ValidationError{Field: "results", Message: fmt.Sprintf("ATM %s bukan bagian dari request ini", r.TerminalID)}
		}
		if seen[r.TerminalID] {
			return &ValidationError{Field: "results", Message: fmt.Sprintf("ATM %s muncul lebih dari sekali", r.TerminalID)}
		}
		seen[r.TerminalID] = true
	}
	if len(seen) != len(want) {
		return &ValidationError{Field: "results", Message: "setiap ATM di request wajib diberi hasil berhasil/gagal"}
	}
	return nil
}

// completionTx is the shared tx -> lock -> state guard -> actor guard ->
// apply -> audit -> commit flow of the three completion actions. apply
// returns the extra audit "after" fields.
func (s *VendorRequestService) completionTx(ctx context.Context, actor Actor, id int64, a action,
	apply func(q *db.Queries, req db.VendorRequest, newStatus string) (map[string]any, error)) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	req, err := q.GetVendorRequestForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load vendor request %d: %w", id, err)
	}
	if req.IsCanceled {
		return ErrInvalidTransition
	}
	newStatus, ok := nextState(req.Status, a)
	if !ok {
		return ErrInvalidTransition
	}
	if err := checkCompletionActor(actor, req, a); err != nil {
		return err
	}

	after, err := apply(q, req, newStatus)
	if err != nil {
		return err
	}
	after["state"] = newStatus
	if err := newAuditWriter(tx).Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: string(a), EntityType: "vendor_request", EntityID: id,
		Before: map[string]string{"state": req.Status}, After: after, IP: actor.IP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit vendor request %d %s: %w", id, a, err)
	}
	return nil
}

// SubmitCompletion: approved -> completion_pending with per-ATM results.
// A resubmission after rejection overwrites the previous results (FR1.1).
func (s *VendorRequestService) SubmitCompletion(ctx context.Context, actor Actor, id int64, results []CompletionResultInput) (*VendorRequestDetail, error) {
	err := s.completionTx(ctx, actor, id, actionSubmitCompletion, func(q *db.Queries, req db.VendorRequest, newStatus string) (map[string]any, error) {
		terminals, err := q.ListDistinctRequestTerminals(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("list terminals of %d: %w", id, err)
		}
		if err := validateCompletionResults(terminals, results); err != nil {
			return nil, err
		}
		byTerminal := make(map[string]string, len(results))
		for _, r := range results {
			// replenish-ticket FR9.2: the result lives on the ATM's active ticket.
			if _, err := q.SetVendorRequestTicketResult(ctx, db.SetVendorRequestTicketResultParams{
				RequestNumber: req.RequestNumber, TerminalID: r.TerminalID, Result: r.Result,
			}); err != nil {
				return nil, fmt.Errorf("save result %s: %w", r.TerminalID, err)
			}
			byTerminal[r.TerminalID] = r.Result
		}
		if _, err := q.UpdateVendorRequestCompletion(ctx, db.UpdateVendorRequestCompletionParams{
			Status: newStatus, ActorID: actor.UserID, ID: id,
		}); err != nil {
			return nil, fmt.Errorf("update vendor request %d: %w", id, err)
		}
		return map[string]any{"results": byTerminal}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// ApproveCompletion: completion_pending -> completed. Every "success" ATM
// records one visit (recordVisit: idempotent per request+ATM, atomic quota
// decrement) in the same tx. Returns the terminals that went over quota so
// the checker sees the warning (FR6.2); over quota never blocks (intent #10).
// Over quota also notifies the report's maker + every active ATM-SPV
// (notification spec FR6), in the same tx: a failed Send rolls back the approve.
func (s *VendorRequestService) ApproveCompletion(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, []string, error) {
	overQuota := []string{}
	err := s.completionTx(ctx, actor, id, actionApproveCompletion, func(q *db.Queries, req db.VendorRequest, newStatus string) (map[string]any, error) {
		results, err := q.ListActiveVendorRequestTickets(ctx, req.RequestNumber)
		if err != nil {
			return nil, fmt.Errorf("list results of %d: %w", id, err)
		}
		today := jakartaCalendarDate(0)
		visits := make([]visitOutcome, 0, len(results))
		for _, r := range results {
			if r.Result == nil || *r.Result != completionSuccess {
				continue
			}
			out, err := recordVisit(ctx, q, id, r.TerminalID, today)
			if err != nil {
				return nil, err
			}
			if out.OverQuota {
				overQuota = append(overQuota, out.TerminalID)
			}
			visits = append(visits, out)
		}
		if len(overQuota) > 0 && s.notifier != nil {
			msg, to := overQuotaNotification(req, overQuota)
			if err := s.notifier.Send(ctx, q, msg, to); err != nil {
				return nil, fmt.Errorf("notify over quota for %d: %w", id, err)
			}
		}
		if _, err := q.UpdateVendorRequestCompletion(ctx, db.UpdateVendorRequestCompletionParams{
			Status: newStatus, ActorID: actor.UserID, ID: id,
		}); err != nil {
			return nil, fmt.Errorf("update vendor request %d: %w", id, err)
		}
		return map[string]any{"visits": visits, "over_quota_terminals": overQuota}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	detail, err := s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return detail, overQuota, nil
}

// RejectCompletion: completion_pending -> approved with a reason (1-500
// chars, same bound as Reject). No visits, kuota untouched (FR1.3).
func (s *VendorRequestService) RejectCompletion(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil, ErrCompletionReasonEmpty
	}
	if len(trimmed) > 500 {
		return nil, &ValidationError{Field: "reason", Message: "maksimal 500 karakter"}
	}
	err := s.completionTx(ctx, actor, id, actionRejectCompletion, func(q *db.Queries, _ db.VendorRequest, newStatus string) (map[string]any, error) {
		if _, err := q.UpdateVendorRequestCompletion(ctx, db.UpdateVendorRequestCompletionParams{
			Status: newStatus, ActorID: actor.UserID, Reason: &trimmed, ID: id,
		}); err != nil {
			return nil, fmt.Errorf("update vendor request %d: %w", id, err)
		}
		return map[string]any{"reason": trimmed}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// requestAtmStatuses merges the report results with current kuota per
// distinct terminal of the request (detail screen, FR6.2/6.3); results and
// ticket numbers come from the active tickets (replenish-ticket FR9.3).
func (s *VendorRequestService) requestAtmStatuses(ctx context.Context, id int64, tickets []db.ListActiveVendorRequestTicketsRow) ([]RequestAtmStatus, error) {
	byTerminal := make(map[string]db.ListActiveVendorRequestTicketsRow, len(tickets))
	for _, t := range tickets {
		byTerminal[t.TerminalID] = t
	}
	rows, err := s.read.ListRequestVisitInfo(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list visit info of %d: %w", id, err)
	}
	out := make([]RequestAtmStatus, len(rows))
	for i, r := range rows {
		out[i] = RequestAtmStatus{
			TerminalID:      r.TerminalID,
			VisitRemaining:  r.VisitRemaining,
			VisitQuotaTotal: r.VisitQuotaTotal,
			IsOverQuota:     r.IsOverQuota,
		}
		if t, ok := byTerminal[r.TerminalID]; ok {
			out[i].TicketNumber = t.TicketNumber
			out[i].CompletionResult = t.Result
		}
	}
	return out, nil
}

func userRefOrNil(id *int64, name *string) *UserRef {
	if id == nil {
		return nil
	}
	return &UserRef{ID: *id, FullName: notesOrEmpty(name)}
}

// overQuotaListMax caps the terminals named in the notification body so it
// stays under the 1000-character limit however many ATMs went over quota.
const overQuotaListMax = 20

// overQuotaNotification builds the "kelebihan kuota" notification (spec
// FR6.1, S1/S2): maker of the report + all active ATM-SPV, in-app + email.
func overQuotaNotification(req db.VendorRequest, terminals []string) (notification.Message, notification.Recipients) {
	listed := terminals
	suffix := ""
	if len(terminals) > overQuotaListMax {
		listed = terminals[:overQuotaListMax]
		suffix = fmt.Sprintf(" dan %d ATM lainnya", len(terminals)-overQuotaListMax)
	}
	id := req.ID
	msg := notification.Message{
		Type:       "visit_quota.over_quota",
		Title:      "Kelebihan kuota kunjungan",
		Body:       fmt.Sprintf("Vendor request %s: ATM %s%s melebihi kuota kunjungan replenish.", req.RequestNumber, strings.Join(listed, ", "), suffix),
		Link:       fmt.Sprintf("/replenishment/vendor-requests/%d", req.ID),
		EntityType: "vendor_request",
		EntityID:   &id,
		Email:      true,
	}
	to := notification.Recipients{Roles: []string{"ATM-SPV"}}
	if req.CompletionSubmittedBy != nil {
		to.UserIDs = []int64{*req.CompletionSubmittedBy}
	}
	return msg, to
}
