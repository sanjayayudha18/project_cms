package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Change_Preview (master-data-import-revamp 2.2): buildChangePreview describes
// what confirm will stage, so its ops, counts and field diffs are pinned here.

// cells builds a header-keyed row from alternating key, value arguments.
func cells(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

func TestBuildChangePreview_PerEntity(t *testing.T) {
	tests := []struct {
		name       string
		entity     string
		row        ImportRow
		cur        map[string]string
		wantNil    bool
		wantOp     string
		wantKey    string
		wantFields []ImportFieldChange
	}{
		{
			name: "vendor create lists only non-empty columns", entity: ExportVendors,
			row:    ImportRow{Row: 2, Op: ImportOpCreate, Cells: cells("name", "Baru", "legal_name", "PT Baru", "npwp", "")},
			wantOp: ImportOpCreate, wantKey: "Baru",
			wantFields: []ImportFieldChange{{Field: "name", After: "Baru"}, {Field: "legal_name", After: "PT Baru"}},
		},
		{
			name: "vendor update lists only changed columns", entity: ExportVendors,
			row:    ImportRow{Row: 3, Op: ImportOpUpdate, ID: 1, Cells: cells("name", "Vendor Satu", "legal_name", "PT B", "npwp", "1")},
			cur:    cells("name", "Vendor Satu", "legal_name", "PT A", "npwp", "1"),
			wantOp: ImportOpUpdate, wantKey: "Vendor Satu",
			wantFields: []ImportFieldChange{{Field: "legal_name", Before: "PT A", After: "PT B"}},
		},
		{
			name: "vendor disable", entity: ExportVendors,
			row:    ImportRow{Row: 4, Op: ImportOpUpdate, ID: 1, Toggle: true, Cells: cells("name", "Vendor Satu", "is_active", "false")},
			cur:    cells("name", "Vendor Satu", "is_active", "true"),
			wantOp: ImportOpDisable, wantKey: "Vendor Satu",
			wantFields: []ImportFieldChange{{Field: "is_active", Before: "true", After: "false"}},
		},
		{
			name: "vendor enable", entity: ExportVendors,
			row:    ImportRow{Row: 5, Op: ImportOpUpdate, ID: 1, Toggle: true, Cells: cells("name", "Vendor Satu", "is_active", "true")},
			cur:    cells("name", "Vendor Satu", "is_active", "false"),
			wantOp: ImportOpEnable, wantKey: "Vendor Satu",
			wantFields: []ImportFieldChange{{Field: "is_active", Before: "false", After: "true"}},
		},
		{
			name: "unchanged row has no preview", entity: ExportVendors,
			row: ImportRow{Row: 6, Op: ImportOpUnchanged, ID: 1, Cells: cells("name", "Vendor Satu")}, wantNil: true,
		},
		{
			name: "branch key joins vendor and branch code", entity: ExportVendorBranches,
			row:    ImportRow{Row: 2, Op: ImportOpUpdate, ID: 4, Cells: cells("vendor_code", "V1", "branch_code", "JKT", "branch_name", "Baru", "category", "ATM")},
			cur:    cells("vendor_code", "V1", "branch_code", "JKT", "branch_name", "Lama", "category", "ATM"),
			wantOp: ImportOpUpdate, wantKey: "V1 / JKT",
			wantFields: []ImportFieldChange{{Field: "branch_name", Before: "Lama", After: "Baru"}},
		},
		{
			name: "vault key joins branch_name and vault_name", entity: ExportVendorVaults,
			row:    ImportRow{Row: 2, Op: ImportOpUpdate, ID: 7, Toggle: true, Cells: cells("branch_name", "Cabang JKT", "vault_name", "VLT1", "is_active", "false")},
			cur:    cells("is_active", "true"),
			wantOp: ImportOpDisable, wantKey: "Cabang JKT / VLT1",
			wantFields: []ImportFieldChange{{Field: "is_active", Before: "true", After: "false"}},
		},
		{
			name: "vendor-wide PIC skips the empty branch_code in the key", entity: ExportVendorPICs,
			row:    ImportRow{Row: 2, Op: ImportOpCreate, Cells: cells("vendor_code", "V1", "branch_code", "", "name", "Budi")},
			wantOp: ImportOpCreate, wantKey: "V1 / Budi",
			wantFields: []ImportFieldChange{{Field: "vendor_code", After: "V1"}, {Field: "name", After: "Budi"}},
		},
		{
			name: "ATM update", entity: ExportATMs,
			row:    ImportRow{Row: 2, Op: ImportOpUpdate, ID: 2, Cells: cells("terminal_id", "T2", "brand", "Diebold")},
			cur:    cells("terminal_id", "T2", "brand", "NCR"),
			wantOp: ImportOpUpdate, wantKey: "T2",
			wantFields: []ImportFieldChange{{Field: "brand", Before: "NCR", After: "Diebold"}},
		},
		{
			name: "assignment enable keyed by terminal and start date", entity: ExportATMAssignments,
			row:    ImportRow{Row: 2, Op: ImportOpUpdate, ID: 1, Toggle: true, Cells: cells("terminal_id", "T1", "effective_start_date", "2026-01-01", "is_active", "true")},
			cur:    cells("is_active", "false"),
			wantOp: ImportOpEnable, wantKey: "T1 / 2026-01-01",
			wantFields: []ImportFieldChange{{Field: "is_active", Before: "false", After: "true"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildChangePreview(tt.entity, exportSpecs[tt.entity].header, tt.row, tt.cur)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("want a preview, got nil")
			}
			if got.Op != tt.wantOp || got.Key != tt.wantKey || got.Row != tt.row.Row {
				t.Errorf("got row %d op %q key %q, want row %d op %q key %q", got.Row, got.Op, got.Key, tt.row.Row, tt.wantOp, tt.wantKey)
			}
			if !reflect.DeepEqual(got.Fields, tt.wantFields) {
				t.Errorf("fields = %+v, want %+v", got.Fields, tt.wantFields)
			}
		})
	}
}

// inactiveVendorRepo makes V1 a disabled vendor so an is_active=true row is an enable.
type inactiveVendorRepo struct{ fakeImportRepo }

func (inactiveVendorRepo) Vendors(_ context.Context, after int64, _ string, limit int32) ([]db.ExportVendorsBatchRow, error) {
	all := []db.ExportVendorsBatchRow{{ID: 1, Code: "V1", Name: "Vendor Satu", LegalName: "PT Satu", Npwp: "012345678901234", ContactEmail: "a@v1.id", ContactPhone: "+62 812-345", HqAddress: "= Jl. Mawar", IsActive: false}}
	return page(all, after, limit, func(r db.ExportVendorsBatchRow) int64 { return r.ID }), nil
}

const (
	v1Prefix = "1,V1,"
	v1Tail   = ",PT Satu,012345678901234,a@v1.id,+62 812-345,'= Jl. Mawar,"
)

// DryRun end to end: the counters and Changes agree for every kind of row.
func TestImport_DryRunChangePreview(t *testing.T) {
	tests := []struct {
		name                                         string
		repo                                         MasterDataImportRepo
		csv                                          string
		creates, dataUpdates, enables, disables, unc int
		wantOps                                      []string
	}{
		{"create", fakeImportRepo{}, ",V2,Vendor Dua,,,,,,\n", 1, 0, 0, 0, 0, []string{ImportOpCreate}},
		{"data update", fakeImportRepo{}, v1Prefix + "Vendor Baru" + v1Tail + "true\n", 0, 1, 0, 0, 0, []string{ImportOpUpdate}},
		{"disable", fakeImportRepo{}, v1Prefix + "Vendor Satu" + v1Tail + "false\n", 0, 0, 0, 1, 0, []string{ImportOpDisable}},
		{"enable", inactiveVendorRepo{}, v1Prefix + "Vendor Satu" + v1Tail + "true\n", 0, 0, 1, 0, 0, []string{ImportOpEnable}},
		{"unchanged", fakeImportRepo{}, v1Prefix + "Vendor Satu" + v1Tail + "true\n", 0, 0, 0, 0, 1, nil},
		{"mixed", fakeImportRepo{}, ",V2,Dua,,,,,,\n" + v1Prefix + "Vendor Satu" + v1Tail + "false\n", 1, 0, 0, 1, 0, []string{ImportOpCreate, ImportOpDisable}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := NewMasterDataImporter(tt.repo).DryRun(adminCtx(7), ExportVendors, strings.NewReader(vendorHeader+tt.csv))
			if err != nil {
				t.Fatal(err)
			}
			p := res.Preview
			if len(p.Errors) != 0 {
				t.Fatalf("unexpected errors %+v", p.Errors)
			}
			if p.Creates != tt.creates || p.DataUpdates != tt.dataUpdates || p.Enables != tt.enables || p.Disables != tt.disables || p.Unchanged != tt.unc {
				t.Errorf("counts = %+v", p)
			}
			if p.Updates != p.DataUpdates+p.Enables+p.Disables {
				t.Errorf("Updates %d != data %d + enables %d + disables %d", p.Updates, p.DataUpdates, p.Enables, p.Disables)
			}
			var ops []string
			for _, c := range p.Changes {
				ops = append(ops, c.Op)
			}
			if !reflect.DeepEqual(ops, tt.wantOps) {
				t.Errorf("change ops = %v, want %v", ops, tt.wantOps)
			}
		})
	}
}

