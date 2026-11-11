//go:build integration

package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// exportHeaderFile turns a 9-column vendor export into an Export_Header file
// (vendor-export-pks task 2.3): header = Import_Header + read-only columns, and
// every row gets the nine read-only cells produced by fill(vendorID).
func exportHeaderFile(t *testing.T, exported []byte, fill func(vendorID string) []string) []byte {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(exported)).ReadAll()
	if err != nil || len(rows) == 0 {
		t.Fatalf("parse export: %v (rows=%d)", err, len(rows))
	}
	out := [][]string{vendorExportCols}
	for _, r := range rows[1:] {
		out = append(out, slices.Concat(r, fill(r[0])))
	}
	var buf bytes.Buffer
	if err := csv.NewWriter(&buf).WriteAll(out); err != nil {
		t.Fatalf("write export-header csv: %v", err)
	}
	return buf.Bytes()
}

// Spec vendor-export-pks Req 2.3/2.4/2.6/6.2: a vendor file with the Export_Header
// round-trips as 0 changes whether the read-only cells hold the real PKS /
// director data from the DB or arbitrary text. Read-only: nothing is inserted.
func TestExportRoundTrip_VendorExportHeader_ZeroChanges(t *testing.T) {
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

	// Real read-only data: director (is_director PIC) + active PKS, per vendor.
	real := map[string][]string{}
	rs, err := pool.Query(ctx, `
		SELECT v.id::text,
		       COALESCE(d.name,''), COALESCE(d.phone,''), COALESCE(d.email,''),
		       COALESCE(k.pks_name,''), COALESCE(k.effective_date::text,''), COALESCE(k.expiry_date::text,'')
		FROM vendors v
		LEFT JOIN vendor_pics d ON d.vendor_id = v.id AND d.is_director AND d.deleted_at IS NULL
		LEFT JOIN vendor_pks k ON k.vendor_id = v.id AND k.replaced_at IS NULL`)
	if err != nil {
		t.Fatalf("load pks/director: %v", err)
	}
	for rs.Next() {
		var id string
		c := make([]string, 6)
		if err := rs.Scan(&id, &c[0], &c[1], &c[2], &c[3], &c[4], &c[5]); err != nil {
			t.Fatalf("scan: %v", err)
		}
		real[id] = append(c, "12", "300", "Aktif")
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		t.Fatalf("load pks/director: %v", err)
	}

	variants := map[string]func(string) []string{
		"db-data": func(id string) []string {
			if c, ok := real[id]; ok {
				return c
			}
			return make([]string, 9)
		},
		"random": func(id string) []string {
			return []string{"=Acak", "+62 1", "x@y.id", "PKS " + id, "2020-02-30", "bukan tanggal", "-1", "abc", "??"}
		},
	}
	for _, status := range []string{"all", "active"} {
		var exported bytes.Buffer
		rows, err := NewMasterDataExporter(repo).Write(adminCtx(1), ExportVendors, status, &exported, nil)
		if err != nil {
			t.Fatalf("export: %v", err)
		}
		for name, fill := range variants {
			t.Run(status+"-"+name, func(t *testing.T) {
				file := exportHeaderFile(t, exported.Bytes(), fill)
				res, err := NewMasterDataImporter(repo).DryRun(adminCtx(1), ExportVendors, bytes.NewReader(file))
				if err != nil {
					t.Fatalf("dry-run: %v", err)
				}
				p := res.Preview
				for i, e := range p.Errors {
					if i >= 5 {
						break
					}
					t.Errorf("row %d %s: %s", e.Row, e.Field, e.Message)
				}
				if p.TotalRows != rows || p.Creates != 0 || p.Updates != 0 || p.Unchanged != rows {
					t.Errorf("export-header round trip should be 0 changes over %d rows: %+v", rows, p)
				}
			})
		}
	}

	// Template stays the 9-column Import_Header (Req 2.6).
	if e, ok := csvexport.Lookup(csvexport.Vendors); !ok || len(e.Header) != 9 {
		t.Errorf("vendor template header = %v (ok=%v), want 9 columns", e.Header, ok)
	}
}

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
