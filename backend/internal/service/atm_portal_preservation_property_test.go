//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: rename_db_table (itm_cashpos -> itm_replenish)
// Property 2: Preservation - Query Results and ETL Behavior Unchanged
//
// This file covers the two preservation behaviors task 2 calls out that
// aren't already exercised by atm_portal_property_test.go /
// atm_portal_filter_property_test.go / atm_portal_summary_property_test.go:
// GetLastUpdated (the data-freshness query) and the CASCADE delete from
// itm_replenish_files to itm_replenish. Written BEFORE the rename fix — ran
// against the pre-rename schema (recording the baseline), and now updated
// to run unchanged against the renamed schema (task 3.9), since the rename
// is metadata-only and touches no query logic or FK behavior.

// TestPreservation_GetLastUpdated_ReturnsGlobalMostRecentRecord validates
// that GetLastUpdated returns (replenish_date + replenish_time) for the
// single most-recent record across ALL terminals (it has no per-terminal
// filter, unlike ListATMsWithCashPos's LEFT JOIN LATERAL). Inserted records
// use a year far beyond any realistic seed data so the freshly-inserted row
// is guaranteed to be the global max within the rolled-back transaction,
// regardless of what other data already exists in the shared `cms` DB.
func TestPreservation_GetLastUpdated_ReturnsGlobalMostRecentRecord(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		numRecords := rapid.IntRange(1, 5).Draw(rt, "numRecords")
		// Year 9000+ guarantees this is the global latest record regardless
		// of pre-existing rows in the shared dataset.
		baseDate := time.Date(9000, 1, 1, 0, 0, 0, 0, time.UTC)

		tx, err := pool.Begin(ctx)
		if err != nil {
			rt.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		locationID, err := insertTestLocation(ctx, tx, regionID)
		if err != nil {
			rt.Fatalf("insert location: %v", err)
		}
		fileID, err := insertTestCashposFile(ctx, tx)
		if err != nil {
			rt.Fatalf("insert cashpos file: %v", err)
		}

		var latestWant time.Time
		for i := 0; i < numRecords; i++ {
			terminalID := "PU-" + uuid.NewString()[:12]
			if err := insertTestATM(ctx, tx, terminalID, locationID, nil, nil); err != nil {
				rt.Fatalf("insert atm: %v", err)
			}
			dayOffset := rapid.IntRange(0, 365).Draw(rt, "dayOffset")
			hh := rapid.IntRange(0, 23).Draw(rt, "hh")
			mm := rapid.IntRange(0, 59).Draw(rt, "mm")
			ss := rapid.IntRange(0, 59).Draw(rt, "ss")
			d := baseDate.AddDate(0, 0, dayOffset)
			timeStr := time.Date(0, 1, 1, hh, mm, ss, 0, time.UTC).Format("15:04:05")

			if err := insertTestCashpos(ctx, tx, fileID, terminalID, d, timeStr, 1000); err != nil {
				rt.Fatalf("insert cashpos: %v", err)
			}

			ts := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, ss, 0, time.UTC)
			if ts.After(latestWant) {
				latestWant = ts
			}
		}

		queries := db.New(tx)
		got, err := queries.GetLastUpdated(ctx)
		if err != nil {
			rt.Fatalf("GetLastUpdated: %v", err)
		}
		if !got.Valid {
			rt.Fatal("GetLastUpdated returned NULL despite inserted records")
		}
		if !got.Time.Equal(latestWant) {
			rt.Fatalf("GetLastUpdated = %s, want %s (numRecords=%d)", got.Time, latestWant, numRecords)
		}
	})
}

// TestPreservation_GetLastUpdated_NoRecordsReturnsNoRows validates the
// no-data branch design.md documents: GetLastUpdated is a `:one` query, so
// zero itm_replenish rows produce pgx.ErrNoRows (not a NULL scan), which
// atm_portal.go's ListATMs translates to last_updated: null instead of a
// 500. Deletes all itm_replenish rows inside a rolled-back transaction so
// nothing is actually lost from the shared `cms` dataset.
func TestPreservation_GetLastUpdated_NoRecordsReturnsNoRows(t *testing.T) {
	pool := propertyTestPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "DELETE FROM itm_replenish"); err != nil {
		t.Fatalf("deleting itm_replenish rows: %v", err)
	}

	queries := db.New(tx)
	_, err = queries.GetLastUpdated(ctx)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetLastUpdated error = %v, want pgx.ErrNoRows", err)
	}
}

// TestPreservation_CascadeDeletesReplenishRowsWithFile validates the FK
// fk_itm_replenish_file's ON DELETE CASCADE: deleting an itm_replenish_files
// row must delete every itm_replenish row referencing it via file_id, and
// must NOT touch rows belonging to other files.
func TestPreservation_CascadeDeletesReplenishRowsWithFile(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		numTargetRows := rapid.IntRange(1, 5).Draw(rt, "numTargetRows")
		numOtherRows := rapid.IntRange(0, 3).Draw(rt, "numOtherRows")

		tx, err := pool.Begin(ctx)
		if err != nil {
			rt.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		locationID, err := insertTestLocation(ctx, tx, regionID)
		if err != nil {
			rt.Fatalf("insert location: %v", err)
		}

		targetFileID, err := insertTestCashposFile(ctx, tx)
		if err != nil {
			rt.Fatalf("insert target cashpos file: %v", err)
		}
		otherFileID, err := insertTestCashposFile(ctx, tx)
		if err != nil {
			rt.Fatalf("insert other cashpos file: %v", err)
		}

		for i := 0; i < numTargetRows; i++ {
			terminalID := "PC-target-" + uuid.NewString()[:12]
			if err := insertTestATM(ctx, tx, terminalID, locationID, nil, nil); err != nil {
				rt.Fatalf("insert atm: %v", err)
			}
			if err := insertTestCashpos(ctx, tx, targetFileID, terminalID, time.Now(), "10:00:00", 1000); err != nil {
				rt.Fatalf("insert cashpos for target file: %v", err)
			}
		}
		for i := 0; i < numOtherRows; i++ {
			terminalID := "PC-other-" + uuid.NewString()[:12]
			if err := insertTestATM(ctx, tx, terminalID, locationID, nil, nil); err != nil {
				rt.Fatalf("insert atm: %v", err)
			}
			if err := insertTestCashpos(ctx, tx, otherFileID, terminalID, time.Now(), "10:00:00", 1000); err != nil {
				rt.Fatalf("insert cashpos for other file: %v", err)
			}
		}

		if _, err := tx.Exec(ctx, "DELETE FROM itm_replenish_files WHERE id = $1", targetFileID); err != nil {
			rt.Fatalf("deleting target file record: %v", err)
		}

		var targetRemaining int
		if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM itm_replenish WHERE file_id = $1", targetFileID).Scan(&targetRemaining); err != nil {
			rt.Fatalf("counting target rows: %v", err)
		}
		if targetRemaining != 0 {
			rt.Fatalf("expected CASCADE to delete all %d row(s) for file %d, %d remain", numTargetRows, targetFileID, targetRemaining)
		}

		var otherRemaining int
		if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM itm_replenish WHERE file_id = $1", otherFileID).Scan(&otherRemaining); err != nil {
			rt.Fatalf("counting other rows: %v", err)
		}
		if otherRemaining != numOtherRows {
			rt.Fatalf("CASCADE affected unrelated file %d: got %d rows remaining, want %d", otherFileID, otherRemaining, numOtherRows)
		}
	})
}
