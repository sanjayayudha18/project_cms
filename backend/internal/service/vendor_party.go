package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
)

// Phase 2.2b (.claude/sdlc/cit-send-vendor): one party per (request, vendor
// branch, role). Its content is the snapshot the vendor sees (FR3, revised by
// cit-vault-select FR7); it is built only from the whitelisted columns of
// ListRequestPartyAtmRows / ListRequestPartyVaultRows, so no saldo/kapasitas,
// tier, urgent, buffer reason or price can leak (FR7.4).

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

// PartyAtm is one ATM of a replenish party with its order (no vault per ATM).
type PartyAtm struct {
	TerminalID   string        `json:"terminal_id"`
	Lokasi       string        `json:"lokasi"`
	TicketNumber string        `json:"ticket_number"`
	Denoms       []DenomAmount `json:"denoms"`
}

// PartyVaultRef is a vendor vault as shown to vendors (address: vault, else branch; U8).
type PartyVaultRef struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	Address string `json:"address,omitempty"`
}

// PartyVault is one allocated vault of the request with its tiket pemenuhan
// vault and amounts (cit-vault-select FR7.2/FR7.3).
type PartyVault struct {
	TicketNumber string        `json:"ticket_number"`
	Vault        PartyVaultRef `json:"vault"`
	Branch       PartyBranch   `json:"branch"`
	Denoms       []DenomAmount `json:"denoms"`
}

// PartyContent is vendor_request_vendor_parties.content. Replenish party: its
// own ATMs + every vault of the request (V6); totals = its ATMs' order. Vault
// party: its own vaults + the request's replenish branches; totals = its vaults.
type PartyContent struct {
	Role              string        `json:"role"`
	Branch            PartyBranch   `json:"branch"`
	Atms              []PartyAtm    `json:"atms"`
	Vaults            []PartyVault  `json:"vaults"`
	ReplenishBranches []PartyBranch `json:"replenish_branches"`
	Totals            []DenomAmount `json:"totals"`
	Currency          string        `json:"currency"`
}

type partyKey struct {
	branchID int64
	role     string
}

func newPartyContent(role string, self PartyBranch) *PartyContent {
	return &PartyContent{Role: role, Branch: self, Atms: []PartyAtm{}, Vaults: []PartyVault{},
		ReplenishBranches: []PartyBranch{}, Totals: []DenomAmount{}, Currency: currencyIDR}
}

// requestVaults groups the per-(vault, denom) rows into vaults, keeping the
// row order (branch code, vault code, denom).
func requestVaults(rows []db.ListRequestPartyVaultRowsRow) []PartyVault {
	out := []PartyVault{}
	for _, r := range rows {
		if n := len(out); n == 0 || out[n-1].Vault.ID != r.VendorVaultID {
			out = append(out, PartyVault{TicketNumber: deref(r.TicketNumber),
				Vault:  PartyVaultRef{ID: r.VendorVaultID, Code: r.VaultCode, Address: r.VaultAddress},
				Branch: PartyBranch{ID: r.VaultBranchID, Code: r.VaultBranchCode, Name: r.VaultBranchName, VendorID: r.VaultVendorID, VendorName: r.VaultVendorName}})
		}
		v := &out[len(out)-1]
		v.Denoms = append(v.Denoms, DenomAmount{Denom: r.Denom, Amount: r.Amount})
	}
	return out
}

