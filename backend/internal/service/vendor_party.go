package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// Phase 2.2b (.claude/sdlc/cit-send-vendor): one party per (request, vendor
// branch, role). Its content is the snapshot the vendor sees (FR3); it is
// built only from the whitelisted columns of ListRequestPartyRows, so no
// saldo/kapasitas, tier, urgent or price can leak (FR3.3).

const (
	partyRoleReplenish = "replenish"
	partyRoleVault     = "vault"
	vendorPartyEntity  = "vendor_request_vendor_party"
	currencyIDR        = "IDR"
)

// PartyBranch is a vendor branch as shown to the other party.
type PartyBranch struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	VendorID   int64  `json:"vendor_id"`
	VendorName string `json:"vendor_name"`
	Address    string `json:"address,omitempty"`
}

// DenomAmount is an order amount in full IDR (bigint, never float).
type DenomAmount struct {
	Denom  int32 `json:"denom"`
	Amount int64 `json:"amount"`
}

// PartyAtm is one ATM of the party with the counterpart branch: the vault for a
// replenish party, the replenish branch for a vault party.
type PartyAtm struct {
	TerminalID   string        `json:"terminal_id"`
	Lokasi       string        `json:"lokasi"`
	TicketNumber string        `json:"ticket_number"`
	Denoms       []DenomAmount `json:"denoms"`
	Counterpart  PartyBranch   `json:"counterpart"`
}

// PartyContent is vendor_request_vendor_parties.content.
type PartyContent struct {
	Role     string        `json:"role"`
	Branch   PartyBranch   `json:"branch"`
	Atms     []PartyAtm    `json:"atms"`
	Totals   []DenomAmount `json:"totals"`
	Currency string        `json:"currency"`
}

type partyKey struct {
	branchID int64
	role     string
}

// buildPartyContents groups the per-(ATM, denom) rows into one content per
// party. Rows arrive ordered by terminal_id, denom, so ATMs and denoms keep
// that order; totals are sorted by denom.
func buildPartyContents(rows []db.ListRequestPartyRowsRow) map[partyKey]*PartyContent {
	out := map[partyKey]*PartyContent{}
	add := func(k partyKey, self, counterpart PartyBranch, r db.ListRequestPartyRowsRow) {
		c, ok := out[k]
		if !ok {
			c = &PartyContent{Role: k.role, Branch: self, Currency: currencyIDR}
			out[k] = c
		}
		if n := len(c.Atms); n == 0 || c.Atms[n-1].TerminalID != r.TerminalID {
			c.Atms = append(c.Atms, PartyAtm{TerminalID: r.TerminalID, Lokasi: r.LokasiAtm,
				TicketNumber: deref(r.TicketNumber), Counterpart: counterpart})
		}
		atm := &c.Atms[len(c.Atms)-1]
		atm.Denoms = append(atm.Denoms, DenomAmount{Denom: r.Denom, Amount: r.AmountReplenish})
		c.Totals = addDenom(c.Totals, r.Denom, r.AmountReplenish)
	}
	for _, r := range rows {
		replenish := PartyBranch{ID: r.ReplenishBranchID, Code: r.ReplenishBranchCode, Name: r.ReplenishBranchName,
			VendorID: r.ReplenishVendorID, VendorName: r.ReplenishVendorName}
		vault := PartyBranch{ID: r.VaultBranchID, Code: r.VaultBranchCode, Name: r.VaultBranchName,
			VendorID: r.VaultVendorID, VendorName: r.VaultVendorName, Address: r.VaultAddress}
		add(partyKey{r.ReplenishBranchID, partyRoleReplenish}, replenish, vault, r)
		add(partyKey{r.VaultBranchID, partyRoleVault}, vault, replenish, r)
	}
	for _, c := range out {
		sort.Slice(c.Totals, func(i, j int) bool { return c.Totals[i].Denom < c.Totals[j].Denom })
	}
	return out
}

