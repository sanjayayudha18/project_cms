package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Replenish ticket + request region (.claude/sdlc/replenish-ticket/spec.md):
// one active ticket per (request, ATM) numbered
// <terminal_id>_<50K|100K|MIX>_<YYYYMMDD>_<NNN>, and one region code per
// request. The number itself is assembled only in SQL (IssueVendorRequestTicket).

// branchRegion is the managing vendor cabang of one item ATM (FR10.1).
type branchRegion struct {
	branchCode string
	regionCode *string
}

// singleRegion returns the one region code every ATM shares (FR10.1). want ""
// = any code (create); otherwise the request's own code (UpdateItems, FR10.4).
// Mixed codes or a cabang without a code -> 422 (FR10.2).
func singleRegion(atms map[string]branchRegion, want string) (string, error) {
	byCode := map[string][]string{}
	noCode := map[string]bool{}
	for terminal, br := range atms {
		if br.regionCode == nil {
			noCode[br.branchCode] = true
			continue
		}
		byCode[*br.regionCode] = append(byCode[*br.regionCode], terminal)
	}
	if len(noCode) > 0 {
		return "", &ValidationError{Field: "items", Message: "region vendor cabang " + strings.Join(sortedKeys(noCode), ", ") + " belum punya kode region"}
	}
	if len(byCode) == 1 {
		for code := range byCode {
			if want == "" || code == want {
				return code, nil
			}
		}
	}
	parts := make([]string, 0, len(byCode)+1)
	if want != "" {
		parts = append(parts, "request "+want)
	}
	for _, code := range sortedKeys(byCode) {
		sort.Strings(byCode[code])
		parts = append(parts, code+": "+strings.Join(byCode[code], ","))
	}
	return "", &ValidationError{Field: "items", Message: "ATM dalam satu request harus dari region vendor yang sama: " + strings.Join(parts, "; ")}
}

// denomCode: one denom -> "50K"/"100K"; more than one -> "MIX" (one ticket =
// one trip per ATM, PO 2026-10-08).
func denomCode(denoms map[int32]bool) string {
	if len(denoms) != 1 {
		return "MIX"
	}
	for d := range denoms {
		return fmt.Sprintf("%dK", d/1000)
	}
	return "MIX"
}

// wantedCodes maps every item terminal to its denom code.
func wantedCodes(items []resolvedItem) map[string]string {
	denoms := map[string]map[int32]bool{}
	for _, r := range items {
		if denoms[r.input.TerminalID] == nil {
			denoms[r.input.TerminalID] = map[int32]bool{}
		}
		denoms[r.input.TerminalID][r.input.Denom] = true
	}
	want := make(map[string]string, len(denoms))
	for terminal, ds := range denoms {
		want[terminal] = denomCode(ds)
	}
	return want
}

type ticketIssue struct{ terminalID, denomCode string }

// ticketPlan is what an item set changes on a request's tickets (FR1, FR2).
type ticketPlan struct {
	Deactivate []int64       // active tickets whose ATM left or changed code
	Reactivate []int64       // same (ATM, code) back in the same request -> same number
	Issue      []ticketIssue // sorted by terminal_id (lock order, D2)
}

// planTickets diffs the request's tickets (active and inactive) against the
// wanted code per ATM. Pure; applyTicketPlan does the writes.
func planTickets(existing []db.VendorRequestTicket, want map[string]string) ticketPlan {
	var p ticketPlan
	active := map[string]string{}
	inactive := map[string]int64{} // terminal|code -> id
	for _, t := range existing {
		if !t.IsActive {
			inactive[t.TerminalID+"|"+t.DenomCode] = t.ID
			continue
		}
		if want[t.TerminalID] == t.DenomCode {
			active[t.TerminalID] = t.DenomCode
			continue
		}
		p.Deactivate = append(p.Deactivate, t.ID)
	}
	for _, terminal := range sortedKeys(want) {
		code := want[terminal]
		if active[terminal] == code {
			continue
		}
		if id, ok := inactive[terminal+"|"+code]; ok {
			p.Reactivate = append(p.Reactivate, id)
			continue
		}
		p.Issue = append(p.Issue, ticketIssue{terminalID: terminal, denomCode: code})
	}
	return p
}

// ticketChange is a ticket issued by applyTicketPlan, for the audit entry (FR6).
type ticketChange struct {
	TerminalID   string `json:"terminal_id"`
	TicketNumber string `json:"ticket_number"`
}

type ticketAudit struct {
	Added       []ticketChange
	Deactivated []string
	Reactivated []string
}

// addTo puts the non-empty update_items keys (FR6.2) into m.
func (a ticketAudit) addTo(m map[string]any) {
	if len(a.Added) > 0 {
		m["tickets_added"] = a.Added
	}
	if len(a.Deactivated) > 0 {
		m["tickets_deactivated"] = a.Deactivated
	}
	if len(a.Reactivated) > 0 {
		m["tickets_reactivated"] = a.Reactivated
	}
}

// applyTicketPlan writes p in an order the partial unique index allows:
// deactivate, reactivate, then issue. Each issue locks (terminal, date) first
// (D2); seq past 999 -> 422 (FR1.3).
func applyTicketPlan(ctx context.Context, q *db.Queries, requestNumber string, replenishDate pgtype.Date, p ticketPlan) (ticketAudit, error) {
	var a ticketAudit
	for _, id := range p.Deactivate {
		n, err := q.SetVendorRequestTicketActive(ctx, db.SetVendorRequestTicketActiveParams{ID: id, IsActive: false})
		if err != nil {
			return a, fmt.Errorf("deactivate ticket %d: %w", id, err)
		}
		a.Deactivated = append(a.Deactivated, n)
	}
	for _, id := range p.Reactivate {
		n, err := q.SetVendorRequestTicketActive(ctx, db.SetVendorRequestTicketActiveParams{ID: id, IsActive: true})
		if err != nil {
			return a, fmt.Errorf("reactivate ticket %d: %w", id, err)
		}
		a.Reactivated = append(a.Reactivated, n)
	}
	for _, it := range p.Issue {
		if err := q.LockTicketSeq(ctx, db.LockTicketSeqParams{TerminalID: it.terminalID, ReplenishDate: replenishDate}); err != nil {
			return a, fmt.Errorf("lock ticket seq %s: %w", it.terminalID, err)
		}
		n, err := q.IssueVendorRequestTicket(ctx, db.IssueVendorRequestTicketParams{
			RequestNumber: requestNumber, TerminalID: it.terminalID, DenomCode: it.denomCode, ReplenishDate: replenishDate,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == "vendor_request_tickets_seq_chk" {
				return a, &ValidationError{Field: "items", Message: fmt.Sprintf("nomor tiket ATM %s tanggal %s sudah mencapai 999",
					it.terminalID, replenishDate.Time.Format("20060102"))}
			}
			return a, fmt.Errorf("issue ticket %s: %w", it.terminalID, err)
		}
		a.Added = append(a.Added, ticketChange{TerminalID: it.terminalID, TicketNumber: n})
	}
	return a, nil
}

// ticketDate is the date segment of a request's tickets: replenish_date, or
// created_at in Asia/Jakarta for old VR- requests without one (FR7.3, same
// rule as the migration 026 backfill).
func ticketDate(req db.VendorRequest) pgtype.Date {
	if req.ReplenishDate.Valid {
		return req.ReplenishDate
	}
	c := req.CreatedAt.Time.In(wibZone)
	return toPgDate(time.Date(c.Year(), c.Month(), c.Day(), 0, 0, 0, 0, time.UTC))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
