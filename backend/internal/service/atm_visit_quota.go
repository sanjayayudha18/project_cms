package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Kuota kunjungan replenish per ATM (.claude/sdlc/atm-visit-quota/spec.md).
// Kuota = package_frequencies.cr_frequency of the ATM's active package (read
// only, never modified). atm_visit_quotas.remaining is decremented once per
// counted visit and may go negative (= kelebihan kuota). No automatic reset:
// a checker resets it to the current kuota (FR4). Reset and visit-cancel
// apply immediately with audit in the same tx and RBAC at route + service —
// documented deviation from maker-checker, same pattern as Role Management.

var (
	ErrAtmNotFound        = errors.New("atm not found")
	ErrQuotaUnknown       = errors.New("paket ATM tidak dikenali, kuota tidak dapat di-reset")
	ErrVisitNotFound      = errors.New("atm visit not found")
	ErrVisitNotCancelable = errors.New("kunjungan sudah dibatalkan atau berasal dari periode sebelum reset terakhir")
	ErrCancelVisitReason  = errors.New("alasan pembatalan wajib diisi")
)

// visitSisa splits the stored remaining counter into what the screen shows:
// sisa (never below 0) and kelebihan (visits beyond the kuota).
func visitSisa(remaining int32) (sisa, over int32) {
	if remaining < 0 {
		return 0, -remaining
	}
	return remaining, 0
}

// visitOutcome is one ATM's result of recordVisit, also written to the
// approve audit entry.
type visitOutcome struct {
	TerminalID string `json:"terminal_id"`
	Recorded   bool   `json:"recorded"`
	QuotaKnown bool   `json:"quota_known"`
	OverQuota  bool   `json:"over_quota"`
	Remaining  *int32 `json:"remaining,omitempty"`
	Skipped    string `json:"skipped,omitempty"` // atm_not_found | already_recorded
}

// recordVisit counts one visit for terminalID under requestID, inside the
// caller's tx. Order: ensure the quota row exists (only when the kuota is
// known) -> lock it -> insert the visit (UNIQUE per request+ATM; a duplicate
// is a no-op) -> decrement. An ATM with an existing quota row is decremented
// even if its current package no longer resolves (the row is the snapshot).
func recordVisit(ctx context.Context, q *db.Queries, requestID int64, terminalID string, asOf time.Time) (visitOutcome, error) {
	out := visitOutcome{TerminalID: terminalID}
	atm, err := q.ResolveAtmQuota(ctx, db.ResolveAtmQuotaParams{AsOfDate: toPgDate(asOf), TerminalID: terminalID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			out.Skipped = "atm_not_found"
			return out, nil
		}
		return out, fmt.Errorf("resolve kuota %s: %w", terminalID, err)
	}

	if label := packageLabel(atm.PackageCode, atm.VendorPackageLabel); label != nil && atm.CrFrequency != nil {
		if err := q.EnsureAtmVisitQuota(ctx, db.EnsureAtmVisitQuotaParams{
			AtmID: atm.AtmID, PackageCode: *label, QuotaTotal: *atm.CrFrequency,
		}); err != nil {
			return out, fmt.Errorf("ensure kuota %s: %w", terminalID, err)
		}
	}
	_, err = q.GetAtmVisitQuotaForUpdate(ctx, atm.AtmID)
	switch {
	case err == nil:
		out.QuotaKnown = true
	case !errors.Is(err, pgx.ErrNoRows):
		return out, fmt.Errorf("lock kuota %s: %w", terminalID, err)
	}

	visit, err := q.InsertAtmVisit(ctx, db.InsertAtmVisitParams{
		AtmID: atm.AtmID, VendorRequestID: requestID, QuotaKnown: out.QuotaKnown,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			out.Skipped = "already_recorded"
			return out, nil
		}
		return out, fmt.Errorf("insert visit %s: %w", terminalID, err)
	}
	out.Recorded = true
	if !out.QuotaKnown {
		return out, nil
	}

	quota, err := q.AdjustAtmVisitQuota(ctx, db.AdjustAtmVisitQuotaParams{Delta: -1, AtmID: atm.AtmID})
	if err != nil {
		return out, fmt.Errorf("decrement kuota %s: %w", terminalID, err)
	}
	out.Remaining = &quota.Remaining
	if quota.Remaining < 0 {
		out.OverQuota = true
		if err := q.MarkAtmVisitOverQuota(ctx, visit.ID); err != nil {
			return out, fmt.Errorf("mark over quota %s: %w", terminalID, err)
		}
	}
	return out, nil
}

// AtmVisitView is one visit of the current period on the ATM profile.
type AtmVisitView struct {
	ID              int64
	VendorRequestID int64
	RequestNumber   string
	QuotaKnown      bool
	IsOverQuota     bool
	CreatedAt       time.Time
	CancelledAt     *time.Time
	CancelledByName *string
	CancelReason    *string
}

