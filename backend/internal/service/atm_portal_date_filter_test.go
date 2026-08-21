//go:build integration

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: atm-portal-fix1
// Property: Date range filter correctness on last_replenish_date (inclusive).

func TestProperty_DateRangeFilterCorrectness(t *testing.T) {
	pool := propertyTestPool(t)
	regionID := seedRegionID(t, pool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		marker := "DF" + uuid.NewString()[:8]
		base := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

		// Three terminals with known last_replenish_date offsets: 0, 10, 20 days.
		// Fourth terminal has no cashpos (NULL last_replenish_date).
		offsets := []int{0, 10, 20}
		dates := make([]time.Time, len(offsets))
		for i, d := range offsets {
			dates[i] = base.AddDate(0, 0, d)
		}

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

		for i, d := range dates {
			terminalID := fmt.Sprintf("%s-%d", marker, i)
			if err := insertTestATM(ctx, tx, terminalID, locationID, nil, nil); err != nil {
				rt.Fatalf("insert atm: %v", err)
			}
			if err := insertTestCashpos(ctx, tx, fileID, terminalID, d, "10:00:00", 1000); err != nil {
				rt.Fatalf("insert cashpos: %v", err)
			}
		}
		nullTerminal := fmt.Sprintf("%s-null", marker)
		if err := insertTestATM(ctx, tx, nullTerminal, locationID, nil, nil); err != nil {
			rt.Fatalf("insert null atm: %v", err)
		}

		// Random inclusive bounds drawn from known dates + empty (unbound).
		fromChoices := []string{"", dates[0].Format("2006-01-02"), dates[1].Format("2006-01-02"), dates[2].Format("2006-01-02")}
		toChoices := []string{"", dates[0].Format("2006-01-02"), dates[1].Format("2006-01-02"), dates[2].Format("2006-01-02")}
		dateFrom := rapid.SampledFrom(fromChoices).Draw(rt, "dateFrom")
		dateTo := rapid.SampledFrom(toChoices).Draw(rt, "dateTo")
		if dateFrom != "" && dateTo != "" && dateFrom > dateTo {
			dateFrom, dateTo = dateTo, dateFrom
		}

		svc := NewAtmPortalService(db.New(tx))
		result, err := svc.ListATMs(ctx, ListATMsParams{
			Page: 1, PageSize: 50, Status: "all", Search: marker,
			DateFrom: dateFrom, DateTo: dateTo,
			SortBy: "terminal_id", SortOrder: "asc",
		})
		if err != nil {
			rt.Fatalf("query: %v", err)
		}

		// Build expected terminal set.
		expected := map[string]bool{}
		for i, d := range dates {
			ds := d.Format("2006-01-02")
			if dateFrom != "" && ds < dateFrom {
				continue
			}
			if dateTo != "" && ds > dateTo {
				continue
			}
			expected[fmt.Sprintf("%s-%d", marker, i)] = true
		}
		// NULL last_replenish_date excluded when any bound is active.
		if dateFrom == "" && dateTo == "" {
			expected[nullTerminal] = true
		}

		got := map[string]bool{}
		for _, atm := range result.Data {
			got[atm.TerminalID] = true
			if dateFrom != "" || dateTo != "" {
				if atm.LastReplenishDate == nil {
					rt.Fatalf("NULL last_replenish_date returned with active date bound (from=%q to=%q)", dateFrom, dateTo)
				}
				ds := atm.LastReplenishDate.Format("2006-01-02")
				if dateFrom != "" && ds < dateFrom {
					rt.Fatalf("row %s date %s < date_from %s", atm.TerminalID, ds, dateFrom)
				}
				if dateTo != "" && ds > dateTo {
					rt.Fatalf("row %s date %s > date_to %s", atm.TerminalID, ds, dateTo)
				}
			}
		}
		for id := range expected {
			if !got[id] {
				rt.Fatalf("expected terminal %s missing (from=%q to=%q got=%v)", id, dateFrom, dateTo, keys(got))
			}
		}
		for id := range got {
			if !expected[id] {
				rt.Fatalf("unexpected terminal %s (from=%q to=%q expected=%v)", id, dateFrom, dateTo, keys(expected))
			}
		}
		if int(result.Total) != len(expected) {
			rt.Fatalf("total=%d want %d (list/count parity)", result.Total, len(expected))
		}
	})
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestListATMs_DateValidation(t *testing.T) {
	// Unit-style: no DB needed — validate() fails before repo calls.
	svc := NewAtmPortalService(nil)
	ctx := context.Background()

	cases := []struct {
		name    string
		params  ListATMsParams
		wantFld string
	}{
		{
			name: "invalid date_from",
			params: ListATMsParams{
				Page: 1, PageSize: 25, Status: "all", SortBy: "terminal_id", SortOrder: "asc",
				DateFrom: "21-08-2026",
			},
			wantFld: "date_from",
		},
		{
			name: "invalid date_to",
			params: ListATMsParams{
				Page: 1, PageSize: 25, Status: "all", SortBy: "terminal_id", SortOrder: "asc",
				DateTo: "not-a-date",
			},
			wantFld: "date_to",
		},
		{
			name: "from after to",
			params: ListATMsParams{
				Page: 1, PageSize: 25, Status: "all", SortBy: "terminal_id", SortOrder: "asc",
				DateFrom: "2026-08-21", DateTo: "2026-01-01",
			},
			wantFld: "date_from",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ListATMs(ctx, tc.params)
			if err == nil {
				t.Fatal("expected validation error")
			}
			ve, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("got %T %v, want *ValidationError", err, err)
			}
			if ve.Field != tc.wantFld {
				t.Fatalf("field=%q want %q (msg=%q)", ve.Field, tc.wantFld, ve.Message)
			}
		})
	}
}
