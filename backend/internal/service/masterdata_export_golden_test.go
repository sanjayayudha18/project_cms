package service

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Golden files that pin the export's bytes BEFORE it moves to pkg/csvexport +
// backend-exq (vendor-data-export tasks 1.1/1.2, Property 3). The fixture is an
// in-memory repo, not the dev DB: a global export reads every row of the table
// and identity ids differ per database, so a DB fixture could not stay
// byte-stable. SQL cell formatting (NULL -> ”, numeric::text) is covered by the
// importer round trip in task 3.3.
//
// Regenerate only on purpose (the files are the contract):
//
//	go test ./internal/service -run TestExportGolden -update
var updateGolden = flag.Bool("update", false, "rewrite master-data export golden files")

const (
	goldenExportDir = "testdata/master_data_export"
	// Shared with pkg/csvexport (task 2.2 snapshots its headers against this file).
	goldenHeadersFile = "../../../pkg/csvexport/testdata/headers.golden"
)

// goldenExportRepo serves fixed rows per entity with the SQL's semantics: status
// filter on is_active, keyset id > afterID, ordered by id, LIMIT.
type goldenExportRepo struct{}

func goldenPage[R any](rows []R, id func(R) int64, active func(R) bool, after int64, status string, limit int32) []R {
	var out []R
	for _, r := range rows {
		if id(r) <= after || (status == "active" && !active(r)) || (status == "disabled" && active(r)) {
			continue
		}
		if int32(len(out)) == limit { //nolint:gosec // G115: test slice, tiny
			break
		}
		out = append(out, r)
	}
	return out
}

func (goldenExportRepo) Vendors(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorsBatchRow, error) {
	return goldenPage(goldenVendors, func(r db.ExportVendorsBatchRow) int64 { return r.ID }, func(r db.ExportVendorsBatchRow) bool { return r.IsActive }, after, status, limit), nil
}

func (goldenExportRepo) VendorBranches(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorBranchesBatchRow, error) {
	return goldenPage(goldenBranches, func(r db.ExportVendorBranchesBatchRow) int64 { return r.ID }, func(r db.ExportVendorBranchesBatchRow) bool { return r.IsActive }, after, status, limit), nil
}

func (goldenExportRepo) VendorVaults(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorVaultsBatchRow, error) {
	return goldenPage(goldenVaults, func(r db.ExportVendorVaultsBatchRow) int64 { return r.ID }, func(r db.ExportVendorVaultsBatchRow) bool { return r.IsActive }, after, status, limit), nil
}

func (goldenExportRepo) VendorPICs(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorPicsBatchRow, error) {
	return goldenPage(goldenPICs, func(r db.ExportVendorPicsBatchRow) int64 { return r.ID }, func(r db.ExportVendorPicsBatchRow) bool { return r.IsActive }, after, status, limit), nil
}

func (goldenExportRepo) ATMs(_ context.Context, after int64, status string, limit int32) ([]db.ExportATMsBatchRow, error) {
	return goldenPage(goldenATMs, func(r db.ExportATMsBatchRow) int64 { return r.ID }, func(r db.ExportATMsBatchRow) bool { return r.IsActive }, after, status, limit), nil
}

func (goldenExportRepo) ATMAssignments(_ context.Context, after int64, status string, limit int32) ([]db.ExportATMAssignmentsBatchRow, error) {
	return goldenPage(goldenAssignments, func(r db.ExportATMAssignmentsBatchRow) int64 { return r.ID }, func(r db.ExportATMAssignmentsBatchRow) bool { return r.IsActive }, after, status, limit), nil
}

