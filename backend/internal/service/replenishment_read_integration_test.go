//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// replenishment-realisasi S6 on real Postgres: the read side over rows shaped like the Python job's
// output -- filters, reports, summary, vendor scope (FR4-FR6), range limit (K5), role check (FR4.3).
func TestIntegration_ReplenishmentRead(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	var number string
	var sched time.Time
	if err := f.pool.QueryRow(ctx, `SELECT request_number, replenish_date FROM vendor_requests WHERE id = $1`, f.reqID).Scan(&number, &sched); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE vendor_requests SET status = 'vendor_accepted' WHERE id = $1`, f.reqID); err != nil {
		t.Fatal(err)
	}
	branch2 := f.replenish2(t)
	var late, noOrder int64
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO replenish_classifications (ticket_id, terminal_id, vendor_id, vendor_branch_id, scheduled_date, realized_date,
			realized_time, workday_offset, classification, is_final, has_amount_diff, finalized_at)
		SELECT t.id, t.terminal_id, $3, $4, t.replenish_date, t.replenish_date + 1, '09:30', 1, 'late', true, true, now()
		FROM vendor_request_tickets t WHERE t.request_number = $1 AND t.terminal_id = $2 AND t.is_active
		RETURNING id`, number, f.terminal, f.vendorID, f.branchID).Scan(&late); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO replenish_classifications (terminal_id, vendor_id, vendor_branch_id, realized_date, classification, is_final, finalized_at)
		VALUES ($1, $2, $3, $4, 'no_order', true, now()) RETURNING id`, f.terminal2, f.vendorID, branch2, sched).Scan(&noOrder); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO replenish_classification_amounts (classification_id, denom, order_amount, itm_amount)
		VALUES ($1, 100000, 1000000, 800000), ($2, 100000, 0, 500000)`, late, noOrder); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM replenish_classifications WHERE id = ANY($1)`, []int64{late, noOrder})
	})

	svc := NewReplenishmentReadService(f.pool)
	viewer := Actor{UserID: f.checker.UserID, Role: "ATM-SPV"}
	base := ReplenishmentFilter{From: sched.AddDate(0, 0, -1), To: sched.AddDate(0, 0, 1), VendorID: &f.vendorID}

	l, err := svc.List(ctx, viewer, base)
	if err != nil || l.Total != 2 || len(l.Items) != 2 {
		t.Fatalf("list = %+v, %v; want 2 rows", l, err)
	}
	onlyDiff := base
	onlyDiff.OnlyDiff = true
	if l, err := svc.List(ctx, viewer, onlyDiff); err != nil || l.Total != 1 || l.Items[0].ID != late ||
		l.Items[0].RequestNumber == nil || *l.Items[0].RequestNumber != number || l.Items[0].Amounts[0].Diff != -200000 ||
		l.Items[0].RealizedTime == nil || *l.Items[0].RealizedTime != "09:30:00" {
		t.Fatalf("only_diff list = %+v, %v", l, err)
	}
	if rows, err := svc.AmountMismatch(ctx, viewer, base); err != nil || len(rows) != 1 || rows[0].ID != late {
		t.Fatalf("amount mismatch = %+v, %v", rows, err)
	}
	trips, err := svc.Trips(ctx, viewer, base.From, base.To, &f.vendorID)
	if err != nil || len(trips) != 2 {
		t.Fatalf("trips = %+v, %v; want one per terminal", trips, err)
	}
	sum, err := svc.Summary(ctx, viewer, base.From, base.To, &f.vendorID)
	if err != nil {
		t.Fatal(err)
	}
	byClass := map[string]ClassCount{}
	for _, c := range sum.ByClass {
		byClass[c.Classification] = c
	}
	if byClass["late"].Total != 1 || byClass["late"].AmountDiff != 1 || byClass["no_order"].Total != 1 || len(sum.Instructions) != 1 ||
		sum.Instructions[0].OrderAmount <= 0 {
		t.Fatalf("summary = %+v", sum)
	}

	// K5 + FR4.3
	var ve *ValidationError
	if _, err := svc.List(ctx, viewer, ReplenishmentFilter{From: sched, To: sched.AddDate(0, 0, 31)}); !errors.As(err, &ve) {
		t.Fatalf("32-day range: want ValidationError, got %v", err)
	}
	if _, err := svc.List(ctx, Actor{UserID: f.checker.UserID, Role: "ACM-USER"}, base); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("ACM-USER: want ErrNotAuthorized, got %v", err)
	}

	// FR5.2: the vendor sees its own rows only, narrowed to its branch when pinned.
	vf := ReplenishmentFilter{From: base.From, To: base.To}
	for _, tc := range []struct {
		name string
		va   VendorActor
		want int64
	}{{"vendor-wide", f.wideUser, 2}, {"pinned to branch 1", f.branchUser, 1}, {"other vendor", f.cash2User, 0}} {
		l, err := f.orders.ListRealizations(ctx, tc.va, vf)
		if err != nil || l.Total != tc.want {
			t.Fatalf("%s: total = %v, %v; want %d", tc.name, l, err, tc.want)
		}
	}
}
