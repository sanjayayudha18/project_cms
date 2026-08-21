//go:build integration

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: atm-portal
// Property 3: Search filter correctness
// Property 4: Status filter correctness
// Property 5: Active-only invariant
// Property 6: Pagination correctness
// Property 7: Sort order correctness
//
// Properties 3, 6, 7 run read-only against the live `cms` dataset (1904
// active ATMs, ~710 with itm_cashpos data) — no writes needed. Properties 4
// and 5 need custom-seeded rows: the real dataset is 100% status
// "unconfigured" (no thresholds configured yet) and 100% is_active=true /
// deleted_at IS NULL, so neither would exercise anything without seeded
// data covering the other cases. Those two insert inside a rolled-back
// transaction per iteration, same pattern as atm_portal_property_test.go.

// containsFold reports whether s contains substr, case-insensitively.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TestProperty3_SearchFilterCorrectness validates Property 3: for any
// non-empty search term, every ATM in the response contains that term
// (case-insensitively) in its terminal_id or location_name.
func TestProperty3_SearchFilterCorrectness(t *testing.T) {
	pool := propertyTestPool(t)
	ctx := context.Background()
	svc := NewAtmPortalService(db.New(pool))

	rows, err := pool.Query(ctx, "SELECT terminal_id FROM atms ORDER BY random() LIMIT 20")
	if err != nil {
		t.Fatalf("fetching search candidates: %v", err)
	}
	candidates := []string{"ZZZZ-NO-MATCH-JUNK-TERM-" + uuid.NewString()[:8]}
	for rows.Next() {
		var terminalID string
		if err := rows.Scan(&terminalID); err != nil {
			t.Fatalf("scanning terminal_id: %v", err)
		}
		candidates = append(candidates, terminalID)
	}
	rows.Close()

	rapid.Check(t, func(rt *rapid.T) {
		term := rapid.SampledFrom(candidates).Draw(rt, "term")

		result, err := svc.ListATMs(ctx, ListATMsParams{
			Page: 1, PageSize: 100, Status: "all", Search: term,
			SortBy: "terminal_id", SortOrder: "asc",
		})
		if err != nil {
			rt.Fatalf("query: %v", err)
		}
		for _, atm := range result.Data {
			if !containsFold(atm.TerminalID, term) && !containsFold(atm.LocationName, term) {
				rt.Fatalf("term %q matched neither terminal_id %q nor location_name %q", term, atm.TerminalID, atm.LocationName)
			}
		}
	})
}