// Fixture: active + disabled rows, empty (NULL) cells, numeric text, and cells
// starting with = + - @ (CSVSafe), plus commas/quotes/newlines/unicode. Fake data only.
var (
	goldenVendors = []db.ExportVendorsBatchRow{
		{ID: 1, Code: "VA", Name: "PT Alpha Kas", LegalName: "PT Alpha Kas Nusantara", Npwp: "01.234.567.8-901.000", ContactEmail: "ops@alpha.example", ContactPhone: "+62 21 555-0101", HqAddress: "Jl. Satu, No. 1\nLt. \"3\"", IsActive: true},
		{ID: 2, Code: "VB", Name: "=SUM(A1)", IsActive: false},
		{ID: 5, Code: "VC", Name: "PT Cahaya — Utama ✓", LegalName: "-cmd", ContactPhone: "(021) 555-1234", IsActive: true},
	}
	goldenBranches = []db.ExportVendorBranchesBatchRow{
		{ID: 1, VendorCode: "VA", BranchCode: "VA_001", BranchName: "Alpha Medan", Region: "Sumatera", RegionCode: "MDN", Category: "ATM_CASH", LocationID: "12", IsActive: true},
		{ID: 3, VendorCode: "VA", BranchCode: "VA_002", BranchName: "Alpha Lama", IsActive: false},
		{ID: 4, VendorCode: "VC", BranchCode: "VC_001", BranchName: "@Cabang, Bandung", Region: "Jawa Barat", RegionCode: "BDG", Category: "CASH", IsActive: true},
	}
	goldenVaults = []db.ExportVendorVaultsBatchRow{
		{ID: 1, VendorCode: "VA", BranchCode: "VA_001", VaultCode: "VA_001_V1", Category: "ATM_CASH", CurrencyCode: "IDR", MinCapacityAmount: "1000000.00", MaxCapacityAmount: "5000000000.00", Latitude: "-6.200000", Longitude: "106.816666", OperatingHours: "24 jam", LocationID: "12", IsActive: true},
		{ID: 2, VendorCode: "VA", BranchCode: "VA_002", VaultCode: "VA_002_V1", Category: "CASH", CurrencyCode: "IDR", IsActive: false},
		{ID: 7, VendorCode: "VC", BranchCode: "VC_001", VaultCode: "VC_001_V1", Category: "ATM", CurrencyCode: "IDR", MinCapacityAmount: "0.00", MaxCapacityAmount: "999999999999999999.99", OperatingHours: "+abc", IsActive: true},
	}
	goldenPICs = []db.ExportVendorPicsBatchRow{
		{ID: 1, VendorCode: "VA", Name: "Budi Contoh", Position: "Kepala Operasional", Phone: "+62 812-3456-7890", Email: "budi@alpha.example", IsNotificationRecipient: true, IsActive: true, Roles: []string{"replenish", "vault", "cashcount"}},
		{ID: 2, VendorCode: "VA", BranchCode: "VA_001", Name: "Sari Contoh", Phone: "0812000111", IsActive: false},
		{ID: 6, VendorCode: "VC", BranchCode: "VC_001", Name: "=HYPERLINK(\"http://evil\")", Email: "@x.example", IsActive: true, Roles: []string{"vault"}},
	}
	goldenATMs = []db.ExportATMsBatchRow{
		{ID: 1, TerminalID: "T0001", LocationID: 12, MachineType: "ATM", Brand: "NCR", Model: "SelfServ 22", OperationHours: "24", DeploymentType: "Onsite", CapacityAmount: "200000000.00", LowThresholdAmount: "50000000.00", CriticalThresholdAmount: "20000000.00", EscrowAccount: "1234567890", PriorityClass: "VIP", IsActive: true},
		{ID: 2, TerminalID: "T0002", LocationID: 13, MachineType: "CRM", Blacklisted: true, IsActive: false},
		{ID: 9, TerminalID: "T0009", LocationID: 12, MachineType: "ATM", Brand: "Hyosung", Model: "-X1", CapacityAmount: "0.00", IsActive: true},
	}
	goldenAssignments = []db.ExportATMAssignmentsBatchRow{
		{ID: 1, TerminalID: "T0001", VendorCode: "VA", BranchCode: "VA_001", PackageSource: "branch", PackageCode: "VA_001_PKT3", EffectiveStartDate: "2026-01-01", IsActive: true},
		{ID: 2, TerminalID: "T0002", VendorCode: "VA", BranchCode: "VA_001", PackageSource: "vendor", PackageCode: "PAKET 3", EffectiveStartDate: "2025-01-01", EffectiveEndDate: "2025-12-31", IsActive: false},
		{ID: 4, TerminalID: "T0009", VendorCode: "VC", BranchCode: "VC_001", PackageSource: "vendor", PackageCode: "PAKET 1", EffectiveStartDate: "2026-10-01", IsActive: true},
	}
)

func exportEntityNames() []string {
	names := make([]string, 0, len(exportSpecs))
	for n := range exportSpecs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func checkGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil { //nolint:gosec // G306: test fixture
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // G304: fixed test path
	if err != nil {
		t.Fatalf("read golden %s (run with -update to record): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden:\n got: %q\nwant: %q", path, got, want)
	}
}

// Task 1.1: byte-for-byte output of MasterDataExporter.Write per entity × status.
// Batch size 2 so every file crosses keyset page boundaries.
func TestExportGolden_Bytes(t *testing.T) {
	for _, entity := range exportEntityNames() {
		for _, status := range []string{"active", "disabled", "all"} {
			t.Run(entity+"-"+status, func(t *testing.T) {
				var buf bytes.Buffer
				if _, err := newTestExporter(goldenExportRepo{}, 2).Write(adminCtx(1), entity, status, &buf, nil); err != nil {
					t.Fatal(err)
				}
				checkGolden(t, filepath.Join(goldenExportDir, entity+"-"+status+".csv"), buf.Bytes())
			})
		}
	}
}

// Task 1.2: header, legacy headers and legacy fill of every entity, one line each.
func TestExportGolden_Headers(t *testing.T) {
	var b strings.Builder
	for _, entity := range exportEntityNames() {
		spec := exportSpecs[entity]
		fmt.Fprintf(&b, "%s header: %s\n", entity, strings.Join(spec.header, ","))
		for i, h := range spec.legacyHeaders {
			fmt.Fprintf(&b, "%s legacy[%d]: %s\n", entity, i, strings.Join(h, ","))
		}
		if len(spec.legacyFill) > 0 {
			cols := make([]string, 0, len(spec.legacyFill))
			for c, v := range spec.legacyFill {
				cols = append(cols, c+"="+v)
			}
			sort.Strings(cols)
			fmt.Fprintf(&b, "%s legacy fill: %s\n", entity, strings.Join(cols, ","))
		}
	}
	checkGolden(t, goldenHeadersFile, []byte(b.String()))
}
