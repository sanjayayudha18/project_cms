package repository

import (
	"context"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// DsrLateReportRepository backs the DSR late report (dsr-late-report FR2-FR4).
// Read-only, replica only (CLAUDE.md Sec 6): status is computed on read from
// dsr_receipts + master data, nothing is written.
type DsrLateReportRepository struct {
	readQueries *db.Queries
}

// NewDsrLateReportRepository wraps the replica pool (readConn).
func NewDsrLateReportRepository(readConn db.DBTX) *DsrLateReportRepository {
	return &DsrLateReportRepository{readQueries: db.New(readConn)}
}

// ListDaily returns one row per (report date, obliged branch); PageLimit nil = all rows.
func (r *DsrLateReportRepository) ListDaily(ctx context.Context, arg db.ListDsrLateDailyParams) ([]db.ListDsrLateDailyRow, error) {
	return r.readQueries.ListDsrLateDaily(ctx, arg)
}

// CountDaily returns the ListDaily total for the same filters.
func (r *DsrLateReportRepository) CountDaily(ctx context.Context, arg db.CountDsrLateDailyParams) (int64, error) {
	return r.readQueries.CountDsrLateDaily(ctx, arg)
}

// ListSummary returns the per-branch monthly recap.
func (r *DsrLateReportRepository) ListSummary(ctx context.Context, arg db.ListDsrLateSummaryParams) ([]db.ListDsrLateSummaryRow, error) {
	return r.readQueries.ListDsrLateSummary(ctx, arg)
}

// ListUnmappedVaults returns active vaults without a DSR location map (warning FR3.4a).
func (r *DsrLateReportRepository) ListUnmappedVaults(ctx context.Context, arg db.ListDsrLateUnmappedVaultsParams) ([]db.ListDsrLateUnmappedVaultsRow, error) {
	return r.readQueries.ListDsrLateUnmappedVaults(ctx, arg)
}

// ListUnmappedLabels returns receipt labels that map to no vault (warning FR3.4b).
func (r *DsrLateReportRepository) ListUnmappedLabels(ctx context.Context, arg db.ListDsrLateUnmappedLabelsParams) ([]db.ListDsrLateUnmappedLabelsRow, error) {
	return r.readQueries.ListDsrLateUnmappedLabels(ctx, arg)
}

// ListUnresolvedVendors returns receipt vendor labels with no vendors row (warning FR3.4c).
func (r *DsrLateReportRepository) ListUnresolvedVendors(ctx context.Context, arg db.ListDsrLateUnresolvedVendorsParams) ([]db.ListDsrLateUnresolvedVendorsRow, error) {
	return r.readQueries.ListDsrLateUnresolvedVendors(ctx, arg)
}