// AtmVisitQuotaView backs GET /atm-visit-quotas/atms/{terminalId}.
// HasQuota=false: the ATM has no quota row yet (Remaining/Sisa/... zero).
// CurrentPackageCode/CurrentQuota describe today's active package — what a
// reset would set; nil = "Paket tidak dikenali".
type AtmVisitQuotaView struct {
	TerminalID         string
	HasQuota           bool
	PackageCode        string
	QuotaTotal         int32
	Remaining          int32
	Sisa               int32
	Kelebihan          int32
	ResetAt            *time.Time
	ResetBy            *UserRef
	CurrentPackageCode *string
	CurrentQuota       *int32
	Visits             []AtmVisitView
}

// VendorQuotaResetResult backs POST /atm-visit-quotas/vendors/{id}/reset.
type VendorQuotaResetResult struct {
	ResetCount   int
	SkippedCount int
}

// AtmVisitQuotaService implements the kuota endpoints. Reads use the primary
// pool (plan D1: the numbers are read right after approve/reset/cancel).
type AtmVisitQuotaService struct {
	pool VendorRequestPool
}

func NewAtmVisitQuotaService(pool VendorRequestPool) *AtmVisitQuotaService {
	return &AtmVisitQuotaService{pool: pool}
}

// Get returns the ATM's kuota, current-package kuota, and visits of the
// current period (since the last reset; all visits if it has no quota row).
func (s *AtmVisitQuotaService) Get(ctx context.Context, terminalID string) (*AtmVisitQuotaView, error) {
	q := db.New(s.pool)
	atm, err := q.ResolveAtmQuota(ctx, db.ResolveAtmQuotaParams{AsOfDate: toPgDate(jakartaCalendarDate(0)), TerminalID: terminalID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAtmNotFound
		}
		return nil, fmt.Errorf("resolve kuota %s: %w", terminalID, err)
	}
	view := &AtmVisitQuotaView{TerminalID: terminalID, CurrentPackageCode: packageLabel(atm.PackageCode, atm.VendorPackageLabel), CurrentQuota: atm.CrFrequency}

	since := pgtype.Timestamptz{Time: time.Unix(0, 0), Valid: true}
	quota, err := q.GetAtmVisitQuotaByAtm(ctx, atm.AtmID)
	switch {
	case err == nil:
		view.HasQuota = true
		view.PackageCode = quota.PackageCode
		view.QuotaTotal = quota.QuotaTotal
		view.Remaining = quota.Remaining
		view.Sisa, view.Kelebihan = visitSisa(quota.Remaining)
		view.ResetAt = timestamptzToPtr(quota.ResetAt)
		view.ResetBy = userRefOrNil(quota.ResetBy, quota.ResetByName)
		since = quota.ResetAt
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("get kuota %s: %w", terminalID, err)
	}

	rows, err := q.ListAtmVisitsSince(ctx, db.ListAtmVisitsSinceParams{AtmID: atm.AtmID, Since: since})
	if err != nil {
		return nil, fmt.Errorf("list visits %s: %w", terminalID, err)
	}
	view.Visits = make([]AtmVisitView, len(rows))
	for i, r := range rows {
		view.Visits[i] = AtmVisitView{
			ID: r.ID, VendorRequestID: r.VendorRequestID, RequestNumber: r.RequestNumber,
			QuotaKnown: r.QuotaKnown, IsOverQuota: r.IsOverQuota, CreatedAt: r.CreatedAt.Time,
			CancelledAt: timestamptzToPtr(r.CancelledAt), CancelledByName: r.CancelledByName, CancelReason: r.CancelReason,
		}
	}
	return view, nil
}