// TestProperty4_StatusFilterCorrectness validates Property 4: for any
// status filter value other than "all", every ATM in the response has that
// computed status. Seeds one ATM per status (with randomized threshold and
// refund_total magnitudes, keeping the critical < low < normal relationship
// that produces each status) under a unique marker, so results can be
// scoped via search and checked non-vacuously.
func TestProperty4_StatusFilterCorrectness(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	statuses := []string{"unconfigured", "no_data", "critical", "low", "normal"}

	rapid.Check(t, func(rt *rapid.T) {
		low := round2(rapid.Float64Range(10, 1_000_000).Draw(rt, "low"))
		critical := round2(rapid.Float64Range(1, low-1).Draw(rt, "critical"))

		marker := "P4" + uuid.NewString()[:8]

		tx, err := pool.Begin(ctx)
		if err != nil {
			rt.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		locationID, err := insertTestLocation(ctx, tx, regionID)
		if err != nil {
			rt.Fatalf("insert location: %v", err)
		}

		for i, status := range statuses {
			terminalID := fmt.Sprintf("%s-%d", marker, i)
			var lowThreshold, criticalThreshold *float64
			var hasCashpos bool
			var refundTotal float64

			switch status {
			case "unconfigured":
				// lowThreshold stays nil
			case "no_data":
				lowThreshold = &low
				hasCashpos = false
			case "critical":
				lowThreshold, criticalThreshold = &low, &critical
				hasCashpos = true
				refundTotal = round2(critical * rapid.Float64Range(0, 1).Draw(rt, fmt.Sprintf("criticalFrac%d", i)))
			case "low":
				lowThreshold, criticalThreshold = &low, &critical
				hasCashpos = true
				refundTotal = round2(critical + (low-critical)*rapid.Float64Range(0.01, 1).Draw(rt, fmt.Sprintf("lowFrac%d", i)))
			case "normal":
				lowThreshold, criticalThreshold = &low, &critical
				hasCashpos = true
				refundTotal = round2(low + low*rapid.Float64Range(0.01, 1).Draw(rt, fmt.Sprintf("normalFrac%d", i)))
			}

			if err := insertTestATM(ctx, tx, terminalID, locationID, lowThreshold, criticalThreshold); err != nil {
				rt.Fatalf("insert atm %s: %v", terminalID, err)
			}
			if hasCashpos {
				fileID, err := insertTestCashposFile(ctx, tx)
				if err != nil {
					rt.Fatalf("insert cashpos file: %v", err)
				}
				if err := insertTestCashpos(ctx, tx, fileID, terminalID, time.Now(), "10:00:00", refundTotal); err != nil {
					rt.Fatalf("insert cashpos: %v", err)
				}
			}
		}

		queries := db.New(tx)
		svc := NewAtmPortalService(queries)

		for _, want := range statuses {
			result, err := svc.ListATMs(ctx, ListATMsParams{
				Page: 1, PageSize: 10, Status: want, Search: marker,
				SortBy: "terminal_id", SortOrder: "asc",
			})
			if err != nil {
				rt.Fatalf("query status=%s: %v", want, err)
			}

			foundSeeded := false
			for _, atm := range result.Data {
				if atm.Status != want {
					rt.Fatalf("status filter %q returned atm %q with status %q", want, atm.TerminalID, atm.Status)
				}
				if strings.Contains(atm.TerminalID, marker) {
					foundSeeded = true
				}
			}
			if !foundSeeded {
				rt.Fatalf("status filter %q did not return the seeded atm for that status (marker %s)", want, marker)
			}
		}

		// Multi-select (Req 89.3): a comma-separated status filter must
		// return the union of the requested statuses, not just the first.
		multiResult, err := svc.ListATMs(ctx, ListATMsParams{
			Page: 1, PageSize: 10, Status: "critical,low", Search: marker,
			SortBy: "terminal_id", SortOrder: "asc",
		})
		if err != nil {
			rt.Fatalf("query status=critical,low: %v", err)
		}
		foundCritical, foundLow := false, false
		for _, atm := range multiResult.Data {
			if atm.Status != "critical" && atm.Status != "low" {
				rt.Fatalf("status filter \"critical,low\" returned atm %q with status %q", atm.TerminalID, atm.Status)
			}
			if atm.Status == "critical" {
				foundCritical = true
			}
			if atm.Status == "low" {
				foundLow = true
			}
		}
		if !foundCritical || !foundLow {
			rt.Fatalf("status filter \"critical,low\" did not return both seeded statuses (marker %s): foundCritical=%v foundLow=%v", marker, foundCritical, foundLow)
		}
	})
}

// TestProperty5_ActiveOnlyInvariant validates Property 5: every ATM
// appearing in any response has is_active=true and deleted_at IS NULL,
// regardless of request parameters. Seeds a batch of ATMs with randomized
// is_active/deleted_at combinations under a unique marker; verifies the
// inactive/deleted ones never appear even though they match the search
// term, while confirming the active ones do (non-vacuous check).
func TestProperty5_ActiveOnlyInvariant(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		marker := "P5" + uuid.NewString()[:8]
		n := rapid.IntRange(3, 8).Draw(rt, "n")

		tx, err := pool.Begin(ctx)
		if err != nil {
			rt.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		locationID, err := insertTestLocation(ctx, tx, regionID)
		if err != nil {
			rt.Fatalf("insert location: %v", err)
		}

		activeTerminals := map[string]bool{}
		for i := 0; i < n; i++ {
			terminalID := fmt.Sprintf("%s-%d", marker, i)
			isActive := rapid.Bool().Draw(rt, fmt.Sprintf("isActive%d", i))
			isDeleted := rapid.Bool().Draw(rt, fmt.Sprintf("isDeleted%d", i))

			if err := insertTestATMWithLifecycle(ctx, tx, terminalID, locationID, isActive, isDeleted); err != nil {
				rt.Fatalf("insert atm %s: %v", terminalID, err)
			}
			if isActive && !isDeleted {
				activeTerminals[terminalID] = true
			}
		}

		queries := db.New(tx)
		svc := NewAtmPortalService(queries)

		sortBy := rapid.SampledFrom([]string{"terminal_id", "location", "last_replenish_date", "refund_total", "replenish_total", "status"}).Draw(rt, "sortBy")
		sortOrder := rapid.SampledFrom([]string{"asc", "desc"}).Draw(rt, "sortOrder")

		result, err := svc.ListATMs(ctx, ListATMsParams{
			Page: 1, PageSize: 20, Status: "all", Search: marker,
			SortBy: sortBy, SortOrder: sortOrder,
		})
		if err != nil {
			rt.Fatalf("query: %v", err)
		}

		seenActive := map[string]bool{}
		for _, atm := range result.Data {
			if !activeTerminals[atm.TerminalID] {
				rt.Fatalf("inactive/deleted atm %q appeared in response (active set: %v)", atm.TerminalID, activeTerminals)
			}
			seenActive[atm.TerminalID] = true
		}
		for terminalID := range activeTerminals {
			if !seenActive[terminalID] {
				rt.Fatalf("active atm %q was seeded but never returned", terminalID)
			}
		}
	})
}

