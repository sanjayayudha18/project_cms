//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupNumberGeneratorHarness opens a real pool -- unlike the forecast
// integration tests, this one needs genuinely separate committed
// transactions to exercise real concurrency (a single rolled-back tx can't
// race against itself). Every row this harness seeds or a test creates is
// deleted in t.Cleanup, in FK-safe order.
func setupNumberGeneratorHarness(t *testing.T) (pool *pgxpool.Pool, vendorID, createdBy int64, terminalID string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.QueryRow(ctx, "SELECT id FROM users ORDER BY id LIMIT 1").Scan(&createdBy); err != nil {
		t.Fatalf("fetch a seed user id: %v", err)
	}

	marker := "ITESTNUM-" + uuid.NewString()[:8]
	var regionID, locationID, branchID, packageID, atmID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO regions (code, region) VALUES ($1, 'Integration Test Region') RETURNING id`,
		marker).Scan(&regionID); err != nil {
		t.Fatalf("insert region: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		VALUES ($1, 'ATM_SITE', 'Integration Test Location', 'Jl. Test No. 1', 'Jakarta', 'DKI Jakarta', 'ID')
		RETURNING id`, regionID).Scan(&locationID); err != nil {
		t.Fatalf("insert location: %v", err)
	}
	terminalID = marker
	if err := pool.QueryRow(ctx, `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active)
		VALUES ($1, $2, 'ATM', 'TestBrand', 'TestModel', '24_HOURS', 'OFFSITE', true)
		RETURNING id`, terminalID, locationID).Scan(&atmID); err != nil {
		t.Fatalf("insert atm: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, $2) RETURNING id`,
		marker, "Number Test Vendor "+marker).Scan(&vendorID); err != nil {
		t.Fatalf("insert vendor: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region)
		VALUES ($1, $2, 'Number Test Branch', 'Integration Test Region') RETURNING id`,
		vendorID, marker).Scan(&branchID); err != nil {
		t.Fatalf("insert vendor branch: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO vendor_packages_branch (vendor_branch_id, code, priority_class, price)
		VALUES ($1, 'PAKET TEST', 'ALL', 0) RETURNING id`, branchID).Scan(&packageID); err != nil {
		t.Fatalf("insert vendor package: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO atm_vendor_packages (atm_id, vendor_package_id, effective_start_date, effective_end_date, is_active)
		VALUES ($1, $2, now() - interval '30 days', NULL, true)`, atmID, packageID); err != nil {
		t.Fatalf("insert atm_vendor_packages: %v", err)
	}

	vid := vendorID
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM vendor_requests WHERE vendor_id = $1`, vid)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM vendor_request_number_seq WHERE vendor_id = $1`, vid)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM atm_vendor_packages WHERE atm_id = $1`, atmID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM vendor_packages_branch WHERE id = $1`, packageID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM vendor_branches WHERE id = $1`, branchID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM vendors WHERE id = $1`, vid)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM atms WHERE id = $1`, atmID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM locations WHERE id = $1`, locationID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM regions WHERE id = $1`, regionID)
	})

	return pool, vendorID, createdBy, terminalID
}

var requestNumberFormat = regexp.MustCompile(`^REP-[A-Z0-9]{3}-\d{8}-\d{3}$`)

// manualCreateInput builds a manual (Emergency, H+0) create input against the
// harness's seeded ATM/vendor — the simplest path that skips DMAA seeding
// entirely (Req 3.7) while still exercising the real number generator and
// the Q2 single-vendor check.
func manualCreateInput(vendorID int64, terminalID string, denom int32) CreateVendorRequestInput {
	today := jakartaCalendarDate(0)
	return CreateVendorRequestInput{
		ForecastDate:    today,
		ReplenishDate:   today,
		RequestCategory: "emergency",
		IsManual:        true,
		VendorID:        vendorID,
		Items: []ItemInput{{
			TerminalID:      terminalID,
			Denom:           denom,
			AmountReplenish: 1000000,
		}},
	}
}

