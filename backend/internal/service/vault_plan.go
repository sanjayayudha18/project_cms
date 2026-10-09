package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// VaultPlanService is the ACM side of penetapan vault (cit-acm-plan FR3-FR6):
// read plans scoped to the actor's ACM areas, compute tiered candidates with
// saldo/capacity, save assignments, submit/approve/reject, and the ATM-SPV
// review of the whole recommendation. Maker-checker uses its own state
// machine (documented deviation (a), same as Vendor Request), audit in tx.
type VaultPlanService struct {
	pool     VaultPlanPool
	read     *db.Queries
	notifier Notifier
}

// VaultPlanPool is the primary pool: transactions + read-after-write.
type VaultPlanPool interface {
	db.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ErrVaultPlanNotFound also hides plans of areas the actor is not a member of (FR6.2).
var ErrVaultPlanNotFound = errors.New("rencana vault tidak ditemukan")

const vaultPlanEntity = "vendor_request_vault_plan"

// NewVaultPlanService creates a VaultPlanService; readDB is the replica.
func NewVaultPlanService(pool VaultPlanPool, readDB db.DBTX) *VaultPlanService {
	return &VaultPlanService{pool: pool, read: db.New(readDB)}
}

// WithNotifier enables the FR6.3 notifications.
func (s *VaultPlanService) WithNotifier(n Notifier) *VaultPlanService {
	s.notifier = n
	return s
}

func hasRole(actor Actor, role string) bool { return strings.EqualFold(actor.Role, role) }

// planFor loads a plan the actor may see: ADMIN sees all, others only plans
// of areas they belong to (404 otherwise, no leak of other areas).
func planFor(ctx context.Context, q *db.Queries, actor Actor, id int64) (db.GetVaultPlanRow, error) {
	p, err := q.GetVaultPlan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrVaultPlanNotFound
	}
	if err != nil {
		return p, fmt.Errorf("load vault plan %d: %w", id, err)
	}
	if hasRole(actor, "ADMIN") {
		return p, nil
	}
	member, err := q.IsAcmAreaMember(ctx, db.IsAcmAreaMemberParams{AcmAreaID: p.AcmAreaID, UserID: actor.UserID})
	if err != nil {
		return p, fmt.Errorf("check area membership: %w", err)
	}
	if !member {
		return p, ErrVaultPlanNotFound
	}
	return p, nil
}

// VaultPlanListFilter narrows List; zero values mean "any".
type VaultPlanListFilter struct {
	Status   string
	AreaID   *int64
	DateFrom pgtype.Date
	DateTo   pgtype.Date
}

// List returns plans of the actor's areas (ADMIN: all), newest replenish date first (replica).
func (s *VaultPlanService) List(ctx context.Context, actor Actor, f VaultPlanListFilter) ([]db.ListVaultPlansRow, error) {
	var userID *int64
	if !hasRole(actor, "ADMIN") {
		userID = &actor.UserID
	}
	return s.read.ListVaultPlans(ctx, db.ListVaultPlansParams{UserID: userID, Status: f.Status, AreaID: f.AreaID, DateFrom: f.DateFrom, DateTo: f.DateTo})
}

