//go:build integration

package service

import (
	"errors"
	"sync"
	"testing"
)

// Property 8, race side (spec vendor-pks task 7.5, per type: vendor-pks-type task 4.4):
// parallel submits of mixed create and terminate for one (vendor, type) leave exactly
// one pending vendor_pks change; every other submit of that type gets
// ErrMasterDataChangePending (handler → 409). The partial unique index on
// (payload vendor_id, pks_type_id) decides the race when the service pre-check
// passes in several goroutines at once. The other type stays free: its own submit
// still succeeds afterwards.
func TestVendorPks_ParallelSubmit_OnePendingPerType_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)

	var pksID int64
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
		VALUES ($1, $2, $3, 'PKS aktif', '2026-01-01', '2027-01-01', $4) RETURNING id`,
		f.vendorID, f.typeAID, nextWod(f.tag), f.maker).Scan(&pksID); err != nil {
		t.Fatalf("seed active pks: %v", err)
	}

	const n = 10
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				_, errs[i] = f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeA, nextWod(f.tag)), "127.0.0.1")
				return
			}
			_, errs[i] = f.pks.SubmitTerminate(ctx, f.maker, f.vendorID, pksID, "Putus kontrak", "127.0.0.1")
		}()
	}
	close(start)
	wg.Wait()

	ok := 0
	for i, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrMasterDataChangePending):
		default:
			t.Errorf("submit %d: %v, want nil or ErrMasterDataChangePending", i, err)
		}
	}
	if ok != 1 {
		t.Errorf("successful submits = %d, want 1 (errs %v)", ok, errs)
	}
	pendingOf := func(typeID int64) int {
		return countInt(t, f.pool, `SELECT count(*) FROM master_data_change_requests WHERE entity_type = 'vendor_pks'
			AND status = 'pending' AND (payload->>'vendor_id')::bigint = $1 AND (payload->>'pks_type_id')::bigint = $2`, f.vendorID, typeID)
	}
	if got := pendingOf(f.typeAID); got != 1 {
		t.Errorf("pending vendor_pks changes of type A = %d, want 1", got)
	}

	// Property 8: a pending change of type A does not block type B.
	if _, err := f.pks.SubmitNew(ctx, f.maker, f.vendorID, f.req(f.typeB, nextWod(f.tag)), "127.0.0.1"); err != nil {
		t.Fatalf("SubmitNew type B: %v", err)
	}
	if got := pendingOf(f.typeBID); got != 1 {
		t.Errorf("pending vendor_pks changes of type B = %d, want 1", got)
	}
}
