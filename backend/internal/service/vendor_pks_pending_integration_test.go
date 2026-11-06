//go:build integration

package service

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// Property 6 (spec vendor-pks task 7.5): parallel submits of mixed create and
// terminate for one vendor leave exactly one pending vendor_pks change; every
// other submit gets ErrMasterDataChangePending (handler → 409). The partial
// unique index on payload->>'vendor_id' decides the race when the service
// pre-check passes in several goroutines at once.
func TestVendorPks_ParallelSubmit_OnePending_Integration(t *testing.T) {
	f := newPksApprovalFixture(t)
	ctx := adminCtx(f.maker)

	var pksID int64
	if err := f.pool.QueryRow(f.ctx, `INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by)
		VALUES ($1, 'PKS aktif', '2026-01-01', '2027-01-01', $2) RETURNING id`, f.vendorID, f.maker).Scan(&pksID); err != nil {
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
				_, errs[i] = f.pks.SubmitNew(ctx, f.maker, f.vendorID, SubmitVendorPksRequest{
					PksName: fmt.Sprintf("PKS %d", i), EffectiveDate: "2027-01-02", ExpiryDate: "2028-01-01",
				}, "127.0.0.1")
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
	if got := countInt(t, f.pool, `SELECT count(*) FROM master_data_change_requests WHERE entity_type = 'vendor_pks'
		AND status = 'pending' AND (payload->>'vendor_id')::bigint = $1`, f.vendorID); got != 1 {
		t.Errorf("pending vendor_pks changes = %d, want 1", got)
	}
}
