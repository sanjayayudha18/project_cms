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

// Property 4 (vendor-data-export task 3.3): export -> unedited -> importer
// DryRun is "0 changes" for every global entity. STRICTLY read-only: export reads
// and DryRun never writes; Confirm is never called and no fixtures are inserted.
func TestExportRoundTrip_DryRunIsZeroChanges(t *testing.T) {
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

	for _, entity := range exportEntityNames() {
		for _, status := range []string{"all", "active"} {
			t.Run(entity+"-"+status, func(t *testing.T) {
				var buf bytes.Buffer
				rows, err := NewMasterDataExporter(repo).Write(adminCtx(1), entity, status, &buf, nil)
				if err != nil {
					t.Fatalf("export: %v", err)
				}
				res, err := NewMasterDataImporter(repo).DryRun(adminCtx(1), entity, &buf)
				if err != nil {
					t.Fatalf("dry-run: %v", err)
				}
				p := res.Preview
				t.Logf("rows=%d total=%d valid=%d creates=%d updates=%d unchanged=%d errors=%d",
					rows, p.TotalRows, p.ValidRows, p.Creates, p.Updates, p.Unchanged, len(p.Errors))
				for i, e := range p.Errors {
					if i >= 5 {
						break
					}
					t.Errorf("row %d %s: %s", e.Row, e.Field, e.Message)
				}
				if p.TotalRows != rows || p.Creates != 0 || p.Updates != 0 || p.Unchanged != rows {
					t.Errorf("round trip should be 0 changes over %d rows: %+v", rows, p)
				}
			})
		}
	}
}
