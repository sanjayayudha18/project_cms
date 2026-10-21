//go:build integration

package service

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend/internal/audit"
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

const picHeader = "id,vendor_code,branch_code,name,position,phone,email,is_notification_recipient,is_active,roles\n"
const picLegacyHeader = "id,vendor_code,branch_code,name,position,phone,email,is_notification_recipient,is_active\n"

func picRow(id, vendor, name string, tail ...string) string {
	return strings.Join(append([]string{id, vendor, "", name, "", "", "", "false", "true"}, tail...), ",") + "\n"
}

// Req 6.1-6.5 / Property 7: roles travel through the import batch -- blank on
// create = all three, "none" = empty, codes sorted; blank or missing column on
// update leaves them; an exported "none" PIC round-trips with 0 changes.
func TestMasterDataImport_PicRoles_Integration(t *testing.T) {
	f := newImportFixture(t)
	repo := repository.NewMasterDataExportRepository(f.h.pool)
	f.importer = NewMasterDataImporter(repo).WithConfirm(ImportConfirmDeps{
		Batches:      repository.NewMasterDataImportBatchRepository(f.h.pool, f.h.pool),
		Orchestrator: f.orch,
		Audit:        audit.NewWriter(f.h.pool),
		Services: func(sub ImportSubmitter) ImportServices {
			return ImportServices{PICs: NewVendorPicAdminService(repository.NewVendorPicAdminRepository(f.h.pool, f.h.pool), sub)}
		},
	})
	code := "ITPI-" + f.tag
	vendorID := f.h.seedVendor(code, "Vendor PIC CSV "+f.tag)
	existing := createPicViaApproval(t, f.svc, f.h, vendorID, "Lama "+f.tag, &[]string{"vault"})

	confirmApprove := func(csv string) {
		t.Helper()
		res, err := f.importer.Confirm(f.ctx, f.h.makerID, ExportVendorPICs, strings.NewReader(csv), "127.0.0.1")
		if err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		f.batches = append(f.batches, res.BatchID)
		if err := f.approve(res); err != nil {
			t.Fatalf("approve: %v", err)
		}
	}
	roleOf := func(name string) []string {
		t.Helper()
		var id int64
		if err := f.h.pool.QueryRow(f.ctx, `SELECT id FROM vendor_pics WHERE vendor_id = $1 AND name = $2`, vendorID, name).Scan(&id); err != nil {
			t.Fatalf("load pic %s: %v", name, err)
		}
		f.h.vendorPicIDs = append(f.h.vendorPicIDs, id)
		return picRoles(t, f.svc, id)
	}

	confirmApprove(picHeader +
		picRow("", code, "Kosong "+f.tag, "") +
		picRow("", code, "Tanpa "+f.tag, "none") +
		picRow("", code, "Dua "+f.tag, "vault;replenish") +
		picRow(strconv.FormatInt(existing, 10), code, "Lama Baru "+f.tag, ""))

	if got := roleOf("Kosong " + f.tag); !slices.Equal(got, AllPicRoles()) {
		t.Errorf("create with blank roles = %q, want all three", got)
	}
	if got := roleOf("Tanpa " + f.tag); len(got) != 0 {
		t.Errorf("create with none = %q, want empty", got)
	}
	if got := roleOf("Dua " + f.tag); !slices.Equal(got, []string{"replenish", "vault"}) {
		t.Errorf("create with vault;replenish = %q, want sorted", got)
	}
	if got := picRoles(t, f.svc, existing); !slices.Equal(got, []string{"vault"}) {
		t.Errorf("update with blank roles = %q, want [vault] unchanged", got)
	}

	confirmApprove(picLegacyHeader + picRow(strconv.FormatInt(existing, 10), code, "Lama Legacy "+f.tag))
	if got := picRoles(t, f.svc, existing); !slices.Equal(got, []string{"vault"}) {
		t.Errorf("update from legacy header = %q, want [vault] unchanged", got)
	}

	confirmApprove(picHeader + picRow(strconv.FormatInt(existing, 10), code, "Lama Legacy "+f.tag, "none"))
	if got := picRoles(t, f.svc, existing); len(got) != 0 {
		t.Errorf("update with none = %q, want empty", got)
	}

	var buf bytes.Buffer
	if _, err := NewMasterDataExporter(repo).Write(f.ctx, ExportVendorPICs, "all", &buf, nil); err != nil {
		t.Fatalf("export: %v", err)
	}
	res, err := NewMasterDataImporter(repo).DryRun(f.ctx, ExportVendorPICs, &buf)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if p := res.Preview; len(p.Errors) != 0 || p.Creates != 0 || p.Updates != 0 || p.Unchanged != p.TotalRows {
		t.Errorf("export of PICs incl. none/sorted roles must round-trip with 0 changes: %+v", p)
	}
}