func TestImport_DryRunWithErrorsHasNoChanges(t *testing.T) {
	res := dryRun(t, ExportVendors, vendorHeader+",V2,Vendor Dua,,,,,,\n,V3,,,,,,,\n")

	if len(res.Preview.Errors) == 0 {
		t.Fatal("want an error for the nameless vendor")
	}
	if res.Preview.Changes == nil || len(res.Preview.Changes) != 0 {
		t.Errorf("Changes must be empty and non-nil when there are errors, got %#v", res.Preview.Changes)
	}
}

// Property 4a: for any valid file, the preview counts add up and every field is a real change.
func TestProperty_PreviewCountsAndFields(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		codes := rapid.SliceOfNDistinct(rapid.StringMatching(`[A-Z]{2}[0-9]{3}`), 0, 8, func(s string) string { return s }).Draw(t, "newCodes")
		v1 := rapid.SampledFrom([]string{"same", "rename", "disable"}).Draw(t, "v1")

		var b strings.Builder
		b.WriteString(vendorHeader)
		for _, c := range codes {
			fmt.Fprintf(&b, ",%s,Name %s,,,,,,\n", c, c)
		}
		name, active := "Vendor Satu", "true"
		switch v1 {
		case "rename":
			name = "Vendor Baru"
		case "disable":
			active = "false"
		}
		b.WriteString(v1Prefix + name + v1Tail + active + "\n")

		res, err := NewMasterDataImporter(fakeImportRepo{}).DryRun(adminCtx(7), ExportVendors, strings.NewReader(b.String()))
		if err != nil {
			t.Fatal(err)
		}
		p := res.Preview
		if len(p.Errors) != 0 {
			t.Fatalf("generated file should be valid, got %+v", p.Errors)
		}
		if p.Creates != len(codes) {
			t.Fatalf("creates = %d, want %d", p.Creates, len(codes))
		}
		if got := p.Creates + p.DataUpdates + p.Enables + p.Disables; got != len(p.Changes) {
			t.Fatalf("counts sum %d != len(changes) %d", got, len(p.Changes))
		}
		if p.Updates != p.DataUpdates+p.Enables+p.Disables {
			t.Fatalf("Updates %d != %d+%d+%d", p.Updates, p.DataUpdates, p.Enables, p.Disables)
		}
		for _, c := range p.Changes {
			if len(c.Fields) == 0 {
				t.Fatalf("change %+v has no fields", c)
			}
			for _, f := range c.Fields {
				if f.Before == f.After {
					t.Fatalf("field %+v does not change anything", f)
				}
			}
		}
	})
}

