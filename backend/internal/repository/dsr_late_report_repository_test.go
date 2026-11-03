package repository

import (
	"context"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CLAUDE.md Sec 6: the DSR late report is a report -- every read goes to the replica.
func TestDsrLateReportRepository_ReadsReplica(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(r *DsrLateReportRepository) error{
		"ListDaily":             func(r *DsrLateReportRepository) error { _, err := r.ListDaily(ctx, db.ListDsrLateDailyParams{}); return err },
		"CountDaily":            func(r *DsrLateReportRepository) error { _, err := r.CountDaily(ctx, db.CountDsrLateDailyParams{}); return err },
		"ListSummary":           func(r *DsrLateReportRepository) error { _, err := r.ListSummary(ctx, db.ListDsrLateSummaryParams{}); return err },
		"ListUnmappedVaults":    func(r *DsrLateReportRepository) error { _, err := r.ListUnmappedVaults(ctx, db.ListDsrLateUnmappedVaultsParams{}); return err },
		"ListUnmappedLabels":    func(r *DsrLateReportRepository) error { _, err := r.ListUnmappedLabels(ctx, db.ListDsrLateUnmappedLabelsParams{}); return err },
		"ListUnresolvedVendors": func(r *DsrLateReportRepository) error { _, err := r.ListUnresolvedVendors(ctx, db.ListDsrLateUnresolvedVendorsParams{}); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			var hits []string
			repo := NewDsrLateReportRepository(poolProbe{name: "replica", hits: &hits})
			if err := call(repo); err == nil {
				t.Fatal("want the probe error, got nil")
			}
			if len(hits) != 1 || hits[0] != "replica" {
				t.Fatalf("hits = %v, want [replica]", hits)
			}
		})
	}
}
