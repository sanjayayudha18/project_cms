//go:build integration

package service

import (
	"context"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: atm-portal
// Property 10: Summary independence from filters
// Validates: Requirements 3.4, 7.3
//
// Read-only against the real `cms` dataset — no seeded data needed, since
// the property only compares two Summary results against each other, not
// against a known expected count.

// drawFilterParams draws a valid ListATMsParams with page/page_size fixed
// (page and page_size aren't filter params — Property 10 is specifically
// about summary independence from *filters*) and every filter field
// randomized across a mix of real, partially-matching, and no-match values.
func drawFilterParams(rt *rapid.T, label string) ListATMsParams {
	dateFrom := rapid.SampledFrom([]string{"", "2020-01-01", "2024-06-01", "2026-01-01"}).Draw(rt, label+".dateFrom")
	dateToChoices := []string{"", "2024-12-31", "2026-08-21", "2030-12-31"}
	if dateFrom != "" {
		// Keep date_from <= date_to when both set so validate() does not reject.
		dateToChoices = append(dateToChoices, dateFrom)
	}
	dateTo := rapid.SampledFrom(dateToChoices).Draw(rt, label+".dateTo")
	if dateFrom != "" && dateTo != "" && dateFrom > dateTo {
		dateFrom, dateTo = dateTo, dateFrom
	}
	return ListATMsParams{
		Page:           1,
		PageSize:       25,
		Search:         rapid.SampledFrom([]string{"", "ATM", "1", "zzz-no-match-junk"}).Draw(rt, label+".search"),
		Status:         rapid.SampledFrom([]string{"all", "low", "critical", "normal", "unconfigured", "no_data"}).Draw(rt, label+".status"),
		MachineType:    rapid.SampledFrom([]string{"", "ATM", "CRM", "ATM,CRM"}).Draw(rt, label+".machineType"),
		Brand:          rapid.SampledFrom([]string{"", "Hyosung", "Wincor", "Diebold"}).Draw(rt, label+".brand"),
		DeploymentType: rapid.SampledFrom([]string{"", "ONSITE", "OFFSITE"}).Draw(rt, label+".deploymentType"),
		Region:         rapid.SampledFrom([]string{"", "Jakarta", "Bali", "zzz-no-match"}).Draw(rt, label+".region"),
		DateFrom:       dateFrom,
		DateTo:         dateTo,
		SortBy:         rapid.SampledFrom([]string{"terminal_id", "location", "last_replenish_date", "refund_total", "replenish_total", "status"}).Draw(rt, label+".sortBy"),
		SortOrder:      rapid.SampledFrom([]string{"asc", "desc"}).Draw(rt, label+".sortOrder"),
	}
}

// TestProperty10_SummaryIndependenceFromFilters validates Property 10: for
// any two requests that differ only in filter parameters, the summary
// object in both responses is identical — summary counts are computed
// across all active ATMs regardless of active filters.
func TestProperty10_SummaryIndependenceFromFilters(t *testing.T) {
	pool := propertyTestPool(t)
	ctx := context.Background()
	svc := NewAtmPortalService(db.New(pool))

	// Baseline: summary with no filters applied at all.
	baseline, err := svc.ListATMs(ctx, ListATMsParams{
		Page: 1, PageSize: 25, Status: "all", SortBy: "terminal_id", SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("baseline query: %v", err)
	}

	rapid.Check(t, func(rt *rapid.T) {
		params1 := drawFilterParams(rt, "params1")
		params2 := drawFilterParams(rt, "params2")

		result1, err := svc.ListATMs(ctx, params1)
		if err != nil {
			rt.Fatalf("query 1 (%+v): %v", params1, err)
		}
		result2, err := svc.ListATMs(ctx, params2)
		if err != nil {
			rt.Fatalf("query 2 (%+v): %v", params2, err)
		}

		if result1.Summary != result2.Summary {
			rt.Fatalf("summary differs between filter sets: %+v (params=%+v) vs %+v (params=%+v)",
				result1.Summary, params1, result2.Summary, params2)
		}
		if result1.Summary != baseline.Summary {
			rt.Fatalf("summary differs from the unfiltered baseline: %+v vs baseline %+v (params=%+v)",
				result1.Summary, baseline.Summary, params1)
		}
	})
}