// insertTestATMWithLifecycle inserts a fresh atms row with the given
// is_active/deleted_at state, for Property 5.
func insertTestATMWithLifecycle(ctx context.Context, tx pgx.Tx, terminalID string, locationID int64, isActive, isDeleted bool) error {
	var deletedAt *time.Time
	if isDeleted {
		t := time.Now()
		deletedAt = &t
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, is_active, deleted_at)
		VALUES ($1, $2, 'ATM', 'Hyosung', 'TestModel', '24H', 'ONSITE', $3, $4)
	`, terminalID, locationID, isActive, deletedAt)
	return err
}

func TestProperty6_PaginationCorrectness(t *testing.T) {
	pool := propertyTestPool(t)
	ctx := context.Background()
	svc := NewAtmPortalService(db.New(pool))

	baseline, err := svc.ListATMs(ctx, ListATMsParams{
		Page: 1, PageSize: 1, Status: "all", SortBy: "terminal_id", SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("baseline query: %v", err)
	}
	total := int(baseline.Total)
	if total == 0 {
		t.Skip("no active atms in cms — nothing to paginate")
	}

	rapid.Check(t, func(rt *rapid.T) {
		pageSize := rapid.IntRange(1, 100).Draw(rt, "pageSize")
		maxPage := total/pageSize + 2
		page := rapid.IntRange(1, maxPage).Draw(rt, "page")

		result, err := svc.ListATMs(ctx, ListATMsParams{
			Page: page, PageSize: pageSize, Status: "all", SortBy: "terminal_id", SortOrder: "asc",
		})
		if err != nil {
			rt.Fatalf("query page=%d page_size=%d: %v", page, pageSize, err)
		}

		wantLen := min(pageSize, max(0, total-(page-1)*pageSize))
		if len(result.Data) != wantLen {
			rt.Fatalf("page=%d page_size=%d total=%d: got %d rows, want %d", page, pageSize, total, len(result.Data), wantLen)
		}
		if int(result.Total) != total {
			rt.Fatalf("total changed across pages: got %d, want %d (page=%d)", result.Total, total, page)
		}
	})
}

func TestProperty7_SortOrderCorrectness(t *testing.T) {
	pool := propertyTestPool(t)
	ctx := context.Background()
	svc := NewAtmPortalService(db.New(pool))

	sortBys := []string{"terminal_id", "location", "last_replenish_date", "refund_total", "replenish_total", "status"}

	rapid.Check(t, func(rt *rapid.T) {
		sortBy := rapid.SampledFrom(sortBys).Draw(rt, "sortBy")
		sortOrder := rapid.SampledFrom([]string{"asc", "desc"}).Draw(rt, "sortOrder")
		pageSize := rapid.IntRange(1, 100).Draw(rt, "pageSize")
		page := rapid.IntRange(1, 20).Draw(rt, "page")

		result, err := svc.ListATMs(ctx, ListATMsParams{
			Page: page, PageSize: pageSize, Status: "all", SortBy: sortBy, SortOrder: sortOrder,
		})
		if err != nil {
			rt.Fatalf("query: %v", err)
		}

		for i := 0; i+1 < len(result.Data); i++ {
			a, b := result.Data[i], result.Data[i+1]
			ok, reason, err := sortedPairOK(ctx, pool, sortBy, sortOrder, a, b)
			if err != nil {
				rt.Fatalf("checking pair order: %v", err)
			}
			if !ok {
				rt.Fatalf("pair (%d,%d) out of order for sort_by=%s sort_order=%s: %s", i, i+1, sortBy, sortOrder, reason)
			}
		}
	})
}

// sortedPairOK checks that a's sort key is correctly ordered relative to
// b's, per sortBy/sortOrder — including Postgres's default NULL placement
// (NULLS LAST for ASC, NULLS FIRST for DESC) for the nullable columns
// (last_replenish_date, refund_total, replenish_total). String comparisons
// are delegated to Postgres itself (SELECT $1 <= $2) rather than
// reimplemented in Go: the database's default collation is not byte-order
// — e.g. it places "Circle" before "CIRCLE" — so a hand-rolled Go string
// comparison would disagree with what the query's ORDER BY actually did.
func sortedPairOK(ctx context.Context, pool *pgxpool.Pool, sortBy, sortOrder string, a, b AtmWithCashPos) (bool, string, error) {
	asc := sortOrder == "asc"
	switch sortBy {
	case "terminal_id":
		ok, err := stringOrderOK(ctx, pool, a.TerminalID, b.TerminalID, asc)
		return ok, fmt.Sprintf("%q vs %q", a.TerminalID, b.TerminalID), err
	case "location":
		ok, err := stringOrderOK(ctx, pool, a.LocationName, b.LocationName, asc)
		return ok, fmt.Sprintf("%q vs %q", a.LocationName, b.LocationName), err
	case "status":
		ok, err := stringOrderOK(ctx, pool, a.Status, b.Status, asc)
		return ok, fmt.Sprintf("%q vs %q", a.Status, b.Status), err
	case "refund_total":
		return nullableFloatOrderOK(a.RefundTotal, b.RefundTotal, asc), fmt.Sprintf("%v vs %v", ptrOrNil(a.RefundTotal), ptrOrNil(b.RefundTotal)), nil
	case "replenish_total":
		return nullableFloatOrderOK(a.ReplenishTotal, b.ReplenishTotal, asc), fmt.Sprintf("%v vs %v", ptrOrNil(a.ReplenishTotal), ptrOrNil(b.ReplenishTotal)), nil
	case "last_replenish_date":
		return nullableTimeOrderOK(a.LastReplenishDate, b.LastReplenishDate, asc), fmt.Sprintf("%v vs %v", a.LastReplenishDate, b.LastReplenishDate), nil
	}
	return true, "", nil
}

func stringOrderOK(ctx context.Context, pool *pgxpool.Pool, a, b string, asc bool) (bool, error) {
	op := "<="
	if !asc {
		op = ">="
	}
	var ok bool
	err := pool.QueryRow(ctx, fmt.Sprintf("SELECT $1::text %s $2::text", op), a, b).Scan(&ok)
	return ok, err
}

func nullableFloatOrderOK(a, b *float64, asc bool) bool {
	if asc {
		// NULLS LAST: once a is nil, b must also be nil.
		if a == nil {
			return b == nil
		}
		if b == nil {
			return true
		}
		return *a <= *b
	}
	// DESC, NULLS FIRST: once a is non-nil, b must also be non-nil.
	if a == nil {
		return true
	}
	if b == nil {
		return false
	}
	return *a >= *b
}

func nullableTimeOrderOK(a, b *time.Time, asc bool) bool {
	if asc {
		if a == nil {
			return b == nil
		}
		if b == nil {
			return true
		}
		return !a.After(*b)
	}
	if a == nil {
		return true
	}
	if b == nil {
		return false
	}
	return !a.Before(*b)
}
