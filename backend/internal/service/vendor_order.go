package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// Phase 2.2b (.claude/sdlc/cit-send-vendor) FR4/FR5: the VendorPortal side.
// A vendor user only ever reaches parties of branches in its scope: its
// vendor, narrowed to users.vendor_branch_id when set. Scope is read from the
// primary per call (FR5.2) and applied in SQL; anything outside it is
// ErrVendorOrderNotFound (404), never 403 (FR5.3).

const vendorUserRole = "VENDOR-USER"

var ErrVendorOrderNotFound = errors.New("vendor order not found")

const (
	vendorOrderMaxPageSize = 100
	vendorRejectMinLen     = 10
	vendorRejectMaxLen     = 500
)

// VendorActor is the vendor caller: Actor plus the vendor_id JWT claim.
type VendorActor struct {
	Actor
	ClaimVendorID *int64
}

type vendorScope struct {
	vendorID int64
	branchID *int64
}

type VendorOrderService struct {
	pool     *pgxpool.Pool
	read     *db.Queries
	notifier Notifier
}

// NewVendorOrderService: list on the replica (NFR2), scope/detail/actions on the primary.
func NewVendorOrderService(pool, readPool *pgxpool.Pool) *VendorOrderService {
	return &VendorOrderService{pool: pool, read: db.New(readPool)}
}

func (s *VendorOrderService) WithNotifier(n Notifier) *VendorOrderService {
	s.notifier = n
	return s
}

// VendorOrderSummary is one party row of the vendor list (FR9.1).
type VendorOrderSummary struct {
	ID            int64         `json:"id"`
	Role          string        `json:"role"`
	Status        string        `json:"status"`
	RequestNumber string        `json:"request_number"`
	ReplenishDate *time.Time    `json:"replenish_date"`
	RequestStatus string        `json:"request_status"`
	IsCanceled    bool          `json:"is_canceled"`
	BranchID      int64         `json:"branch_id"`
	BranchCode    string        `json:"branch_code"`
	BranchName    string        `json:"branch_name"`
	AtmCount      int           `json:"atm_count"`
	Totals        []DenomAmount `json:"totals"`
	Currency      string        `json:"currency"`
	SentAt        *time.Time    `json:"sent_at"`
	DecidedAt     *time.Time    `json:"decided_at"`
	CanDecide     bool          `json:"can_decide"`
}

// VendorOrderDetail adds the full content (FR3) and the vendor's own rejection reason.
type VendorOrderDetail struct {
	VendorOrderSummary
	RejectionReason *string      `json:"rejection_reason"`
	Content         PartyContent `json:"content"`
}

type VendorOrderFilter struct {
	PartyStatus   string
	RequestStatus string
	From, To      *time.Time
	Page          int
	PageSize      int
}