// withTx runs fn in one tx after the checker RBAC check (FR4.3).
func (s *AtmVisitQuotaService) withTx(ctx context.Context, actor Actor, fn func(tx pgx.Tx, q *db.Queries) error) error {
	if !isChecker(actor.Role) {
		return ErrNotChecker
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// resetOne sets one ATM's kuota back to its active package's cr_frequency
// and audits before/after.
func resetOne(ctx context.Context, tx pgx.Tx, q *db.Queries, actor Actor, atmID int64, terminalID, packageCode string, quotaTotal int32) error {
	var before any
	if old, err := q.GetAtmVisitQuotaForUpdate(ctx, atmID); err == nil {
		before = map[string]any{"package_code": old.PackageCode, "quota_total": old.QuotaTotal, "remaining": old.Remaining}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lock kuota %s: %w", terminalID, err)
	}
	quota, err := q.ResetAtmVisitQuota(ctx, db.ResetAtmVisitQuotaParams{
		AtmID: atmID, PackageCode: packageCode, QuotaTotal: quotaTotal, ResetBy: actor.UserID,
	})
	if err != nil {
		return fmt.Errorf("reset kuota %s: %w", terminalID, err)
	}
	if err := newAuditWriter(tx).Write(ctx, audit.Entry{
		ActorID: actor.UserID, Action: "reset", EntityType: "atm_visit_quota", EntityID: atmID,
		Before: before,
		After:  map[string]any{"terminal_id": terminalID, "package_code": quota.PackageCode, "quota_total": quota.QuotaTotal, "remaining": quota.Remaining},
		IP:     actor.IP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

// ResetATM resets one ATM (FR4.1). Unknown kuota -> ErrQuotaUnknown (422).
func (s *AtmVisitQuotaService) ResetATM(ctx context.Context, actor Actor, terminalID string) (*AtmVisitQuotaView, error) {
	err := s.withTx(ctx, actor, func(tx pgx.Tx, q *db.Queries) error {
		atm, err := q.ResolveAtmQuota(ctx, db.ResolveAtmQuotaParams{AsOfDate: toPgDate(jakartaCalendarDate(0)), TerminalID: terminalID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAtmNotFound
			}
			return fmt.Errorf("resolve kuota %s: %w", terminalID, err)
		}
		label := packageLabel(atm.PackageCode, atm.VendorPackageLabel)
		if label == nil || atm.CrFrequency == nil {
			return ErrQuotaUnknown
		}
		return resetOne(ctx, tx, q, actor, atm.AtmID, terminalID, *label, *atm.CrFrequency)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, terminalID)
}

// ResetVendor resets every ATM whose active package belongs to the vendor
// (FR4.2); ATMs with an unknown kuota are skipped and counted. One tx.
func (s *AtmVisitQuotaService) ResetVendor(ctx context.Context, actor Actor, vendorID int64) (*VendorQuotaResetResult, error) {
	res := &VendorQuotaResetResult{}
	err := s.withTx(ctx, actor, func(tx pgx.Tx, q *db.Queries) error {
		atms, err := q.ListAtmsForVendorQuotaReset(ctx, db.ListAtmsForVendorQuotaResetParams{
			AsOfDate: toPgDate(jakartaCalendarDate(0)), VendorID: vendorID,
		})
		if err != nil {
			return fmt.Errorf("list ATMs of vendor %d: %w", vendorID, err)
		}
		for _, a := range atms {
			label := packageLabel(a.PackageCode, a.VendorPackageLabel)
			if a.CrFrequency == nil || label == nil {
				res.SkippedCount++
				continue
			}
			if err := resetOne(ctx, tx, q, actor, a.AtmID, a.TerminalID, *label, *a.CrFrequency); err != nil {
				return err
			}
			res.ResetCount++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// CancelVisit soft-cancels one visit of the current period and gives the
// kuota back (FR3.2/3.3). A visit not counted against a quota row
// (quota_known=false) is cancelled without touching any counter.
func (s *AtmVisitQuotaService) CancelVisit(ctx context.Context, actor Actor, visitID int64, reason string) error {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return ErrCancelVisitReason
	}
	if len(trimmed) > 500 {
		return &ValidationError{Field: "reason", Message: "maksimal 500 karakter"}
	}
	return s.withTx(ctx, actor, func(tx pgx.Tx, q *db.Queries) error {
		visit, err := q.GetAtmVisitForUpdate(ctx, visitID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVisitNotFound
			}
			return fmt.Errorf("load visit %d: %w", visitID, err)
		}
		if visit.CancelledAt.Valid {
			return ErrVisitNotCancelable
		}
		var remaining *int32
		if visit.QuotaKnown {
			quota, err := q.GetAtmVisitQuotaForUpdate(ctx, visit.AtmID)
			if err != nil {
				return fmt.Errorf("lock kuota of visit %d: %w", visitID, err)
			}
			if visit.CreatedAt.Time.Before(quota.ResetAt.Time) {
				return ErrVisitNotCancelable
			}
			adj, err := q.AdjustAtmVisitQuota(ctx, db.AdjustAtmVisitQuotaParams{Delta: 1, AtmID: visit.AtmID})
			if err != nil {
				return fmt.Errorf("restore kuota of visit %d: %w", visitID, err)
			}
			remaining = &adj.Remaining
		}
		if _, err := q.CancelAtmVisit(ctx, db.CancelAtmVisitParams{CancelledBy: actor.UserID, CancelReason: trimmed, ID: visitID}); err != nil {
			return fmt.Errorf("cancel visit %d: %w", visitID, err)
		}
		return newAuditWriter(tx).Write(ctx, audit.Entry{
			ActorID: actor.UserID, Action: "cancel", EntityType: "atm_visit", EntityID: visitID,
			Before: map[string]any{"cancelled": false},
			After:  map[string]any{"cancelled": true, "reason": trimmed, "atm_id": visit.AtmID, "remaining": remaining},
			IP:     actor.IP,
		})
	})
}

// packageLabel is the package label of an ATM's active kelolaan: the branch
// package's code, or the vendor-wide label (migration 023). nil = no active
// package. The two come from separate columns because sqlc types a COALESCE of
// them as non-null, which would crash the scan for an ATM without a package.
func packageLabel(branchCode, vendorLabel *string) *string {
	if branchCode != nil {
		return branchCode
	}
	return vendorLabel
}