// Property 4b: the op a row previews as is the op the real confirm path
// (stageRow -> entity service -> importCollector) stages for that row.
func TestProperty_PreviewOpEqualsStagedOp(t *testing.T) {
	header := exportSpecs[ExportVendorBranches].header
	repo := &fakeVendorBranchAdminRepo{getByIDFunc: func(_ context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) {
		return &db.GetVendorBranchAdminByIDRow{ID: id}, nil
	}}

	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom([]string{"create", "update", "enable", "disable"}).Draw(t, "kind")
		name := rapid.StringMatching(`[A-Za-z0-9]{1,12}`).Draw(t, "name")
		category := rapid.SampledFrom([]string{"ATM", "CASH", "ATM_CASH"}).Draw(t, "category")
		otherCategory := rapid.SampledFrom([]string{"ATM", "CASH", "ATM_CASH"}).Draw(t, "otherCategory")
		region := rapid.SampledFrom([]string{"", "JKT", "BDG"}).Draw(t, "region")
		id := rapid.Int64Range(1, 1000).Draw(t, "id")

		cur := cells("vendor_code", "V1", "branch_code", "B1", "branch_name", name, "category", category, "region_code", region, "is_active", "true")
		var row ImportRow
		switch kind {
		case "create":
			row = ImportRow{Row: 2, Op: ImportOpCreate, Cells: cells("vendor_code", "V1", "branch_code", "B1", "branch_name", name, "category", category, "region_code", region)}
			cur = nil
		case "update":
			next := cells("vendor_code", "V1", "branch_code", "B1", "branch_name", name+"x", "category", otherCategory, "region_code", region, "is_active", "true")
			row = ImportRow{Row: 2, Op: ImportOpUpdate, ID: id, Cells: next}
		default:
			active := map[string]string{"enable": "true", "disable": "false"}[kind]
			cur["is_active"] = map[string]string{"true": "false", "false": "true"}[active]
			next := cells("vendor_code", "V1", "branch_code", "B1", "is_active", active)
			row = ImportRow{Row: 2, Op: ImportOpUpdate, ID: id, Toggle: true, Cells: next}
		}

		preview := buildChangePreview(ExportVendorBranches, header, row, cur)
		if preview == nil {
			t.Fatalf("row %+v must preview", row)
		}
		if preview.Op != kind {
			t.Fatalf("preview op = %q, want %q", preview.Op, kind)
		}
		for _, f := range preview.Fields {
			if f.Before == f.After {
				t.Fatalf("field %+v does not change anything", f)
			}
		}

		collector := &importCollector{row: row.Row}
		svcs := ImportServices{Branches: NewVendorBranchAdminService(repo, collector, &fakeVendorBranchChildCounter{})}
		env := &importEnv{vendors: map[string]int64{"V1": 1}}
		if err := stageRow(adminCtx(7), 7, ExportVendorBranches, row, env, svcs, "127.0.0.1"); err != nil {
			t.Fatalf("stageRow: %v", err)
		}
		if len(collector.reqs) != 1 {
			t.Fatalf("staged %d changes, want 1", len(collector.reqs))
		}
		if staged := collector.reqs[0].req.Op; staged != preview.Op {
			t.Fatalf("staged op %q != preview op %q", staged, preview.Op)
		}
	})
}
