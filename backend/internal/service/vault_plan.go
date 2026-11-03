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

// VaultPlanService is the ACM side of pemenuhan vault (cit-acm-plan, revised
// by cit-vault-select v3): read plans scoped to the actor's ACM areas, list
// tiered vault candidates with saldo/capacity, save the per-vault allocation of
// the request's nominal, submit (+ vault tickets)/approve/reject, and the
// ATM-SPV review of the whole recommendation. Maker-checker uses its own state
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

// VaultPlanAtm is one ATM of the request with its order (read-only, FR2.3).
type VaultPlanAtm struct {
	TerminalID      string            `json:"terminal_id"`
	ReplenishBranch VaultBranchRef    `json:"replenish_branch"`
	Order           map[string]string `json:"order"`
}

// VaultAllocationView is one saved vault row of the plan (FR4.1, FR9.1).
type VaultAllocationView struct {
	ID              int64             `json:"id"`
	VendorVaultID   int64             `json:"vendor_vault_id"`
	VaultCode       string            `json:"vault_code"`
	VaultCategory   string            `json:"vault_category"`
	Branch          VaultBranchRef    `json:"branch"`
	Tier            int16             `json:"tier"`
	IsUrgent        bool              `json:"is_urgent"`
	UrgentReason    *string           `json:"urgent_reason"`
	Amounts         map[string]string `json:"amounts"`
	Saldo           map[string]string `json:"saldo_snapshot"`
	Capacity        map[string]string `json:"capacity_snapshot"`
	CapacityWarning bool              `json:"capacity_warning"`
	TicketNumber    *string           `json:"ticket_number"`
}

// VaultPlanDetail is a plan header, the request's ATMs and order totals, the
// allocation rows and the request's vault tickets (active + inactive).
type VaultPlanDetail struct {
	Plan        db.GetVaultPlanRow
	Atms        []VaultPlanAtm
	OrderTotals map[string]string
	Allocations []VaultAllocationView
	Tickets     []db.ListVaultTicketsForRequestRow
}

// Get returns a plan with ATMs, order totals, allocations and tickets (replica).
func (s *VaultPlanService) Get(ctx context.Context, actor Actor, id int64) (*VaultPlanDetail, error) {
	return s.detail(ctx, s.read, actor, id)
}

func (s *VaultPlanService) detail(ctx context.Context, q *db.Queries, actor Actor, id int64) (*VaultPlanDetail, error) {
	p, err := planFor(ctx, q, actor, id)
	if err != nil {
		return nil, err
	}
	return detailOf(ctx, q, p)
}

func detailOf(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow) (*VaultPlanDetail, error) {
	atms, err := planAtms(ctx, q, p.VendorRequestID)
	if err != nil {
		return nil, err
	}
	order, err := orderTotals(ctx, q, p.VendorRequestID)
	if err != nil {
		return nil, err
	}
	rows, amounts, err := planAllocations(ctx, q, p.ID)
	if err != nil {
		return nil, err
	}
	tickets, err := q.ListVaultTicketsForRequest(ctx, p.VendorRequestID)
	if err != nil {
		return nil, fmt.Errorf("list vault tickets: %w", err)
	}
	out := &VaultPlanDetail{Plan: p, Atms: atms, OrderTotals: amountsJSON(order), Allocations: make([]VaultAllocationView, len(rows)), Tickets: tickets}
	for i, a := range rows {
		out.Allocations[i] = VaultAllocationView{ID: a.ID, VendorVaultID: a.VendorVaultID, VaultCode: a.VaultCode, VaultCategory: a.VaultCategory,
			Branch: VaultBranchRef{ID: a.VaultBranchID, Code: a.BranchCode, Name: a.BranchName, VendorID: a.VendorID, VendorName: a.VendorName, RegionCode: a.BranchRegionCode},
			Tier:   a.Tier, IsUrgent: a.IsUrgent, UrgentReason: a.UrgentReason, Amounts: amountsJSON(amounts[a.ID]),
			Saldo: decodeSnapshot(a.SaldoSnapshot), Capacity: decodeSnapshot(a.CapacitySnapshot), CapacityWarning: a.CapacityWarning,
			TicketNumber: a.TicketNumber}
	}
	return out, nil
}

