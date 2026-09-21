//go:build integration

package service

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// TestMasterDataImport_RoundTripOnRealDatabase exports every real table with
// the real exporter and feeds the file straight back to the dry-run: importing
// what was just exported must be "0 changes" (plan Validate Fase 5). Read-only.
func TestMasterDataImport_RoundTripOnRealDatabase(t *testing.T) {
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
	repo := repository.NewMasterDataExportRepository(pool)

	for _, entity := range []string{ExportVendors, ExportVendorBranches, ExportVendorVaults, ExportVendorPICs, ExportATMs, ExportATMAssignments} {
		t.Run(entity, func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := NewMasterDataExporter(repo).Write(adminCtx(1), entity, "all", &buf, nil); err != nil {
				t.Fatalf("export: %v", err)
			}

			res, err := NewMasterDataImporter(repo).DryRun(adminCtx(1), entity, &buf)
			if err != nil {
				t.Fatalf("dry-run: %v", err)
			}

			p := res.Preview
			if p.Creates != 0 || p.Updates != 0 || p.Unchanged != p.TotalRows {
				t.Errorf("round trip should be 0 changes: %+v", p)
			}
			for i, e := range p.Errors {
				if i >= 5 {
					break
				}
				t.Errorf("row %d %s: %s", e.Row, e.Field, e.Message)
			}
		})
	}
}
