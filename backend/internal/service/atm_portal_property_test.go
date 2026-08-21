//go:build integration

package service

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: atm-portal
// Property 1: Replenishment status classification
// Property 2: Latest record selection
//
// Both properties concern SQL query correctness (the CASE expression and
// the LEFT JOIN LATERAL in atm_portal.sql), so — per design.md's Testing
// Strategy — these run against a real Postgres (DATABASE_URL, expected to
// point at the shared `cms` database, same as internal/handler's
// integration tests) rather than as pure-function tests. Each check
// iteration runs inside its own transaction that is always rolled back, so
// nothing persists in `cms`.

func propertyTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping property test")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedRegionID returns an existing region id to reference from freshly
// inserted locations rows. Regions are stable seed data — read-only here,
// not part of any rolled-back transaction.
func seedRegionID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), "SELECT id FROM regions ORDER BY id LIMIT 1").Scan(&id)
	if err != nil {
		t.Fatalf("reading a seed region id: %v", err)
	}
	return id
}

// insertTestLocation inserts a fresh locations row and returns its id.
func insertTestLocation(ctx context.Context, tx pgx.Tx, regionID int64) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO locations (region_id, type, name, address_line1, city_or_regency, province, country_code)
		VALUES ($1, 'branch', 'Property Test Location', 'Jl. Test No. 1', 'Jakarta', 'DKI Jakarta', 'ID')
		RETURNING id
	`, regionID).Scan(&id)
	return id, err
}

// insertTestATM inserts a fresh atms row with the given nullable thresholds.
func insertTestATM(ctx context.Context, tx pgx.Tx, terminalID string, locationID int64, lowThreshold, criticalThreshold *float64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO atms (terminal_id, location_id, machine_type, brand, model, operation_hours, deployment_type, low_threshold_amount, critical_threshold_amount, is_active)
		VALUES ($1, $2, 'ATM', 'Hyosung', 'TestModel', '24H', 'ONSITE', $3, $4, true)
	`, terminalID, locationID, lowThreshold, criticalThreshold)
	return err
}

// insertTestCashposFile inserts a fresh itm_cashpos_files row and returns its id.
func insertTestCashposFile(ctx context.Context, tx pgx.Tx) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO itm_cashpos_files (filename, file_date, status)
		VALUES ($1, CURRENT_DATE, 'completed')
		RETURNING id
	`, "proptest-"+uuid.NewString()+".txt").Scan(&id)
	return id, err
}

// insertTestCashpos inserts a fresh itm_cashpos row.
func insertTestCashpos(ctx context.Context, tx pgx.Tx, fileID int64, terminalID string, replenishDate time.Time, replenishTime string, refundTotal float64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO itm_cashpos (file_id, replenish_date, replenish_time, terminal_id, machine_type, teller_id, branch_code, refund_total, replenish_total)
		VALUES ($1, $2::date, $3::time, $4, 'ATM', 'T001', 'BR001', $5, $5)
	`, fileID, replenishDate, replenishTime, terminalID, refundTotal)
	return err
}

// round2 rounds to 2 decimal places, matching the numeric(20,2) columns.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// expectedStatus mirrors the CASE precedence in atm_portal.sql exactly:
// unconfigured > no_data > critical > low > normal.
func expectedStatus(lowThreshold, criticalThreshold *float64, hasCashpos bool, refundTotal float64) string {
	if lowThreshold == nil {
		return "unconfigured"
	}
	if !hasCashpos {
		return "no_data"
	}
	if criticalThreshold != nil && refundTotal <= *criticalThreshold {
		return "critical"
	}
	if refundTotal <= *lowThreshold {
		return "low"
	}
	return "normal"
}