func addDenom(totals []DenomAmount, denom int32, amount int64) []DenomAmount {
	for i := range totals {
		if totals[i].Denom == denom {
			totals[i].Amount += amount
			return totals
		}
	}
	return append(totals, DenomAmount{Denom: denom, Amount: amount})
}

// sameContent compares a stored jsonb snapshot with a freshly built one.
// Postgres jsonb reorders keys, so the stored value is decoded and re-encoded
// through the struct before comparing.
func sameContent(stored []byte, fresh []byte) bool {
	var c PartyContent
	if err := json.Unmarshal(stored, &c); err != nil {
		return false
	}
	norm, err := json.Marshal(c)
	return err == nil && bytes.Equal(norm, fresh)
}

// partyNotice is a party whose vendor must be told (FR2.2): sent, resent or withdrawn.
type partyNotice struct {
	partyID  int64
	branchID int64
	event    string
}

// syncVendorParties creates/refreshes the parties of req after it reached
// sent_to_vendor (FR2.1–FR2.3), inside the caller's tx (request already
// locked, so parties are locked after it, NFR3). Returns the parties to
// notify (FR7.1), also used in the audit entry.
func syncVendorParties(ctx context.Context, q *db.Queries, actorID int64, req db.VendorRequest) ([]partyNotice, error) {
	rows, err := q.ListRequestPartyRows(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("list party rows of request %d: %w", req.ID, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("request %d has no vault assignments to send", req.ID)
	}
	built := buildPartyContents(rows)
	existing, err := q.LockVendorPartiesForRequest(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("lock parties of request %d: %w", req.ID, err)
	}
	forceAll := reapprovedSinceSent(req, existing)

	var notices []partyNotice
	record := func(p db.VendorRequestVendorParty, event string, content []byte) error {
		if err := q.InsertVendorPartyEvent(ctx, db.InsertVendorPartyEventParams{
			PartyID: p.ID, Event: event, ActorID: actorID, Content: content,
		}); err != nil {
			return fmt.Errorf("insert party event: %w", err)
		}
		notices = append(notices, partyNotice{partyID: p.ID, branchID: p.VendorBranchID, event: event})
		return nil
	}

	seen := map[partyKey]bool{}
	for _, p := range existing {
		k := partyKey{p.VendorBranchID, p.Role}
		c, ok := built[k]
		if !ok {
			if p.Status == "withdrawn" {
				continue
			}
			w, err := q.WithdrawVendorParty(ctx, p.ID)
			if err != nil {
				return nil, fmt.Errorf("withdraw party %d: %w", p.ID, err)
			}
			if err := record(w, "withdrawn", nil); err != nil {
				return nil, err
			}
			continue
		}
		seen[k] = true
		fresh, err := json.Marshal(c)
		if err != nil {
			return nil, fmt.Errorf("encode party content: %w", err)
		}
		// Unchanged pending/accepted parties are left alone (FR2.2); a rejected or
		// withdrawn party always gets the request again.
		unchanged := sameContent(p.Content, fresh) && (p.Status == "accepted" || p.Status == "pending")
		if unchanged && !forceAll {
			continue
		}
		r, err := q.ResendVendorParty(ctx, db.ResendVendorPartyParams{Content: fresh, ID: p.ID})
		if err != nil {
			return nil, fmt.Errorf("resend party %d: %w", p.ID, err)
		}
		if err := record(r, "resent", fresh); err != nil {
			return nil, err
		}
	}

	keys := make([]partyKey, 0, len(built))
	for k := range built {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].branchID != keys[j].branchID {
			return keys[i].branchID < keys[j].branchID
		}
		return keys[i].role < keys[j].role
	})
	for _, k := range keys {
		fresh, err := json.Marshal(built[k])
		if err != nil {
			return nil, fmt.Errorf("encode party content: %w", err)
		}
		p, err := q.InsertVendorParty(ctx, db.InsertVendorPartyParams{
			VendorRequestID: req.ID, VendorBranchID: k.branchID, Role: k.role, Content: fresh,
		})
		if err != nil {
			return nil, fmt.Errorf("insert party: %w", err)
		}
		if err := record(p, "sent", fresh); err != nil {
			return nil, err
		}
	}
	return notices, nil
}