// buildPartyContents builds one content per party (cit-vault-select FR7).
// ATM rows arrive ordered by terminal_id, denom; totals are sorted by denom.
func buildPartyContents(atms []db.ListRequestPartyAtmRowsRow, vaultRows []db.ListRequestPartyVaultRowsRow) map[partyKey]*PartyContent {
	vaults := requestVaults(vaultRows)
	out := map[partyKey]*PartyContent{}
	replenishBranches := []PartyBranch{}
	for _, r := range atms {
		k := partyKey{r.ReplenishBranchID, partyRoleReplenish}
		c, ok := out[k]
		if !ok {
			b := PartyBranch{ID: r.ReplenishBranchID, Code: r.ReplenishBranchCode, Name: r.ReplenishBranchName,
				VendorID: r.ReplenishVendorID, VendorName: r.ReplenishVendorName}
			c = newPartyContent(partyRoleReplenish, b)
			c.Vaults = slices.Clone(vaults)
			out[k] = c
			replenishBranches = append(replenishBranches, b)
		}
		if n := len(c.Atms); n == 0 || c.Atms[n-1].TerminalID != r.TerminalID {
			c.Atms = append(c.Atms, PartyAtm{TerminalID: r.TerminalID, Lokasi: r.LokasiAtm, TicketNumber: deref(r.TicketNumber)})
		}
		atm := &c.Atms[len(c.Atms)-1]
		atm.Denoms = append(atm.Denoms, DenomAmount{Denom: r.Denom, Amount: r.AmountReplenish})
		c.Totals = addDenom(c.Totals, r.Denom, r.AmountReplenish)
	}
	sort.Slice(replenishBranches, func(i, j int) bool { return replenishBranches[i].Code < replenishBranches[j].Code })
	for _, v := range vaults {
		k := partyKey{v.Branch.ID, partyRoleVault}
		c, ok := out[k]
		if !ok {
			c = newPartyContent(partyRoleVault, v.Branch)
			c.ReplenishBranches = slices.Clone(replenishBranches)
			out[k] = c
		}
		c.Vaults = append(c.Vaults, v)
		for _, d := range v.Denoms {
			c.Totals = addDenom(c.Totals, d.Denom, d.Amount)
		}
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
	role     string // party role (replenish|vault): decides which vendor roles/PICs are told
	event    string
}

// syncVendorParties creates/refreshes the parties of req after it reached
// sent_to_vendor (FR2.1–FR2.3), inside the caller's tx (request already
// locked, so parties are locked after it, NFR3). Returns the parties to
// notify (FR7.1), also used in the audit entry.
func syncVendorParties(ctx context.Context, q *db.Queries, actorID int64, req db.VendorRequest) ([]partyNotice, error) {
	atms, err := q.ListRequestPartyAtmRows(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("list party atm rows of request %d: %w", req.ID, err)
	}
	vaults, err := q.ListRequestPartyVaultRows(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("list party vault rows of request %d: %w", req.ID, err)
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("request %d has no vault allocations to send", req.ID)
	}
	built := buildPartyContents(atms, vaults)
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
		notices = append(notices, partyNotice{partyID: p.ID, branchID: p.VendorBranchID, role: p.Role, event: event})
		return nil
	}

	seen := map[partyKey]bool{}
	for _, p := range existing {
		k := partyKey{p.VendorBranchID, p.Role}
		c, ok := built[k]
		if ok {
			seen[k] = true
		}
		if err := refreshParty(ctx, q, p, c, ok, forceAll, record); err != nil {
			return nil, err
		}
	}

	for _, k := range newPartyKeys(built, seen) {
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

// refreshParty brings one existing party in line with the request: withdrawn
// when it is no longer part of it (wanted=false), resent when its content
// changed, it was rejected/withdrawn, or forceAll; otherwise left alone.
func refreshParty(ctx context.Context, q *db.Queries, p db.VendorRequestVendorParty, c *PartyContent, wanted, forceAll bool,
	record func(db.VendorRequestVendorParty, string, []byte) error) error {
	if !wanted {
		if p.Status == "withdrawn" {
			return nil
		}
		w, err := q.WithdrawVendorParty(ctx, p.ID)
		if err != nil {
			return fmt.Errorf("withdraw party %d: %w", p.ID, err)
		}
		return record(w, "withdrawn", nil)
	}
	fresh, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode party content: %w", err)
	}
	// Unchanged pending/accepted parties are left alone (FR2.2); a rejected or
	// withdrawn party always gets the request again.
	unchanged := sameContent(p.Content, fresh) && (p.Status == "accepted" || p.Status == "pending")
	if unchanged && !forceAll {
		return nil
	}
	r, err := q.ResendVendorParty(ctx, db.ResendVendorPartyParams{Content: fresh, ID: p.ID})
	if err != nil {
		return fmt.Errorf("resend party %d: %w", p.ID, err)
	}
	return record(r, "resent", fresh)
}

// newPartyKeys lists the built parties not yet stored, ordered by branch then role.
func newPartyKeys(built map[partyKey]*PartyContent, seen map[partyKey]bool) []partyKey {
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
	return keys
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
		}, notification.Recipients{VendorParties: []notification.VendorPartyTarget{{
			BranchID: nt.branchID, UserRoles: vendorrole.ForParty(nt.role), PicRole: nt.role,
		}}}); err != nil {
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
		notices = append(notices, partyNotice{partyID: p.ID, branchID: p.VendorBranchID, role: p.Role, event: "cancelled"})
	}
	return notifyParties(ctx, n, q, req, notices)
}