type VendorOrderList struct {
	Items    []VendorOrderSummary `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// scope resolves the caller's vendor + branch from users (FR5.1/FR5.2). A
// non-vendor role, a user without vendor, or a claim that no longer matches
// the row is not authorized.
func (s *VendorOrderService) scope(ctx context.Context, q *db.Queries, va VendorActor) (vendorScope, error) {
	if va.Role != vendorUserRole {
		return vendorScope{}, ErrNotAuthorized
	}
	row, err := q.GetUserVendorScope(ctx, va.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return vendorScope{}, ErrNotAuthorized
	}
	if err != nil {
		return vendorScope{}, fmt.Errorf("load vendor scope of user %d: %w", va.UserID, err)
	}
	if row.VendorID == nil || va.ClaimVendorID == nil || *row.VendorID != *va.ClaimVendorID {
		return vendorScope{}, ErrNotAuthorized
	}
	return vendorScope{vendorID: *row.VendorID, branchID: row.VendorBranchID}, nil
}

func (s *VendorOrderService) List(ctx context.Context, va VendorActor, f VendorOrderFilter) (*VendorOrderList, error) {
	sc, err := s.scope(ctx, db.New(s.pool), va)
	if err != nil {
		return nil, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > vendorOrderMaxPageSize {
		f.PageSize = 20
	}
	from, to := optDate(f.From), optDate(f.To)
	rows, err := s.read.ListVendorOrders(ctx, db.ListVendorOrdersParams{
		VendorID: sc.vendorID, VendorBranchID: sc.branchID, PartyStatus: f.PartyStatus, RequestStatus: f.RequestStatus,
		FromDate: from, ToDate: to, PageOffset: int32((f.Page - 1) * f.PageSize), PageLimit: int32(f.PageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("list vendor orders: %w", err)
	}
	total, err := s.read.CountVendorOrders(ctx, db.CountVendorOrdersParams{
		VendorID: sc.vendorID, VendorBranchID: sc.branchID, PartyStatus: f.PartyStatus, RequestStatus: f.RequestStatus,
		FromDate: from, ToDate: to,
	})
	if err != nil {
		return nil, fmt.Errorf("count vendor orders: %w", err)
	}
	items := make([]VendorOrderSummary, 0, len(rows))
	for _, r := range rows {
		var c PartyContent
		if err := json.Unmarshal(r.Content, &c); err != nil {
			return nil, fmt.Errorf("decode party %d content: %w", r.ID, err)
		}
		items = append(items, summary(r.ID, r.Role, r.Status, r.RequestNumber, r.ReplenishDate, r.RequestStatus, r.IsCanceled,
			r.VendorBranchID, r.BranchCode, r.BranchName, r.SentAt, r.DecidedAt, c))
	}
	return &VendorOrderList{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (s *VendorOrderService) Get(ctx context.Context, va VendorActor, partyID int64) (*VendorOrderDetail, error) {
	q := db.New(s.pool)
	sc, err := s.scope(ctx, q, va)
	if err != nil {
		return nil, err
	}
	row, err := s.getScoped(ctx, q, sc, partyID)
	if err != nil {
		return nil, err
	}
	var c PartyContent
	if err := json.Unmarshal(row.Content, &c); err != nil {
		return nil, fmt.Errorf("decode party %d content: %w", row.ID, err)
	}
	return &VendorOrderDetail{
		VendorOrderSummary: summary(row.ID, row.Role, row.Status, row.RequestNumber, row.ReplenishDate, row.RequestStatus,
			row.IsCanceled, row.VendorBranchID, row.BranchCode, row.BranchName, row.SentAt, row.DecidedAt, c),
		RejectionReason: row.RejectionReason,
		Content:         c,
	}, nil
}

func (s *VendorOrderService) getScoped(ctx context.Context, q *db.Queries, sc vendorScope, partyID int64) (db.GetVendorOrderRow, error) {
	row, err := q.GetVendorOrder(ctx, db.GetVendorOrderParams{ID: partyID, VendorID: sc.vendorID, VendorBranchID: sc.branchID})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrVendorOrderNotFound
	}
	if err != nil {
		return row, fmt.Errorf("get vendor order %d: %w", partyID, err)
	}
	return row, nil
}

// Accept: FR4.2.
func (s *VendorOrderService) Accept(ctx context.Context, va VendorActor, partyID int64) (*VendorOrderDetail, error) {
	if err := s.decide(ctx, va, partyID, nil); err != nil {
		return nil, err
	}
	return s.Get(ctx, va, partyID)
}

// Reject: FR4.3 (vault) / FR4.4 (replenish), reason 10-500 chars.
func (s *VendorOrderService) Reject(ctx context.Context, va VendorActor, partyID int64, reason string) (*VendorOrderDetail, error) {
	r := strings.TrimSpace(reason)
	if n := len([]rune(r)); n < vendorRejectMinLen || n > vendorRejectMaxLen {
		return nil, &ValidationError{Field: "reason", Message: fmt.Sprintf("alasan %d-%d karakter", vendorRejectMinLen, vendorRejectMaxLen)}
	}
	if err := s.decide(ctx, va, partyID, &r); err != nil {
		return nil, err
	}
	return s.Get(ctx, va, partyID)
}

// decide records one party decision and moves the request (FR4). Lock order:
// request, then party (NFR3), so the "last accept" transition happens once.
func (s *VendorOrderService) decide(ctx context.Context, va VendorActor, partyID int64, reason *string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	sc, err := s.scope(ctx, q, va)
	if err != nil {
		return err
	}
	scoped, err := s.getScoped(ctx, q, sc, partyID)
	if err != nil {
		return err
	}
	req, err := q.GetVendorRequestForUpdate(ctx, scoped.VendorRequestID)
	if err != nil {
		return fmt.Errorf("lock vendor request %d: %w", scoped.VendorRequestID, err)
	}
	party, err := q.LockVendorParty(ctx, partyID)
	if err != nil {
		return fmt.Errorf("lock party %d: %w", partyID, err)
	}
	if req.IsCanceled || req.Status != "sent_to_vendor" || party.Status != "pending" {
		return ErrInvalidTransition
	}

	status := "accepted"
	if reason != nil {
		status = "rejected"
	}
	decided, err := q.DecideVendorParty(ctx, db.DecideVendorPartyParams{Status: status, DecidedBy: va.UserID, RejectionReason: reason, ID: partyID})
	if err != nil {
		return fmt.Errorf("decide party %d: %w", partyID, err)
	}
	if err := q.InsertVendorPartyEvent(ctx, db.InsertVendorPartyEventParams{
		PartyID: partyID, Event: status, ActorID: va.UserID, Reason: reason, Content: decided.Content,
	}); err != nil {
		return fmt.Errorf("insert party event: %w", err)
	}
	after := map[string]any{"status": status, "role": party.Role, "vendor_branch_id": party.VendorBranchID}
	if reason != nil {
		after["rejection_reason"] = *reason
	}
	aw := audit.NewWriter(tx)
	if err := aw.Write(ctx, audit.Entry{ActorID: va.UserID, Action: "vendor_" + status, EntityType: vendorPartyEntity, EntityID: partyID,
		Before: map[string]string{"status": party.Status}, After: after, IP: va.IP}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	switch {
	case reason == nil:
		err = s.afterAccept(ctx, q, aw, va, req)
	case party.Role == partyRoleVault:
		err = s.afterVaultReject(ctx, q, aw, va, req, party, *reason)
	default:
		err = s.moveRequest(ctx, q, aw, va, req, "vendor_rejected", map[string]any{"rejected_party_id": partyID, "rejection_reason": *reason},
			notification.Message{Type: "vendor_order.replenish_rejected", Title: "Request ditolak branch replenish",
				Body: fmt.Sprintf("Request %s ditolak %s: %s", req.RequestNumber, scoped.BranchName, *reason)},
			notification.Recipients{Roles: []string{"ATM-SPV", "BRANCH-ATM-SPV"}, UserIDs: []int64{req.CreatedBy}})
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// afterAccept: every non-withdrawn party accepted -> vendor_accepted (FR4.2).
func (s *VendorOrderService) afterAccept(ctx context.Context, q *db.Queries, aw *audit.Writer, va VendorActor, req db.VendorRequest) error {
	parties, err := q.LockVendorPartiesForRequest(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("lock parties of request %d: %w", req.ID, err)
	}
	for _, p := range parties {
		if p.Status != "accepted" && p.Status != "withdrawn" {
			return nil
		}
	}
	return s.moveRequest(ctx, q, aw, va, req, "vendor_accepted", nil,
		notification.Message{Type: "vendor_order.all_accepted", Title: "Request diterima semua vendor",
			Body: fmt.Sprintf("Request %s sudah diterima semua branch vendor.", req.RequestNumber)},
		notification.Recipients{Roles: []string{"ATM-SPV"}, UserIDs: []int64{req.CreatedBy}})
}

// afterVaultReject: request back to vault_assignment, only the area plans
// holding that vault go back to draft (FR4.3), ACM of those areas notified.
// SetVendorRequestStatusOnly keeps approved_at, so the later resend is not
// mistaken for a re-approval (reapprovedSinceSent).
func (s *VendorOrderService) afterVaultReject(ctx context.Context, q *db.Queries, aw *audit.Writer, va VendorActor,
	req db.VendorRequest, party db.VendorRequestVendorParty, reason string) error {
	plans, err := q.ReturnVaultPlansForVaultBranch(ctx, db.ReturnVaultPlansForVaultBranchParams{
		ActorID: va.UserID, Reason: reason, RequestID: req.ID, VaultBranchID: party.VendorBranchID,
	})
	if err != nil {
		return fmt.Errorf("return plans of request %d: %w", req.ID, err)
	}
	if err := q.SetVendorRequestStatusOnly(ctx, db.SetVendorRequestStatusOnlyParams{Status: "vault_assignment", ID: req.ID}); err != nil {
		return fmt.Errorf("move request %d: %w", req.ID, err)
	}
	planIDs := make([]int64, 0, len(plans))
	for _, p := range plans {
		planIDs = append(planIDs, p.ID)
	}
	if err := aw.Write(ctx, audit.Entry{ActorID: va.UserID, Action: "vendor_vault_reject", EntityType: "vendor_request", EntityID: req.ID,
		Before: map[string]string{"state": req.Status},
		After:  map[string]any{"state": "vault_assignment", "rejected_party_id": party.ID, "plans_to_draft": planIDs, "rejection_reason": reason},
		IP:     va.IP}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	if s.notifier == nil {
		return nil
	}
	for _, p := range plans {
		var users []int64
		for _, role := range []string{"ACM-USER", "ACM-SPV"} {
			ids, err := q.ListAcmAreaMemberIDsByRole(ctx, db.ListAcmAreaMemberIDsByRoleParams{AreaIds: []int64{p.AcmAreaID}, Role: role})
			if err != nil {
				return fmt.Errorf("list acm area members: %w", err)
			}
			users = append(users, ids...)
		}
		if len(users) == 0 {
			continue
		}
		planID := p.ID
		if err := s.notifier.Send(ctx, q, notification.Message{
			Type: "vendor_order.vault_rejected", Title: "Vault ditolak vendor",
			Body: fmt.Sprintf("Branch vault menolak request %s: %s. Pilih vault lain.", req.RequestNumber, reason),
			Link: fmt.Sprintf("/cit/vault-plans/%d", planID), EntityType: vaultPlanEntity, EntityID: &planID, Email: true,
		}, notification.Recipients{UserIDs: users}); err != nil {
			return fmt.Errorf("notify vault reject: %w", err)
		}
	}
	return nil
}

// moveRequest sets a request status reached by a vendor decision, audits it
// and notifies internal users (FR7.2) with a link to the request.
func (s *VendorOrderService) moveRequest(ctx context.Context, q *db.Queries, aw *audit.Writer, va VendorActor, req db.VendorRequest,
	status string, extra map[string]any, msg notification.Message, to notification.Recipients) error {
	if _, err := q.UpdateVendorRequestStatus(ctx, db.UpdateVendorRequestStatusParams{Status: status, ID: req.ID}); err != nil {
		return fmt.Errorf("move request %d to %s: %w", req.ID, status, err)
	}
	after := map[string]any{"state": status}
	for k, v := range extra {
		after[k] = v
	}
	if err := aw.Write(ctx, audit.Entry{ActorID: va.UserID, Action: "vendor_" + status, EntityType: "vendor_request", EntityID: req.ID,
		Before: map[string]string{"state": req.Status}, After: after, IP: va.IP}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	if s.notifier == nil {
		return nil
	}
	id := req.ID
	msg.Link, msg.EntityType, msg.EntityID, msg.Email = fmt.Sprintf("/replenishment/vendor-requests/%d", id), "vendor_request", &id, true
	if err := s.notifier.Send(ctx, q, msg, to); err != nil {
		return fmt.Errorf("notify %s: %w", msg.Type, err)
	}
	return nil
}

func summary(id int64, role, status, number string, date pgtype.Date, reqStatus string, canceled bool,
	branchID int64, code, name string, sent, decided pgtype.Timestamptz, c PartyContent) VendorOrderSummary {
	return VendorOrderSummary{
		ID: id, Role: role, Status: status, RequestNumber: number, ReplenishDate: dateToPtr(date),
		RequestStatus: reqStatus, IsCanceled: canceled, BranchID: branchID, BranchCode: code, BranchName: name,
		AtmCount: len(c.Atms), Totals: c.Totals, Currency: currencyIDR,
		SentAt: timestamptzToPtr(sent), DecidedAt: timestamptzToPtr(decided),
		CanDecide: status == "pending" && reqStatus == "sent_to_vendor" && !canceled,
	}
}

func optDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return toPgDate(*t)
}

// RequestVendorParty / RequestVendorPartyEvent: the internal "Status Vendor"
// panel of a Vendor Request (FR6.6). Internal users see every party.
type RequestVendorParty struct {
	ID              int64      `json:"id"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	BranchID        int64      `json:"branch_id"`
	BranchCode      string     `json:"branch_code"`
	BranchName      string     `json:"branch_name"`
	VendorID        int64      `json:"vendor_id"`
	VendorName      string     `json:"vendor_name"`
	SentAt          *time.Time `json:"sent_at"`
	DecidedAt       *time.Time `json:"decided_at"`
	DecidedByName   *string    `json:"decided_by_name"`
	RejectionReason *string    `json:"rejection_reason"`
}