// VaultBranchRef names a branch in responses.
type VaultBranchRef struct {
	ID         int64   `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	VendorID   int64   `json:"vendor_id"`
	VendorName string  `json:"vendor_name"`
	RegionCode *string `json:"region_code"`
}

// VaultAssignmentView is the stored assignment of one ATM.
type VaultAssignmentView struct {
	VaultBranch     VaultBranchRef    `json:"vault_branch"`
	Tier            int16             `json:"tier"`
	IsUrgent        bool              `json:"is_urgent"`
	UrgentReason    *string           `json:"urgent_reason"`
	Saldo           map[string]string `json:"saldo_snapshot"`
	Capacity        map[string]string `json:"capacity_snapshot"`
	CapacityWarning bool              `json:"capacity_warning"`
}

// VaultPlanAtm is one ATM of a plan with its order and assignment.
type VaultPlanAtm struct {
	TerminalID      string               `json:"terminal_id"`
	ReplenishBranch VaultBranchRef       `json:"replenish_branch"`
	Order           map[string]string    `json:"order"`
	Assignment      *VaultAssignmentView `json:"assignment"`
}

// VaultPlanDetail is a plan header plus its ATMs.
type VaultPlanDetail struct {
	Plan db.GetVaultPlanRow
	Atms []VaultPlanAtm
}

// Get returns a plan with ATMs, orders and assignments (replica).
func (s *VaultPlanService) Get(ctx context.Context, actor Actor, id int64) (*VaultPlanDetail, error) {
	return s.detail(ctx, s.read, actor, id)
}

func (s *VaultPlanService) detail(ctx context.Context, q *db.Queries, actor Actor, id int64) (*VaultPlanDetail, error) {
	p, err := planFor(ctx, q, actor, id)
	if err != nil {
		return nil, err
	}
	atms, err := q.ListVaultPlanAtms(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list plan atms: %w", err)
	}
	items, err := q.ListVendorRequestItems(ctx, p.VendorRequestID)
	if err != nil {
		return nil, fmt.Errorf("list request items: %w", err)
	}
	orders := map[string]denomAmounts{}
	for _, it := range items {
		if orders[it.TerminalID] == nil {
			orders[it.TerminalID] = denomAmounts{}
		}
		orders[it.TerminalID][it.Denom] += it.AmountReplenish
	}
	branches, err := candidateBranchIndex(ctx, q)
	if err != nil {
		return nil, err
	}

	out := &VaultPlanDetail{Plan: p, Atms: make([]VaultPlanAtm, len(atms))}
	for i, a := range atms {
		atm := VaultPlanAtm{
			TerminalID: a.TerminalID,
			ReplenishBranch: VaultBranchRef{ID: a.ReplenishBranchID, Code: a.ReplenishBranchCode, Name: a.ReplenishBranchName,
				VendorID: a.ReplenishVendorID, VendorName: a.ReplenishVendorName, RegionCode: a.ReplenishRegionCode},
			Order: amountsJSON(orders[a.TerminalID]),
		}
		if a.VaultBranchID != nil {
			v := VaultAssignmentView{Tier: deref(a.Tier), IsUrgent: deref(a.IsUrgent), UrgentReason: a.UrgentReason,
				Saldo: decodeSnapshot(a.SaldoSnapshot), Capacity: decodeSnapshot(a.CapacitySnapshot), CapacityWarning: deref(a.CapacityWarning)}
			v.VaultBranch = VaultBranchRef{ID: *a.VaultBranchID}
			if b, ok := branches[*a.VaultBranchID]; ok {
				region := b.RegionCode
				v.VaultBranch = VaultBranchRef{ID: b.ID, Code: b.BranchCode, Name: b.BranchName, VendorID: b.VendorID, VendorName: b.VendorName, RegionCode: &region}
			}
			atm.Assignment = &v
		}
		out.Atms[i] = atm
	}
	return out, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func decodeSnapshot(b []byte) map[string]string {
	if len(b) == 0 {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func candidateBranchIndex(ctx context.Context, q *db.Queries) (map[int64]db.ListVaultCandidateBranchesRow, error) {
	rows, err := q.ListVaultCandidateBranches(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vault candidate branches: %w", err)
	}
	out := make(map[int64]db.ListVaultCandidateBranchesRow, len(rows))
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, nil
}

// loadMetrics returns the FR3 inputs for one replenish date: demand for every
// live vault-flow ATM and saldo for the given branches.
func loadMetrics(ctx context.Context, q *db.Queries, date pgtype.Date, branchIDs []int64) (*vaultDemand, map[int64]branchSaldo, error) {
	demandRows, err := q.ListVaultDemand(ctx, date)
	if err != nil {
		return nil, nil, fmt.Errorf("list vault demand: %w", err)
	}
	saldo := map[int64]branchSaldo{}
	if len(branchIDs) > 0 {
		rows, err := q.ListVaultBranchSaldo(ctx, db.ListVaultBranchSaldoParams{BranchIds: branchIDs, ReportDate: date})
		if err != nil {
			return nil, nil, fmt.Errorf("list vault branch saldo: %w", err)
		}
		for _, r := range rows {
			saldo[r.VendorBranchID] = saldoFromRow(r)
		}
	}
	return newVaultDemand(demandRows), saldo, nil
}

// Candidates lists the vault branches for one ATM of the plan (FR4.2): tier 1
// and 2 always, tier 3 only when urgent; sorted by tier, known saldo, capacity.
func (s *VaultPlanService) Candidates(ctx context.Context, actor Actor, id int64, terminalID string, urgent bool) ([]VaultCandidate, error) {
	q := s.read
	p, err := planFor(ctx, q, actor, id)
	if err != nil {
		return nil, err
	}
	atms, err := q.ListVaultPlanAtms(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list plan atms: %w", err)
	}
	var atm *db.ListVaultPlanAtmsRow
	for i := range atms {
		if atms[i].TerminalID == terminalID {
			atm = &atms[i]
		}
	}
	if atm == nil {
		return nil, &ValidationError{Field: "terminal_id", Message: "ATM bukan bagian dari rencana ini"}
	}
	branches, err := q.ListVaultCandidateBranches(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vault candidate branches: %w", err)
	}
	var picked []db.ListVaultCandidateBranchesRow
	var ids []int64
	for _, b := range branches {
		if vaultTier(b.VendorID, b.RegionCode, atm.ReplenishVendorID, atm.ReplenishRegionCode) == 3 && !urgent {
			continue
		}
		picked = append(picked, b)
		ids = append(ids, b.ID)
	}
	demand, saldo, err := loadMetrics(ctx, q, p.ReplenishDate, ids)
	if err != nil {
		return nil, err
	}
	k := atmKey{p.VendorRequestID, terminalID}
	need := demand.needs[k]
	out := make([]VaultCandidate, len(picked))
	for i, b := range picked {
		sb := saldo[b.ID]
		capa, known := demand.capacity(sb, b.ID, k)
		c := VaultCandidate{VendorBranchID: b.ID, BranchCode: b.BranchCode, BranchName: b.BranchName, VendorName: b.VendorName,
			RegionCode: b.RegionCode, Category: b.Category, Tier: vaultTier(b.VendorID, b.RegionCode, atm.ReplenishVendorID, atm.ReplenishRegionCode),
			SaldoKnown: known, Capacity: amountsJSON(capa), Warning: capacityWarning(capa, known, need), coverage: coverage(capa, need)}
		if known {
			c.Saldo = amountsJSON(sb.amounts)
		}
		out[i] = c
	}
	sortCandidates(out)
	return out, nil
}

// recomputeSnapshots refreshes saldo/capacity snapshots + capacity_warning of
// every assignment of the plan from current data (FR4.3 save, FR4.4 submit).
func recomputeSnapshots(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow) error {
	atms, err := q.ListVaultPlanAtms(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("list plan atms: %w", err)
	}
	var ids []int64
	for _, a := range atms {
		if a.VaultBranchID != nil {
			ids = append(ids, *a.VaultBranchID)
		}
	}
	demand, saldo, err := loadMetrics(ctx, q, p.ReplenishDate, ids)
	if err != nil {
		return err
	}
	for _, a := range atms {
		if a.AssignmentID == nil || a.VaultBranchID == nil {
			continue
		}
		k := atmKey{p.VendorRequestID, a.TerminalID}
		sb := saldo[*a.VaultBranchID]
		capa, known := demand.capacity(sb, *a.VaultBranchID, k)
		var saldoSnap denomAmounts
		if known {
			saldoSnap = sb.amounts
		}
		if err := q.UpdateVaultAssignmentSnapshot(ctx, db.UpdateVaultAssignmentSnapshotParams{
			ID: *a.AssignmentID, SaldoSnapshot: snapshotBytes(saldoSnap), CapacitySnapshot: snapshotBytes(capa),
			CapacityWarning: capacityWarning(capa, known, demand.needs[k]),
		}); err != nil {
			return fmt.Errorf("update snapshot %s: %w", a.TerminalID, err)
		}
	}
	return nil
}

// Audit returns the audit trail of a plan.
func (s *VaultPlanService) Audit(ctx context.Context, actor Actor, id int64) ([]AuditEntry, error) {
	if _, err := planFor(ctx, s.read, actor, id); err != nil {
		return nil, err
	}
	rows, err := s.read.ListAuditLogsByEntity(ctx, db.ListAuditLogsByEntityParams{EntityType: vaultPlanEntity, EntityID: id})
	if err != nil {
		return nil, fmt.Errorf("list audit log for vault plan %d: %w", id, err)
	}
	out := make([]AuditEntry, len(rows))
	for i, r := range rows {
		out[i] = AuditEntry{ID: r.ID, EntityType: r.EntityType, EntityID: r.EntityID, Action: r.Action, PerformedBy: r.ActorID,
			PerformedAt: r.CreatedAt.Time, PreviousState: extractState(r.Before), NewState: extractState(r.After), Metadata: extractMetadata(r.After)}
	}
	return out, nil
}

// RequestVaultReview is the ATM-SPV panel (FR5.1): area plans + assignments of a request.
type RequestVaultReview struct {
	Plans       []db.ListVaultPlansForRequestRow
	Assignments []db.ListVaultAssignmentsForRequestRow
}

// ForRequest returns the vault recommendation of a Vendor Request (replica).
func (s *VaultPlanService) ForRequest(ctx context.Context, requestID int64) (*RequestVaultReview, error) {
	plans, err := s.read.ListVaultPlansForRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list plans of request %d: %w", requestID, err)
	}
	asg, err := s.read.ListVaultAssignmentsForRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list assignments of request %d: %w", requestID, err)
	}
	return &RequestVaultReview{Plans: plans, Assignments: asg}, nil
}
