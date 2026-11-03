//go:build integration

package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Property 4 (vendor-data-export task 11.4): rows from the SHARED SQL with a
// vendor / cabang scope -> csvexport cells -> csvexport.Writer -> importer
// DryRun is "0 changes" (all unchanged). Fixtures are inserted into the dev DB
// and removed by harness's t.Cleanup (also on failure). Closes the 3.3 gap: dev
// has no PIC rows and no disabled rows.

type scopedFixture struct {
	vendorID, branch1ID, branch2ID int64
	vendorCode                     string
	// expected row counts (status=all)
	vaults, pics, branchVaults, branchPics int
}

func seedScopedFixture(t *testing.T, h *testHelpers, tag string) scopedFixture {
	t.Helper()
	f := scopedFixture{vendorCode: "RT" + tag}
	scan := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := h.pool.QueryRow(h.ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
		return id
	}

	f.vendorID = scan(`INSERT INTO vendors (code, name, legal_name, npwp, contact_email, contact_phone, hq_address)
		VALUES ($1, $2, $3, '012345678901000', 'rt@example.co.id', '+628123456789', $4) RETURNING id`,
		f.vendorCode, "=HYPERLINK(\"x\",\"y\") Vendor, Tbk "+tag, "-PT Legal, Sejahtera "+tag, "+Jl. Merdeka No. 1,\nJakarta\n@pusat")
	h.trackVendor(f.vendorID)

	branch := func(code, name, region string, regionCode *string, category string) int64 {
		id := scan(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, region, region_code, category)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, f.vendorID, code, name, region, regionCode, category)
		h.trackVendorBranch(id)
		return id
	}
	rc := "RT" + tag[len(tag)-6:]
	f.branch1ID = branch("RTB1-"+tag, "-Cabang, Satu\n"+tag, "=Jakarta Timur", &rc, "ATM_CASH")
	f.branch2ID = branch("RTB2-"+tag, "Cabang Dua "+tag, "", nil, "CASH")

	vault := func(branchID int64, code, category string, min, max, lat, lon, hours *string, active bool) {
		var id int64
		err := h.pool.QueryRow(h.ctx, `INSERT INTO vendor_vaults
			(vendor_branch_id, vault_code, type, category, currency_code, min_capacity_amount, max_capacity_amount,
			 latitude, longitude, operating_hours, is_active, deleted_at)
			VALUES ($1, $2, $3, $3, 'IDR', $4::numeric, $5::numeric, $6::numeric, $7::numeric, $8, $9,
			        CASE WHEN $9 THEN NULL ELSE now() END) RETURNING id`,
			branchID, code, category, min, max, lat, lon, hours, active).Scan(&id)
		if err != nil {
			t.Fatalf("fixture vault %s: %v", code, err)
		}
		h.vendorVaultIDs = append(h.vendorVaultIDs, id)
	}
	s := func(v string) *string { return &v }
	vault(f.branch1ID, "RTV1-"+tag, "ATM_CASH", s("1000000.00"), s("50000000.50"), s("-6.200000"), s("106.816666"), s("+07:00-17:00, Senin\nSabtu"), true)
	vault(f.branch1ID, "RTV2-"+tag, "CASH", nil, nil, nil, nil, nil, true)
	vault(f.branch1ID, "RTV3-"+tag, "ATM", s("0"), s("123456789012.00"), nil, nil, s("=24 jam"), false) // disabled
	vault(f.branch2ID, "RTV4-"+tag, "CASH", s("500.00"), nil, nil, nil, nil, true)
	f.vaults, f.branchVaults = 4, 3

	pic := func(branchID *int64, name, position string, roles []string, notify, active bool) {
		var id int64
		err := h.pool.QueryRow(h.ctx, `INSERT INTO vendor_pics
			(vendor_id, vendor_branch_id, name, position, phone, email, is_notification_recipient, roles, is_active, deleted_at)
			VALUES ($1, $2, $3, $4, '+628111222333', 'pic@example.co.id', $5, $6, $7,
			        CASE WHEN $7 THEN NULL ELSE now() END) RETURNING id`,
			f.vendorID, branchID, name, position, notify, roles, active).Scan(&id)
		if err != nil {
			t.Fatalf("fixture pic %s: %v", name, err)
		}
		h.vendorPicIDs = append(h.vendorPicIDs, id)
	}
	pic(nil, "PIC Pusat "+tag, "=Manager, Pusat", []string{"replenish", "vault", "cashcount"}, true, true)
	pic(&f.branch1ID, "-PIC Kosong, "+tag, "+Staff\nShift", []string{}, false, true)
	pic(&f.branch1ID, "PIC Vault "+tag, "", []string{"vault"}, true, true)
	pic(&f.branch1ID, "PIC Nonaktif "+tag, "Lama", []string{"replenish"}, false, false) // disabled
	pic(&f.branch2ID, "PIC Cabang Dua "+tag, "@Koordinator", []string{"replenish", "cashcount"}, true, true)
	f.pics, f.branchPics = 5, 3

	return f
}