// TestIntegration_RequestNumber_PerVendorPerDayScoping covers Task 6.2 (Req
// 4.5): the same vendor+date scope increments by 1 for a second create, and
// the number carries the fixed REP-<prefix>-<date>-<seq> format.
func TestIntegration_RequestNumber_PerVendorPerDayScoping(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	first, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if !requestNumberFormat.MatchString(first.RequestNumber) {
		t.Fatalf("request_number %q does not match REP-<3>-<8>-<3> format", first.RequestNumber)
	}
	if got := first.RequestNumber[len(first.RequestNumber)-3:]; got != "001" {
		t.Errorf("first create in a fresh scope: seq = %q, want 001", got)
	}

	second, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 100000))
	if err != nil {
		t.Fatalf("second create (same vendor+date scope): %v", err)
	}
	if got := second.RequestNumber[len(second.RequestNumber)-3:]; got != "002" {
		t.Errorf("second create in the same scope: seq = %q, want 002 (Req 4.5 increments by 1)", got)
	}
	if first.RequestNumber[:len(first.RequestNumber)-3] != second.RequestNumber[:len(second.RequestNumber)-3] {
		t.Errorf("same vendor+date scope should share the prefix+date segment: %q vs %q", first.RequestNumber, second.RequestNumber)
	}
}

// TestIntegration_RequestNumber_ConcurrentCreatesGetDistinctNumbers covers
// Task 6.2 (Req 4.6): N concurrent creates in one (vendor, date) scope must
// all succeed with N distinct sequence numbers — the atomic
// INSERT ... ON CONFLICT DO UPDATE ... RETURNING serializes on the row lock
// instead of racing like the old MAX(SUBSTRING(...)) approach would.
func TestIntegration_RequestNumber_ConcurrentCreatesGetDistinctNumbers(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	const n = 10
	numbers := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, int32(1000+i)))
			errs[i] = err
			if err == nil {
				numbers[i] = result.RequestNumber
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent create %d failed: %v", i, err)
		}
		if seen[numbers[i]] {
			t.Fatalf("duplicate request_number %q from concurrent creates", numbers[i])
		}
		seen[numbers[i]] = true
	}
	if len(seen) != n {
		t.Fatalf("expected %d distinct request numbers, got %d", n, len(seen))
	}
}

// TestIntegration_RequestNumber_ExhaustionRejectsWithoutPersisting covers
// Task 6.2 (Req 4.7): once a (vendor, date) scope's sequence is at 999,
// the next create is rejected with ErrNumberExhausted and nothing is
// persisted — no new vendor_requests row.
func TestIntegration_RequestNumber_ExhaustionRejectsWithoutPersisting(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	today := jakartaCalendarDate(0)
	if _, err := pool.Exec(ctx, `
		INSERT INTO vendor_request_number_seq (vendor_id, seq_date, last_seq)
		VALUES ($1, $2, 999)
		ON CONFLICT (vendor_id, seq_date) DO UPDATE SET last_seq = 999`,
		vendorID, today); err != nil {
		t.Fatalf("pre-seed sequence at 999: %v", err)
	}

	var countBefore int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vendor_requests WHERE vendor_id = $1`, vendorID).Scan(&countBefore); err != nil {
		t.Fatalf("count before: %v", err)
	}

	_, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err == nil {
		t.Fatal("expected ErrNumberExhausted, got nil")
	}
	if !errors.Is(err, ErrNumberExhausted) {
		t.Fatalf("expected ErrNumberExhausted, got %v", err)
	}

	var countAfter int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vendor_requests WHERE vendor_id = $1`, vendorID).Scan(&countAfter); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if countAfter != countBefore {
		t.Errorf("vendor_requests row count changed from %d to %d — exhaustion must not persist anything", countBefore, countAfter)
	}
}