type RequestVendorPartyEvent struct {
	ID        int64      `json:"id"`
	PartyID   int64      `json:"party_id"`
	Event     string     `json:"event"`
	Reason    *string    `json:"reason"`
	ActorName string     `json:"actor_name"`
	CreatedAt *time.Time `json:"created_at"`
}

type RequestVendorParties struct {
	Parties []RequestVendorParty      `json:"parties"`
	Events  []RequestVendorPartyEvent `json:"events"`
}

// RequestParties reads the parties + history of one request on the replica
// (NFR2). Route RBAC is the Vendor Request viewer roles.
func (s *VendorOrderService) RequestParties(ctx context.Context, requestID int64) (*RequestVendorParties, error) {
	rows, err := s.read.ListVendorPartiesForRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list parties of request %d: %w", requestID, err)
	}
	evs, err := s.read.ListVendorPartyEventsForRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list party events of request %d: %w", requestID, err)
	}
	out := &RequestVendorParties{Parties: make([]RequestVendorParty, 0, len(rows)), Events: make([]RequestVendorPartyEvent, 0, len(evs))}
	for _, r := range rows {
		out.Parties = append(out.Parties, RequestVendorParty{
			ID: r.ID, Role: r.Role, Status: r.Status, BranchID: r.VendorBranchID, BranchCode: r.BranchCode, BranchName: r.BranchName,
			VendorID: r.VendorID, VendorName: r.VendorName, SentAt: timestamptzToPtr(r.SentAt), DecidedAt: timestamptzToPtr(r.DecidedAt),
			DecidedByName: r.DecidedByName, RejectionReason: r.RejectionReason,
		})
	}
	for _, e := range evs {
		out.Events = append(out.Events, RequestVendorPartyEvent{
			ID: e.ID, PartyID: e.PartyID, Event: e.Event, Reason: e.Reason, ActorName: e.ActorName, CreatedAt: timestamptzToPtr(e.CreatedAt),
		})
	}
	return out, nil
}
