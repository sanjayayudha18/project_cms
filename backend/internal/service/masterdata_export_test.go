package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeExportRepo serves vendors from an in-memory slice with real keyset
// semantics (id > afterID, ordered, limited) and records every call; the other
// entities return one canned row so header/cell alignment can be checked.
type fakeExportRepo struct {
	vendors []db.ExportVendorsBatchRow
	calls   []exportCall
	failAt  int // 1-based call number that returns an error (0 = never)
}

type exportCall struct {
	afterID int64
	status  string
	limit   int32
}

func (f *fakeExportRepo) record(after int64, status string, limit int32) error {
	f.calls = append(f.calls, exportCall{after, status, limit})
	if f.failAt != 0 && len(f.calls) == f.failAt {
		return errors.New("replica connection lost")
	}
	return nil
}

func (f *fakeExportRepo) Vendors(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorsBatchRow, error) {
	if err := f.record(after, status, limit); err != nil {
		return nil, err
	}
	var out []db.ExportVendorsBatchRow
	for _, v := range f.vendors {
		if v.ID > after && int32(len(out)) < limit {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeExportRepo) VendorBranches(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorBranchesBatchRow, error) {
	return []db.ExportVendorBranchesBatchRow{{ID: 1, VendorCode: "V", BranchCode: "B", BranchName: "Cab", Region: "R", LocationID: "9", IsActive: true}}, f.record(after, status, limit)
}

func (f *fakeExportRepo) VendorVaults(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorVaultsBatchRow, error) {
	return []db.ExportVendorVaultsBatchRow{{ID: 1, VendorCode: "V", BranchCode: "B", VaultCode: "VL", Category: "ATM", CurrencyCode: "IDR",
		MinCapacityAmount: "1.00", MaxCapacityAmount: "2.00", Latitude: "-6.2", Longitude: "106.8", OperatingHours: "24h", LocationID: "9", IsActive: true}}, f.record(after, status, limit)
}

func (f *fakeExportRepo) VendorPICs(_ context.Context, after int64, status string, limit int32) ([]db.ExportVendorPicsBatchRow, error) {
	return []db.ExportVendorPicsBatchRow{{ID: 1, VendorCode: "V", BranchCode: "", Name: "Budi", Position: "Kepala", Phone: "+62 812", Email: "b@x.id", IsNotificationRecipient: true, IsActive: true}}, f.record(after, status, limit)
}

func (f *fakeExportRepo) ATMs(_ context.Context, after int64, status string, limit int32) ([]db.ExportATMsBatchRow, error) {
	return []db.ExportATMsBatchRow{{ID: 1, TerminalID: "T1", LocationID: 5, MachineType: "ATM", Brand: "NCR", Model: "M", OperationHours: "24", DeploymentType: "Onsite",
		CapacityAmount: "100.50", LowThresholdAmount: "", CriticalThresholdAmount: "", Blacklisted: false, EscrowAccount: "", PriorityClass: "VIP", IsActive: true}}, f.record(after, status, limit)
}

func (f *fakeExportRepo) ATMAssignments(_ context.Context, after int64, status string, limit int32) ([]db.ExportATMAssignmentsBatchRow, error) {
	return []db.ExportATMAssignmentsBatchRow{{ID: 1, TerminalID: "T1", VendorCode: "V", BranchCode: "B", PackageCode: "P", EffectiveStartDate: "2026-01-01", EffectiveEndDate: "", IsActive: true}}, f.record(after, status, limit)
}

func vendorRows(ids ...int64) []db.ExportVendorsBatchRow {
	out := make([]db.ExportVendorsBatchRow, len(ids))
	for i, id := range ids {
		out[i] = db.ExportVendorsBatchRow{ID: id, Code: "V" + strconv.FormatInt(id, 10), Name: "Vendor " + strconv.FormatInt(id, 10), IsActive: true}
	}
	return out
}

func newTestExporter(repo MasterDataExportRepo, batch int32) *MasterDataExporter {
	e := NewMasterDataExporter(repo)
	e.batchSize = batch
	return e
}

// export runs the exporter as an ADMIN and parses the output back with a real
// csv.Reader (after asserting the BOM), returning header, data rows, row count.
func export(t *testing.T, e *MasterDataExporter, entity, status string) (header []string, rows [][]string, n int) {
	t.Helper()
	var buf bytes.Buffer
	n, err := e.Write(adminCtx(1), entity, status, &buf, nil)
	if err != nil {
		t.Fatalf("Write(%s): %v", entity, err)
	}
	if !strings.HasPrefix(buf.String(), utf8BOM) {
		t.Fatalf("output must start with a UTF-8 BOM, got %q", buf.String()[:min(8, buf.Len())])
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(buf.String(), utf8BOM))).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	return records[0], records[1:], n
}

// The header order is part of the API (T5.2 templates / T5.3 importer depend on
// it). Pinned literally so an accidental reorder or rename fails loudly.
func TestExport_HeaderContract(t *testing.T) {
	want := map[string][]string{
		ExportVendors:        {"id", "code", "name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"},
		ExportVendorBranches: {"id", "vendor_code", "branch_code", "branch_name", "region", "location_id", "is_active"},
		ExportVendorVaults:   {"id", "vendor_code", "branch_code", "vault_code", "category", "currency_code", "min_capacity_amount", "max_capacity_amount", "latitude", "longitude", "operating_hours", "location_id", "is_active"},
		ExportVendorPICs:     {"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active"},
		ExportATMs:           {"id", "terminal_id", "location_id", "machine_type", "brand", "model", "operation_hours", "deployment_type", "capacity_amount", "low_threshold_amount", "critical_threshold_amount", "blacklisted", "escrow_account", "priority_class", "is_active"},
		ExportATMAssignments: {"id", "terminal_id", "vendor_code", "branch_code", "package_code", "effective_start_date", "effective_end_date", "is_active"},
	}
	if len(exportSpecs) != len(want) {
		t.Fatalf("exportSpecs has %d entities, contract lists %d", len(exportSpecs), len(want))
	}
	for entity, header := range want {
		got, err := MasterDataExportHeader(entity)
		if err != nil || !reflect.DeepEqual(got, header) {
			t.Errorf("%s header = %v (err %v), want %v", entity, got, err, header)
		}
	}
	// The returned header is a copy: mutating it must not corrupt the contract.
	h, _ := MasterDataExportHeader(ExportVendors)
	h[0] = "tampered"
	if again, _ := MasterDataExportHeader(ExportVendors); again[0] != "id" {
		t.Error("MasterDataExportHeader must return a copy")
	}
}

// Every entity's rows must have exactly as many cells as the header, in the
// header's order (guards the hand-written row mappers against drift).
func TestExport_CellsAlignWithHeader(t *testing.T) {
	for entity := range exportSpecs {
		t.Run(entity, func(t *testing.T) {
			repo := &fakeExportRepo{vendors: vendorRows(1)}
			header, rows, n := export(t, newTestExporter(repo, 1000), entity, "active")
			if n != 1 || len(rows) != 1 {
				t.Fatalf("want 1 row, got n=%d rows=%d", n, len(rows))
			}
			if len(rows[0]) != len(header) {
				t.Errorf("%s row has %d cells, header has %d", entity, len(rows[0]), len(header))
			}
			if rows[0][0] != "1" {
				t.Errorf("first column must be the id, got %q", rows[0][0])
			}
		})
	}
}

func TestExport_KeysetPaging(t *testing.T) {
	t.Run("partial last page ends without an extra query", func(t *testing.T) {
		repo := &fakeExportRepo{vendors: vendorRows(10, 20, 30, 40, 50)}
		_, rows, n := export(t, newTestExporter(repo, 2), ExportVendors, "all")

		if n != 5 || len(rows) != 5 {
			t.Fatalf("want 5 rows, got n=%d rows=%d", n, len(rows))
		}
		wantCalls := []exportCall{{0, "all", 2}, {20, "all", 2}, {40, "all", 2}}
		if !reflect.DeepEqual(repo.calls, wantCalls) {
			t.Errorf("calls = %+v, want keyset cursor advancing to the last id of each page: %+v", repo.calls, wantCalls)
		}
		for i, id := range []string{"10", "20", "30", "40", "50"} {
			if rows[i][0] != id {
				t.Errorf("row %d id = %s, want %s (ordered, no gaps or duplicates)", i, rows[i][0], id)
			}
		}
	})

	t.Run("exact multiple of the batch size needs one final empty page", func(t *testing.T) {
		repo := &fakeExportRepo{vendors: vendorRows(1, 2, 3, 4)}
		_, rows, _ := export(t, newTestExporter(repo, 2), ExportVendors, "active")
		if len(rows) != 4 || len(repo.calls) != 3 {
			t.Errorf("rows=%d calls=%d, want 4 rows over 3 calls (last page empty)", len(rows), len(repo.calls))
		}
	})

	t.Run("empty table still yields BOM + header", func(t *testing.T) {
		header, rows, n := export(t, newTestExporter(&fakeExportRepo{}, 1000), ExportVendors, "active")
		if n != 0 || len(rows) != 0 || header[0] != "id" {
			t.Errorf("want header only, got header=%v rows=%d n=%d", header, len(rows), n)
		}
	})

	t.Run("status is passed to every page", func(t *testing.T) {
		repo := &fakeExportRepo{vendors: vendorRows(1, 2, 3)}
		export(t, newTestExporter(repo, 2), ExportVendors, "disabled")
		for _, c := range repo.calls {
			if c.status != "disabled" {
				t.Errorf("call %+v: status must be disabled", c)
			}
		}
	})
}

func TestExport_FlushesAfterEveryPage(t *testing.T) {
	repo := &fakeExportRepo{vendors: vendorRows(1, 2, 3, 4, 5)}
	flushes := 0
	var buf bytes.Buffer
	if _, err := newTestExporter(repo, 2).Write(adminCtx(1), ExportVendors, "active", &buf, func() { flushes++ }); err != nil {
		t.Fatal(err)
	}
	if flushes != 3 {
		t.Errorf("flush called %d times, want once per page (3)", flushes)
	}
}

func TestCSVSafe(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"PT Acme", "PT Acme"},
		{"=HYPERLINK(\"http://evil\")", "'=HYPERLINK(\"http://evil\")"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
		{"+cmd|' /C calc'!A0", "'+cmd|' /C calc'!A0"},
		{"-2+3", "'-2+3"},
		{"-cmd", "'-cmd"},
		// Legitimate data starting with + or - must NOT be altered.
		{"+62 812-3456-7890", "+62 812-3456-7890"},
		{"+6281234567890", "+6281234567890"},
		{"(021) 555-1234", "(021) 555-1234"},
		{"-6.200000", "-6.200000"},
		{"106.816666", "106.816666"},
		{"-", "'-"},
	}
	for _, tt := range tests {
		if got := csvSafe(tt.in); got != tt.want {
			t.Errorf("csvSafe(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// End to end: a hostile vendor name is defused in the file, while awkward but
// harmless values (commas, quotes, newlines, unicode, a phone with +) survive a
// real csv.Reader round trip unchanged.
func TestExport_InjectionDefusedAndAwkwardValuesRoundTrip(t *testing.T) {
	repo := &fakeExportRepo{vendors: []db.ExportVendorsBatchRow{
		{ID: 1, Code: "EVIL", Name: `=HYPERLINK("http://evil","x")`, HqAddress: "Jl. A, No. 1\nLt. \"3\"", ContactPhone: "+62 812-3456", IsActive: true},
		{ID: 2, Code: "UNI", Name: "PT Sinar Jaya — Cabang Utama ✓", LegalName: "  spaced  ", IsActive: false},
	}}
	_, rows, _ := export(t, newTestExporter(repo, 1000), ExportVendors, "all")

	if rows[0][2] != `'=HYPERLINK("http://evil","x")` {
		t.Errorf("formula name not defused, got %q", rows[0][2])
	}
	if rows[0][7] != "Jl. A, No. 1\nLt. \"3\"" {
		t.Errorf("address with comma/newline/quotes must round-trip, got %q", rows[0][7])
	}
	if rows[0][6] != "+62 812-3456" {
		t.Errorf("phone starting with + must be left alone, got %q", rows[0][6])
	}
	if rows[1][2] != "PT Sinar Jaya — Cabang Utama ✓" || rows[1][3] != "  spaced  " || rows[1][8] != "false" {
		t.Errorf("unicode/whitespace/bool cells wrong: %q", rows[1])
	}
}

func TestExport_FailureModes(t *testing.T) {
	t.Run("first page failure writes nothing", func(t *testing.T) {
		repo := &fakeExportRepo{vendors: vendorRows(1, 2, 3), failAt: 1}
		var buf bytes.Buffer
		_, err := newTestExporter(repo, 2).Write(adminCtx(1), ExportVendors, "active", &buf, nil)
		if err == nil || buf.Len() != 0 {
			t.Fatalf("err=%v written=%d bytes; want an error and NOTHING written so the caller can still send a normal error response", err, buf.Len())
		}
	})

	t.Run("later page failure returns an error after partial output", func(t *testing.T) {
		repo := &fakeExportRepo{vendors: vendorRows(1, 2, 3, 4), failAt: 2}
		var buf bytes.Buffer
		n, err := newTestExporter(repo, 2).Write(adminCtx(1), ExportVendors, "active", &buf, nil)
		if err == nil || n != 2 || buf.Len() == 0 {
			t.Fatalf("n=%d err=%v written=%d; want the first page written and an error returned (never silent success)", n, err, buf.Len())
		}
	})

	t.Run("cancelled context stops the export", func(t *testing.T) {
		ctx, cancel := context.WithCancel(adminCtx(1))
		cancel()
		var buf bytes.Buffer
		if _, err := newTestExporter(&fakeExportRepo{vendors: vendorRows(1)}, 2).Write(ctx, ExportVendors, "active", &buf, nil); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("unknown entity and bad status", func(t *testing.T) {
		e := newTestExporter(&fakeExportRepo{}, 2)
		if _, err := e.Write(adminCtx(1), "users", "active", &bytes.Buffer{}, nil); !errors.Is(err, ErrExportUnknownEntity) {
			t.Errorf("unknown entity err = %v", err)
		}
		var ve *ValidationError
		if _, err := e.Write(adminCtx(1), ExportVendors, "bogus", &bytes.Buffer{}, nil); !errors.As(err, &ve) || ve.Field != "status" {
			t.Errorf("bad status err = %v, want ValidationError{status}", err)
		}
	})
}

func TestExport_ServiceLayerRBAC(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"ADMIN", ctxWithRole(1, "ADMIN"), nil},
		{"ADMIN_PARAM", ctxWithRole(1, "ADMIN_PARAM"), nil},
		{"no auth context", context.Background(), ErrMasterDataForbidden},
		{"vendor role", ctxWithRole(1, "VENDOR"), ErrMasterDataForbidden},
		{"APPACCESS", ctxWithRole(1, "APPACCESS"), ErrMasterDataForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeExportRepo{vendors: vendorRows(1)}
			var buf bytes.Buffer
			_, err := newTestExporter(repo, 10).Write(tc.ctx, ExportVendors, "active", &buf, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if tc.want != nil && (buf.Len() != 0 || len(repo.calls) != 0) {
				t.Errorf("a forbidden export must not query or write anything (bytes=%d calls=%d)", buf.Len(), len(repo.calls))
			}
		})
	}
}
