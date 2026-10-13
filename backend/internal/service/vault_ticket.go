package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Tiket pemenuhan vault (cit-vault-select spec v3 FR6): one ticket per
// (request, vault), synced at ACM-USER submit. Same pattern as the replenish
// tickets (vendor_request_ticket.go).

// vaultTicketPlan is the diff applyVaultTickets writes.
type vaultTicketPlan struct {
	Keep       []string // ticket numbers left as they are
	Deactivate []int64  // vault no longer allocated
	Reactivate []int64  // same vault back in the same request -> same number
	Issue      []int64  // vendor_vault_id, ascending (lock order)
}

// planVaultTickets diffs the request's tickets (active and inactive) against
// the vaults the plan allocates now (FR6.2). Pure.
func planVaultTickets(existing []db.ListVaultTicketsForRequestRow, wanted []int64) vaultTicketPlan {
	want := map[int64]bool{}
	for _, v := range wanted {
		want[v] = true
	}
	var p vaultTicketPlan
	have := map[int64]bool{}
	inactive := map[int64]int64{} // vault -> ticket id
	for _, t := range existing {
		switch {
		case !t.IsActive:
			inactive[t.VendorVaultID] = t.ID
		case want[t.VendorVaultID]:
			have[t.VendorVaultID] = true
			p.Keep = append(p.Keep, t.TicketNumber)
		default:
			p.Deactivate = append(p.Deactivate, t.ID)
		}
	}
	ids := make([]int64, 0, len(want))
	for v := range want {
		ids = append(ids, v)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, v := range ids {
		if have[v] {
			continue
		}
		if id, ok := inactive[v]; ok {
			p.Reactivate = append(p.Reactivate, id)
			continue
		}
		p.Issue = append(p.Issue, v)
	}
	return p
}

// vaultTicketAudit is after.vault_tickets of the submit audit entry (FR6.4).
type vaultTicketAudit struct {
	Issued      []string `json:"issued"`
	Kept        []string `json:"kept"`
	Reactivated []string `json:"reactivated"`
	Deactivated []string `json:"deactivated"`
}

// applyVaultTickets writes p: deactivate, reactivate, then issue. Each issue
// locks (vault, date) first; seq past 999 -> 422 (FR6.1).
func applyVaultTickets(ctx context.Context, q *db.Queries, requestID int64, replenishDate pgtype.Date, p vaultTicketPlan) (vaultTicketAudit, error) {
	a := vaultTicketAudit{Issued: []string{}, Kept: p.Keep, Reactivated: []string{}, Deactivated: []string{}}
	if a.Kept == nil {
		a.Kept = []string{}
	}
	for _, id := range p.Deactivate {
		n, err := q.SetVaultTicketActive(ctx, db.SetVaultTicketActiveParams{ID: id, IsActive: false})
		if err != nil {
			return a, fmt.Errorf("deactivate vault ticket %d: %w", id, err)
		}
		a.Deactivated = append(a.Deactivated, n)
	}
	for _, id := range p.Reactivate {
		n, err := q.SetVaultTicketActive(ctx, db.SetVaultTicketActiveParams{ID: id, IsActive: true})
		if err != nil {
			return a, fmt.Errorf("reactivate vault ticket %d: %w", id, err)
		}
		a.Reactivated = append(a.Reactivated, n)
	}
	for _, vaultID := range p.Issue {
		if err := q.LockVaultTicketSeq(ctx, db.LockVaultTicketSeqParams{VendorVaultID: vaultID, ReplenishDate: replenishDate}); err != nil {
			return a, fmt.Errorf("lock vault ticket seq %d: %w", vaultID, err)
		}
		n, err := q.IssueVaultTicket(ctx, db.IssueVaultTicketParams{
			VendorRequestID: requestID, VendorVaultID: vaultID, ReplenishDate: replenishDate,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == "vendor_request_vault_tickets_seq_chk" {
				return a, allocErr("nomor tiket vault %d tanggal %s sudah mencapai 999", vaultID, replenishDate.Time.Format("20060102"))
			}
			return a, fmt.Errorf("issue vault ticket %d: %w", vaultID, err)
		}
		a.Issued = append(a.Issued, n)
	}
	return a, nil
}