// planAtms lists the request's ATMs with their order per denom.
func planAtms(ctx context.Context, q *db.Queries, requestID int64) ([]VaultPlanAtm, error) {
	atms, err := q.ListVaultPlanAtms(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list plan atms: %w", err)
	}
	items, err := q.ListVendorRequestItems(ctx, requestID)
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
	out := make([]VaultPlanAtm, len(atms))
	for i, a := range atms {
		out[i] = VaultPlanAtm{TerminalID: a.TerminalID, Order: amountsJSON(orders[a.TerminalID]),
			ReplenishBranch: VaultBranchRef{ID: a.ReplenishBranchID, Code: a.ReplenishBranchCode, Name: a.ReplenishBranchName,
				VendorID: a.ReplenishVendorID, VendorName: a.ReplenishVendorName, RegionCode: a.ReplenishRegionCode}}
	}
	return out, nil
}

// orderTotals is FR2.3: the request's order per denom.
func orderTotals(ctx context.Context, q *db.Queries, requestID int64) (denomAmounts, error) {
	rows, err := q.ListRequestOrderTotals(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list order totals: %w", err)
	}
	out := denomAmounts{}
	for _, r := range rows {
		out[r.Denom] = r.Amount
	}
	return out, nil
}

// planAllocations returns the plan's allocation rows and their amounts by allocation id.
func planAllocations(ctx context.Context, q *db.Queries, planID int64) ([]db.ListVaultPlanAllocationsRow, map[int64]denomAmounts, error) {
	rows, err := q.ListVaultPlanAllocations(ctx, planID)
	if err != nil {
		return nil, nil, fmt.Errorf("list allocations: %w", err)
	}
	amts, err := q.ListVaultPlanAllocationAmounts(ctx, planID)
	if err != nil {
		return nil, nil, fmt.Errorf("list allocation amounts: %w", err)
	}
	amounts := map[int64]denomAmounts{}
	for _, m := range amts {
		if amounts[m.AllocationID] == nil {
			amounts[m.AllocationID] = denomAmounts{}
		}
		amounts[m.AllocationID][m.Denom] = m.Amount
	}
	return rows, amounts, nil
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

// vaultMetrics is FR5 for one plan: saldo per vault on the replenish date and
// what other live plans already allocated to it (NFR1: two set-based reads).
type vaultMetrics struct {
	saldo     map[int64]vaultSaldo
	allocated map[int64]denomAmounts
}

func loadVaultMetrics(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow, vaultIDs []int64) (*vaultMetrics, error) {
	m := &vaultMetrics{saldo: map[int64]vaultSaldo{}}
	if len(vaultIDs) > 0 {
		rows, err := q.ListVaultSaldo(ctx, db.ListVaultSaldoParams{VaultIds: vaultIDs, ReportDate: p.ReplenishDate})
		if err != nil {
			return nil, fmt.Errorf("list vault saldo: %w", err)
		}
		for _, r := range rows {
			m.saldo[r.VendorVaultID] = vaultSaldoFromRow(r)
		}
	}
	alloc, err := q.ListVaultAllocatedTotals(ctx, db.ListVaultAllocatedTotalsParams{ReplenishDate: p.ReplenishDate, ExcludePlanID: p.ID})
	if err != nil {
		return nil, fmt.Errorf("list vault allocated totals: %w", err)
	}
	m.allocated = allocatedByVault(alloc)
	return m, nil
}

// capacity returns the vault's saldo and FR5.3 remaining capacity (known=false: saldo unknown).
func (m *vaultMetrics) capacity(vaultID int64) (vaultSaldo, denomAmounts, bool) {
	s, ok := m.saldo[vaultID]
	if !ok {
		s = vaultSaldo{unknown: true}
	}
	capa, known := vaultCapacity(s, m.allocated[vaultID])
	return s, capa, known
}

// Candidates lists the eligible vaults grouped by branch (FR3): tier 1 and 2
// always, tier 3 only when urgent; sorted per FR3.4.
func (s *VaultPlanService) Candidates(ctx context.Context, actor Actor, id int64, urgent bool) ([]VaultBranchOption, error) {
	q := s.read
	p, err := planFor(ctx, q, actor, id)
	if err != nil {
		return nil, err
	}
	vaults, err := q.ListVaultCandidateVaults(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list vault candidates: %w", err)
	}
	order, err := orderTotals(ctx, q, p.VendorRequestID)
	if err != nil {
		return nil, err
	}
	type picked struct {
		row  db.ListVaultCandidateVaultsRow
		tier int16
	}
	var list []picked
	var ids []int64
	for _, v := range vaults {
		tier := vaultTier(v.VendorID, v.RegionCode, deref(p.VendorID), p.RegionCode)
		if tier == 3 && !urgent {
			continue
		}
		list = append(list, picked{v, tier})
		ids = append(ids, v.VendorVaultID)
	}
	m, err := loadVaultMetrics(ctx, q, p, ids)
	if err != nil {
		return nil, err
	}
	branches := map[int64]*VaultBranchOption{}
	for _, c := range list {
		v := c.row
		saldo, capa, known := m.capacity(v.VendorVaultID)
		o := VaultOption{VendorVaultID: v.VendorVaultID, VaultCode: v.VaultCode, Category: v.Category, Tier: c.tier,
			SaldoKnown: known, Capacity: amountsJSON(capa), coverage: coverage(capa, order)}
		if known {
			o.Saldo = amountsJSON(saldo.amounts)
		}
		b := branches[v.VendorBranchID]
		if b == nil {
			b = &VaultBranchOption{VendorBranchID: v.VendorBranchID, BranchCode: v.BranchCode, BranchName: v.BranchName,
				VendorID: v.VendorID, VendorName: v.VendorName, RegionCode: v.RegionCode, Tier: c.tier}
			branches[v.VendorBranchID] = b
		}
		b.Vaults = append(b.Vaults, o)
	}
	return groupVaultOptions(branches), nil
}

// refreshAllocationSnapshots recomputes saldo/capacity snapshots and
// capacity_warning of every allocation row of the plan (FR4.4: save + submit).
func refreshAllocationSnapshots(ctx context.Context, q *db.Queries, p db.GetVaultPlanRow) error {
	rows, amounts, err := planAllocations(ctx, q, p.ID)
	if err != nil {
		return err
	}
	ids := make([]int64, len(rows))
	for i, a := range rows {
		ids[i] = a.VendorVaultID
	}
	m, err := loadVaultMetrics(ctx, q, p, ids)
	if err != nil {
		return err
	}
	for _, a := range rows {
		saldo, capa, known := m.capacity(a.VendorVaultID)
		var saldoSnap denomAmounts
		if known {
			saldoSnap = saldo.amounts
		}
		if err := q.UpdateVaultAllocationSnapshot(ctx, db.UpdateVaultAllocationSnapshotParams{
			ID: a.ID, SaldoSnapshot: snapshotBytes(saldoSnap), CapacitySnapshot: snapshotBytes(capa),
			CapacityWarning: capacityWarning(capa, known, amounts[a.ID]),
		}); err != nil {
			return fmt.Errorf("update snapshot %s: %w", a.VaultCode, err)
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

// ForRequest returns the live vault plan of a Vendor Request in the detail
// shape (FR9.2 ATM-SPV review; replica). nil = the request has no live plan.
func (s *VaultPlanService) ForRequest(ctx context.Context, requestID int64) (*VaultPlanDetail, error) {
	plans, err := s.read.ListVaultPlansForRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list plans of request %d: %w", requestID, err)
	}
	if len(plans) == 0 {
		return nil, nil
	}
	p, err := s.read.GetVaultPlan(ctx, plans[0].ID)
	if err != nil {
		return nil, fmt.Errorf("load vault plan %d: %w", plans[0].ID, err)
	}
	return detailOf(ctx, s.read, p)
}