// scopedRows pulls every page of entity for the given scope straight from the
// generated shared queries (status=all) and shapes them with csvexport cells.
func scopedRows(t *testing.T, ctx context.Context, q *db.Queries, entity string, vendorID, branchID *int64) [][]string {
	t.Helper()
	var out [][]string
	var after int64
	for {
		var page [][]string
		var last int64
		switch entity {
		case csvexport.Vendors:
			rows, err := q.ExportVendorsBatch(ctx, db.ExportVendorsBatchParams{AfterID: after, VendorID: vendorID, Status: "all", BatchSize: 1000})
			if err != nil {
				t.Fatalf("%s: %v", entity, err)
			}
			for _, r := range rows {
				page, last = append(page, csvexport.VendorCells(csvexport.VendorRow(r))), r.ID
			}
		case csvexport.VendorBranches:
			rows, err := q.ExportVendorBranchesBatch(ctx, db.ExportVendorBranchesBatchParams{AfterID: after, VendorID: vendorID, Status: "all", BatchSize: 1000})
			if err != nil {
				t.Fatalf("%s: %v", entity, err)
			}
			for _, r := range rows {
				page, last = append(page, csvexport.VendorBranchCells(csvexport.VendorBranchRow(r))), r.ID
			}
		case csvexport.VendorVaults:
			rows, err := q.ExportVendorVaultsBatch(ctx, db.ExportVendorVaultsBatchParams{AfterID: after, VendorID: vendorID, BranchID: branchID, Status: "all", BatchSize: 1000})
			if err != nil {
				t.Fatalf("%s: %v", entity, err)
			}
			for _, r := range rows {
				page, last = append(page, csvexport.VendorVaultCells(csvexport.VendorVaultRow(r))), r.ID
			}
		case csvexport.VendorPICs:
			rows, err := q.ExportVendorPicsBatch(ctx, db.ExportVendorPicsBatchParams{AfterID: after, VendorID: vendorID, BranchID: branchID, Status: "all", BatchSize: 1000})
			if err != nil {
				t.Fatalf("%s: %v", entity, err)
			}
			for _, r := range rows {
				page, last = append(page, csvexport.VendorPICCells(csvexport.VendorPICRow(r))), r.ID
			}
		}
		out = append(out, page...)
		if len(page) < 1000 {
			return out
		}
		after = last
	}
}

func TestExportScopedRoundTrip_DryRunIsZeroChanges(t *testing.T) {
	_, tag, h := harness(t)
	f := seedScopedFixture(t, h, tag)
	q := db.New(h.pool)
	repo := repository.NewMasterDataExportRepository(h.pool)
	importer := NewMasterDataImporter(repo)

	vid, b1 := f.vendorID, f.branch1ID
	cases := []struct {
		name     string
		entity   string
		vendorID *int64
		branchID *int64
		want     int
	}{
		{"vendor/vendor", csvexport.Vendors, &vid, nil, 1},
		{"branches/vendor", csvexport.VendorBranches, &vid, nil, 2},
		{"vaults/vendor", csvexport.VendorVaults, &vid, nil, f.vaults},
		{"pics/vendor", csvexport.VendorPICs, &vid, nil, f.pics},
		{"vaults/cabang", csvexport.VendorVaults, &vid, &b1, f.branchVaults},
		{"pics/cabang", csvexport.VendorPICs, &vid, &b1, f.branchPics},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := scopedRows(t, h.ctx, q, c.entity, c.vendorID, c.branchID)
			if len(rows) != c.want {
				t.Fatalf("scoped rows = %d, want %d", len(rows), c.want)
			}
			ent, ok := csvexport.Lookup(c.entity)
			if !ok {
				t.Fatalf("unknown entity %s", c.entity)
			}
			var buf bytes.Buffer
			w, err := csvexport.NewWriter(&buf, ent.Header)
			if err != nil {
				t.Fatalf("writer: %v", err)
			}
			for _, r := range rows {
				if err := w.Write(r); err != nil {
					t.Fatalf("write: %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
			res, err := importer.DryRun(adminCtx(1), c.entity, &buf)
			if err != nil {
				t.Fatalf("dry-run: %v", err)
			}
			p := res.Preview
			for i, e := range p.Errors {
				if i >= 10 {
					break
				}
				t.Errorf("row %d %s: %s", e.Row, e.Field, e.Message)
			}
			if p.TotalRows != len(rows) || p.Creates != 0 || p.Updates != 0 || p.Unchanged != len(rows) || len(p.Errors) != 0 {
				t.Errorf("want %d unchanged and 0 else, got %+v", len(rows), p)
			}
		})
	}
}