// TestProperty1_ReplenishmentStatusClassification validates Property 1:
// for any ATM with a given low_threshold_amount, critical_threshold_amount,
// and latest refund_total, the computed status matches the documented
// precedence rules.
func TestProperty1_ReplenishmentStatusClassification(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		hasLowThreshold := rapid.Bool().Draw(rt, "hasLowThreshold")
		hasCriticalThreshold := rapid.Bool().Draw(rt, "hasCriticalThreshold")
		hasCashpos := rapid.Bool().Draw(rt, "hasCashpos")

		var lowThreshold, criticalThreshold *float64
		if hasLowThreshold {
			v := round2(rapid.Float64Range(1, 1_000_000_000).Draw(rt, "lowThreshold"))
			lowThreshold = &v
		}
		if hasCriticalThreshold {
			v := round2(rapid.Float64Range(1, 1_000_000_000).Draw(rt, "criticalThreshold"))
			criticalThreshold = &v
		}

		refundTotal := round2(rapid.Float64Range(0, 1_000_000_000).Draw(rt, "refundTotal"))
		if hasCashpos {
			// Bias some cases to land exactly on a threshold boundary, since
			// the CASE expression uses <=, not <.
			switch rapid.IntRange(0, 3).Draw(rt, "snapChoice") {
			case 1:
				if lowThreshold != nil {
					refundTotal = *lowThreshold
				}
			case 2:
				if criticalThreshold != nil {
					refundTotal = *criticalThreshold
				}
			}
		}

		terminalID := "P1-" + uuid.NewString()[:12]

		tx, err := pool.Begin(ctx)
		if err != nil {
			rt.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		locationID, err := insertTestLocation(ctx, tx, regionID)
		if err != nil {
			rt.Fatalf("insert location: %v", err)
		}
		if err := insertTestATM(ctx, tx, terminalID, locationID, lowThreshold, criticalThreshold); err != nil {
			rt.Fatalf("insert atm: %v", err)
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

		queries := db.New(tx)
		rows, err := queries.ListATMsWithCashPos(ctx, db.ListATMsWithCashPosParams{
			Search:    terminalID,
			Status:    "all",
			SortBy:    "terminal_id",
			SortOrder: "asc",
			Page:      1,
			PageSize:  1,
		})
		if err != nil {
			rt.Fatalf("query: %v", err)
		}
		if len(rows) != 1 {
			rt.Fatalf("expected exactly 1 row for terminal %q, got %d", terminalID, len(rows))
		}

		want := expectedStatus(lowThreshold, criticalThreshold, hasCashpos, refundTotal)
		got := rows[0].Status
		if got != want {
			rt.Fatalf("status = %q, want %q (lowThreshold=%v criticalThreshold=%v hasCashpos=%v refundTotal=%v)",
				got, want, ptrOrNil(lowThreshold), ptrOrNil(criticalThreshold), hasCashpos, refundTotal)
		}
	})
}

// TestProperty2_LatestRecordSelection validates Property 2: for any
// terminal with multiple itm_cashpos records, the system uses exclusively
// the record with the maximum (replenish_date, replenish_time) tuple —
// refund_total, last_replenish_date, and last_replenish_time must all come
// from that single most recent record, not a mix of rows.
func TestProperty2_LatestRecordSelection(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		numRecords := rapid.IntRange(2, 5).Draw(rt, "numRecords")
		baseDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		type record struct {
			date        time.Time
			refundTotal float64
		}
		records := make([]record, numRecords)
		for i := 0; i < numRecords; i++ {
			records[i] = record{
				date:        baseDate.AddDate(0, 0, i), // strictly increasing by index
				refundTotal: round2(rapid.Float64Range(0, 1_000_000_000).Draw(rt, fmt.Sprintf("refundTotal%d", i))),
			}
		}
		latest := records[numRecords-1] // largest date offset = the true latest

		terminalID := "P2-" + uuid.NewString()[:12]

		tx, err := pool.Begin(ctx)
		if err != nil {
			rt.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		locationID, err := insertTestLocation(ctx, tx, regionID)
		if err != nil {
			rt.Fatalf("insert location: %v", err)
		}
		if err := insertTestATM(ctx, tx, terminalID, locationID, nil, nil); err != nil {
			rt.Fatalf("insert atm: %v", err)
		}
		fileID, err := insertTestCashposFile(ctx, tx)
		if err != nil {
			rt.Fatalf("insert cashpos file: %v", err)
		}

		// Insert in reverse order so the result can't be an artifact of
		// insertion/table order — only ORDER BY replenish_date DESC,
		// replenish_time DESC should determine the winner.
		for i := numRecords - 1; i >= 0; i-- {
			if err := insertTestCashpos(ctx, tx, fileID, terminalID, records[i].date, "10:00:00", records[i].refundTotal); err != nil {
				rt.Fatalf("insert cashpos %d: %v", i, err)
			}
		}

		queries := db.New(tx)
		rows, err := queries.ListATMsWithCashPos(ctx, db.ListATMsWithCashPosParams{
			Search:    terminalID,
			Status:    "all",
			SortBy:    "terminal_id",
			SortOrder: "asc",
			Page:      1,
			PageSize:  1,
		})
		if err != nil {
			rt.Fatalf("query: %v", err)
		}
		if len(rows) != 1 {
			rt.Fatalf("expected exactly 1 row for terminal %q, got %d", terminalID, len(rows))
		}
		got := rows[0]

		if !got.LastReplenishDate.Valid {
			rt.Fatalf("last_replenish_date is NULL, want %s", latest.date.Format("2006-01-02"))
		}
		if !got.LastReplenishDate.Time.Equal(latest.date) {
			rt.Fatalf("last_replenish_date = %s, want %s", got.LastReplenishDate.Time.Format("2006-01-02"), latest.date.Format("2006-01-02"))
		}
		if !got.RefundTotal.Valid {
			rt.Fatalf("refund_total is NULL, want %v", latest.refundTotal)
		}
		gotRefund, err := got.RefundTotal.Float64Value()
		if err != nil {
			rt.Fatalf("converting refund_total: %v", err)
		}
		if gotRefund.Float64 != latest.refundTotal {
			rt.Fatalf("refund_total = %v, want %v (numRecords=%d)", gotRefund.Float64, latest.refundTotal, numRecords)
		}
	})
}

func ptrOrNil(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