// reapprovedSinceSent reports whether req went back through ATM-SPV approve
// (pending_approval -> vault_assignment stamps approved_at) after its parties
// were last sent. That only happens via vendor-return -> revise -> edit, after
// which every party must accept again (FR2.3, Q12). A vault rejection does not
// touch approved_at.
func reapprovedSinceSent(req db.VendorRequest, parties []db.VendorRequestVendorParty) bool {
	if !req.ApprovedAt.Valid || len(parties) == 0 {
		return false
	}
	for _, p := range parties {
		if p.SentAt.Valid && !req.ApprovedAt.Time.After(p.SentAt.Time) {
			return false
		}
	}
	return true
}

var partyNoticeText = map[string][2]string{
	"sent":      {"Request replenish baru", "Request %s (%s) dikirim ke branch Anda. Mohon terima atau tolak."},
	"resent":    {"Request replenish diperbarui", "Request %s (%s) dikirim ulang ke branch Anda. Mohon terima atau tolak lagi."},
	"withdrawn": {"Request replenish ditarik", "Request %s (%s) tidak lagi ditujukan ke branch Anda."},
	"cancelled": {"Request replenish dibatalkan", "Request %s (%s) dibatalkan oleh CIMB Niaga."},
}

// notifyParties sends FR7.1 to every vendor user/PIC in scope of each notice's branch.
func notifyParties(ctx context.Context, n Notifier, q *db.Queries, req db.VendorRequest, notices []partyNotice) error {
	if n == nil {
		return nil
	}
	date := "-"
	if req.ReplenishDate.Valid {
		date = req.ReplenishDate.Time.Format("2006-01-02")
	}
	for _, nt := range notices {
		text, ok := partyNoticeText[nt.event]
		if !ok {
			return fmt.Errorf("no notification text for party event %q", nt.event)
		}
		id := nt.partyID
		if err := n.Send(ctx, q, notification.Message{
			Type: "vendor_order." + nt.event, Title: text[0], Body: fmt.Sprintf(text[1], req.RequestNumber, date),
			Link: fmt.Sprintf("/orders/%d", id), EntityType: vendorPartyEntity, EntityID: &id, Email: true,
		}, notification.Recipients{VendorBranchIDs: []int64{nt.branchID}}); err != nil {
			return fmt.Errorf("notify party %d: %w", id, err)
		}
	}
	return nil
}

// auditParties is the parties part of the request audit entry (FR8.1).
func auditParties(notices []partyNotice) []map[string]any {
	out := make([]map[string]any, 0, len(notices))
	for _, nt := range notices {
		out = append(out, map[string]any{"party_id": nt.partyID, "vendor_branch_id": nt.branchID, "event": nt.event})
	}
	return out
}

// cancelVendorParties: the request was cancelled after it reached the vendors
// (FR6.4). Every party not withdrawn gets a cancelled event and its vendor is
// notified; party status stays as history and the vendor sees the request as
// cancelled (is_canceled) without actions.
func cancelVendorParties(ctx context.Context, q *db.Queries, n Notifier, actor Actor, req db.VendorRequest) error {
	parties, err := q.LockVendorPartiesForRequest(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("lock parties of request %d: %w", req.ID, err)
	}
	var notices []partyNotice
	for _, p := range parties {
		if p.Status == "withdrawn" {
			continue
		}
		if err := q.InsertVendorPartyEvent(ctx, db.InsertVendorPartyEventParams{PartyID: p.ID, Event: "cancelled", ActorID: actor.UserID}); err != nil {
			return fmt.Errorf("insert party event: %w", err)
		}
		notices = append(notices, partyNotice{partyID: p.ID, branchID: p.VendorBranchID, event: "cancelled"})
	}
	return notifyParties(ctx, n, q, req, notices)
}
