//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// replenish-ticket (.claude/sdlc/replenish-ticket/spec.md) against real
// Postgres (needs migration 026). Reuses setupNumberGeneratorHarness: one
// vendor, one cabang with region_code ITEST, one ATM.

// seedDmaaRows makes the harness ATM valid for UpdateItems (resolveItems needs
// a dmaa_atm_forecast row per (terminal, periode, denom)).
func seedDmaaRows(t *testing.T, pool *pgxpool.Pool, terminalID string, denoms ...int32) {
	t.Helper()
	ctx := context.Background()
	var fileID int64
	if err := pool.QueryRow(ctx, `INSERT INTO dmaa_files (name, status, checksum) VALUES ($1, 'completed', $1) RETURNING id`,
		"itest-"+uuid.NewString()+".csv").Scan(&fileID); err != nil {
		t.Fatalf("insert dmaa_files: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM dmaa_atm_forecast WHERE dmaa_file_id = $1`, fileID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM dmaa_files WHERE id = $1`, fileID)
	})
	for _, d := range denoms {
		if _, err := pool.Exec(ctx, `
			INSERT INTO dmaa_atm_forecast (terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund)
			VALUES ($1, $2, $3, $4, 1000000, 0)`, terminalID, fileID, jakartaCalendarDate(0), d); err != nil {
			t.Fatalf("insert dmaa_atm_forecast: %v", err)
		}
	}
}

func ticketNumbers(d *VendorRequestDetail) []string {
	out := make([]string, len(d.Atms))
	for i, a := range d.Atms {
		out[i] = a.TicketNumber
	}
	return out
}

func TestIntegration_Ticket_CreateNumbersAndRegion(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}
	date := jakartaCalendarDate(0).Format("20060102")

	first, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.Contains(first.RequestNumber, "-ITEST-"+date+"-001") || first.RegionCode == nil || *first.RegionCode != "ITEST" {
		t.Fatalf("request_number %q region %v, want ...-ITEST-%s-001 / ITEST", first.RequestNumber, first.RegionCode, date)
	}
	want := terminalID + "_50K_" + date + "_001"
	if got := ticketNumbers(first); len(got) != 1 || got[0] != want || first.Items[0].TicketNumber != want {
		t.Fatalf("tickets %v item %q, want %q", got, first.Items[0].TicketNumber, want)
	}

	// Second request for the same ATM + date: NNN is global per (ATM, date).
	in := manualCreateInput(vendorID, terminalID, 50000)
	in.Items = append(in.Items, ItemInput{TerminalID: terminalID, Denom: 100000, AmountReplenish: 1000000})
	second, err := svc.Create(ctx, actor, in)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if got := ticketNumbers(second); len(got) != 1 || got[0] != terminalID+"_MIX_"+date+"_002" {
		t.Fatalf("second tickets %v, want one MIX _002", got)
	}

	// FK: a ticket cannot point at a request number that doesn't exist.
	if _, err := pool.Exec(ctx, `INSERT INTO vendor_request_tickets (request_number, terminal_id, denom_code, replenish_date, seq, ticket_number)
		VALUES ('NOPE-'||$1, $1, '50K', CURRENT_DATE, 999, 'x-'||$1)`, terminalID); err == nil {
		t.Fatal("ticket with unknown request_number accepted, want FK violation")
	}
}

// FR1.3: an ATM whose NNN for the date already reached 999 -> 422, nothing persisted.
func TestIntegration_Ticket_SeqExhaustedRejects(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	first, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE vendor_request_tickets SET seq = 999 WHERE request_number = $1`, first.RequestNumber); err != nil {
		t.Fatalf("push seq to 999: %v", err)
	}
	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM vendor_requests WHERE vendor_id = $1`, vendorID).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}

	_, err = svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Message, "sudah mencapai 999") {
		t.Fatalf("err = %v, want 422 sudah mencapai 999", err)
	}
	var after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM vendor_requests WHERE vendor_id = $1`, vendorID).Scan(&after); err != nil {
		t.Fatalf("count: %v", err)
	}
	if after != before {
		t.Fatalf("requests %d -> %d, want the whole create rolled back", before, after)
	}
}

func TestIntegration_Ticket_RegionRules(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}
	setCode := func(code any) {
		if _, err := pool.Exec(ctx, `UPDATE vendor_branches SET region_code = $2 WHERE vendor_id = $1`, vendorID, code); err != nil {
			t.Fatalf("set region_code: %v", err)
		}
	}

	setCode(nil)
	_, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Message, "belum punya kode region") {
		t.Fatalf("create with cabang without code: err = %v, want 422 belum punya kode region", err)
	}

	// The counter is per (vendor, region_code, date): a new code starts at 001.
	setCode("ITEST")
	if _, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000)); err != nil {
		t.Fatalf("create ITEST: %v", err)
	}
	setCode("ITESTB")
	other, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create ITESTB: %v", err)
	}
	if !strings.Contains(other.RequestNumber, "-ITESTB-") || !strings.HasSuffix(other.RequestNumber, "-001") {
		t.Fatalf("request_number %q, want -ITESTB- ... -001", other.RequestNumber)
	}
}

func TestIntegration_Ticket_UpdateItemsDeactivatesAndReactivates(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	seedDmaaRows(t, pool, terminalID, 50000, 100000)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}
	date := jakartaCalendarDate(0).Format("20060102")
	item := func(denom int32) ItemInput {
		return ItemInput{TerminalID: terminalID, PeriodePred: jakartaCalendarDate(0), Denom: denom, AmountReplenish: 1000000}
	}

	created, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t50 := terminalID + "_50K_" + date + "_001"

	// Same code -> unchanged.
	got, err := svc.UpdateItems(ctx, actor, created.ID, []ItemInput{item(50000)})
	if err != nil || ticketNumbers(got)[0] != t50 {
		t.Fatalf("update same code: %v %v, want %s", err, got, t50)
	}
	// 50K -> MIX: old ticket inactive, new number _002.
	got, err = svc.UpdateItems(ctx, actor, created.ID, []ItemInput{item(50000), item(100000)})
	if err != nil || ticketNumbers(got)[0] != terminalID+"_MIX_"+date+"_002" {
		t.Fatalf("update to MIX: %v %v", err, got)
	}
	// Back to 50K: the old ticket comes back with the same number.
	got, err = svc.UpdateItems(ctx, actor, created.ID, []ItemInput{item(50000)})
	if err != nil || ticketNumbers(got)[0] != t50 {
		t.Fatalf("update back to 50K: %v %v, want %s", err, got, t50)
	}

	var active, total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE is_active), count(*) FROM vendor_request_tickets WHERE request_number = $1`,
		created.RequestNumber).Scan(&active, &total); err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if active != 1 || total != 2 {
		t.Fatalf("tickets active=%d total=%d, want 1/2", active, total)
	}
}