// TestIntegration_RequestNumber_RetryExhaustionReturnsErrNumberGeneration
// covers Task 6.4 (Req 4.6): if every one of the 5 attempts collides with an
// already-taken request_number (unique-violation retry path in
// createWithRetryingNumber), Create gives up with ErrNumberGeneration and
// persists nothing beyond the 5 pre-seeded rows forcing the collisions.
func TestIntegration_RequestNumber_RetryExhaustionReturnsErrNumberGeneration(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	var vendorCode string
	if err := pool.QueryRow(ctx, "SELECT code FROM vendors WHERE id = $1", vendorID).Scan(&vendorCode); err != nil {
		t.Fatalf("fetch vendor code: %v", err)
	}
	prefix := fallbackVendorPrefix(vendorCode)
	dateSeg := jakartaCalendarDate(0).Format("20060102")

	// Pre-seed a row for every seq createWithRetryingNumber's 5 attempts will
	// try, so each attempt's INSERT collides on vendor_requests_number_uq.
	for seq := 1; seq <= 5; seq++ {
		blocker := fmt.Sprintf("REP-%s-%s-%03d", prefix, dateSeg, seq)
		if _, err := pool.Exec(ctx, `
			INSERT INTO vendor_requests (request_number, forecast_date, created_by, vendor_id)
			VALUES ($1, $2, $3, $4)`,
			blocker, jakartaCalendarDate(0), createdBy, vendorID); err != nil {
			t.Fatalf("pre-seed blocker %q: %v", blocker, err)
		}
	}

	var countBefore int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vendor_requests WHERE vendor_id = $1`, vendorID).Scan(&countBefore); err != nil {
		t.Fatalf("count before: %v", err)
	}

	_, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err == nil {
		t.Fatal("expected ErrNumberGeneration, got nil")
	}
	if !errors.Is(err, ErrNumberGeneration) {
		t.Fatalf("expected ErrNumberGeneration, got %v", err)
	}

	var countAfter int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vendor_requests WHERE vendor_id = $1`, vendorID).Scan(&countAfter); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if countAfter != countBefore {
		t.Errorf("vendor_requests row count changed from %d to %d — exhausted retries must not persist anything new", countBefore, countAfter)
	}
}

// TestIntegration_RequestNumber_LegacyFormatNotRewritten covers Task 6.4
// (Req 4.9): an existing row using the old no-separator REP<prefix><date><seq>
// format is read back unchanged — the new hyphenated generator only shapes
// numbers assembled by createWithRetryingNumber for new creates, it never
// touches or reformats rows already in the table.
func TestIntegration_RequestNumber_LegacyFormatNotRewritten(t *testing.T) {
	pool, vendorID, createdBy, _ := setupNumberGeneratorHarness(t)
	ctx := context.Background()
	svc := NewVendorRequestService(pool)

	legacyNumber := "REPTAG20250101001"
	var id int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO vendor_requests (request_number, forecast_date, created_by, vendor_id)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		legacyNumber, jakartaCalendarDate(0), createdBy, vendorID).Scan(&id); err != nil {
		t.Fatalf("seed legacy-format row: %v", err)
	}

	detail, err := svc.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get legacy row: %v", err)
	}
	if detail.RequestNumber != legacyNumber {
		t.Errorf("legacy request_number was rewritten: got %q, want unchanged %q", detail.RequestNumber, legacyNumber)
	}
}

// TestIntegration_RequestNumber_SearchMatchesHyphenatedFullAndPartial covers
// Task 6.4 (Req 4.9): List's request_number filter (ILIKE substring) matches
// a hyphenated number both by the full string and by a partial segment.
func TestIntegration_RequestNumber_SearchMatchesHyphenatedFullAndPartial(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}

	created, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	full := created.RequestNumber
	if !requestNumberFormat.MatchString(full) {
		t.Fatalf("seeded request_number %q does not match REP-<3>-<8>-<3> format", full)
	}
	// A partial segment spanning the date-seq hyphen boundary, e.g. the last
	// 6 characters ("<4 date digits>-<seq>"), still contains a literal '-'.
	partial := full[len(full)-6:]

	for _, term := range []string{full, partial} {
		result, err := svc.List(ctx, ListVendorRequestParams{
			CreatedBy:     createdBy,
			RequestNumber: term,
			Page:          1,
			PageSize:      50,
		})
		if err != nil {
			t.Fatalf("List(request_number=%q): %v", term, err)
		}
		found := false
		for _, row := range result.Data {
			if row.ID == created.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("List(request_number=%q) did not return created request %q", term, full)
		}
	}
}
