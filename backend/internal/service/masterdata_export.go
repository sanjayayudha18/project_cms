package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Export entities (the {entity} path segment of the export endpoint). The CSV
// contract (headers, cells, BOM, CSVSafe) lives in pkg/csvexport, shared with
// backend-exq (vendor-data-export task 3.2); these names are aliases.
const (
	ExportVendors        = csvexport.Vendors
	ExportVendorBranches = csvexport.VendorBranches
	ExportVendorVaults   = csvexport.VendorVaults
	ExportVendorPICs     = csvexport.VendorPICs
	ExportATMs           = csvexport.ATMs
	ExportATMAssignments = csvexport.ATMAssignments
)

// masterDataExportBatchSize is the keyset page size: how many rows are held in
// memory (and flushed to the client) at a time, whatever the table size.
const masterDataExportBatchSize int32 = 1000

// utf8BOM lets Excel open the file as UTF-8 (Indonesian names/addresses).
// The T5.3 importer must strip it.
const utf8BOM = csvexport.BOM

// ErrExportUnknownEntity is returned for an {entity} that is not exportable.
var ErrExportUnknownEntity = errors.New("unknown export entity")

// MasterDataExportRepo is the read surface MasterDataExporter needs: one keyset
// page per entity. *repository.MasterDataExportRepository satisfies it.
type MasterDataExportRepo interface {
	Vendors(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorsBatchRow, error)
	VendorBranches(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorBranchesBatchRow, error)
	VendorVaults(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorVaultsBatchRow, error)
	VendorPICs(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportVendorPicsBatchRow, error)
	ATMs(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportATMsBatchRow, error)
	ATMAssignments(ctx context.Context, afterID int64, status string, limit int32) ([]db.ExportATMAssignmentsBatchRow, error)
}

// exportSpec is one entity's stable CSV header and its page -> cells mapping.
type exportSpec struct {
	header []string
	// legacyHeaders are older headers the importer still accepts: the same
	// columns minus some; each missing column is filled with legacyFill[column].
	legacyHeaders [][]string
	legacyFill    map[string]string
	page          func(ctx context.Context, r MasterDataExportRepo, afterID int64, status string, limit int32) (rows [][]string, lastID int64, err error)
}

// exportPage converts one repo page into CSV cells and the last row's id (the
// next keyset cursor).
func exportPage[R any](rows []R, err error, id func(R) int64, cells func(R) []string) ([][]string, int64, error) {
	if err != nil {
		return nil, 0, err
	}
	out := make([][]string, len(rows))
	var last int64
	for i, r := range rows {
		out[i], last = cells(r), id(r)
	}
	return out, last, nil
}

// picRolesCellNone is the roles cell of a PIC with an empty role set.
const picRolesCellNone = csvexport.PICRolesNone

// spec builds an exportSpec from the shared contract entity plus this module's
// page adapter (sqlc row -> csvexport row -> cells).
func spec(entity string, page func(ctx context.Context, r MasterDataExportRepo, afterID int64, status string, limit int32) ([][]string, int64, error)) exportSpec {
	e, ok := csvexport.Lookup(entity)
	if !ok {
		panic("csvexport: unknown entity " + entity)
	}
	return exportSpec{header: e.Header, legacyHeaders: e.LegacyHeaders, legacyFill: e.LegacyFill, page: page}
}

// exportSpecs is the contract with the T5.3 importer and T5.2 templates.
// Headers, legacy headers and cell formats come from pkg/csvexport (one source
// for export and import, Req 14.3/14.4); only the repo paging lives here.
// Each sqlc row converts directly to its csvexport row (same fields, same order).
var exportSpecs = map[string]exportSpec{
	ExportVendors: spec(ExportVendors, func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
		rows, err := r.Vendors(ctx, after, status, limit)
		return exportPage(rows, err, func(x db.ExportVendorsBatchRow) int64 { return x.ID }, func(x db.ExportVendorsBatchRow) []string {
			return csvexport.VendorCells(csvexport.VendorRow(x))
		})
	}),
	ExportVendorBranches: spec(ExportVendorBranches, func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
		rows, err := r.VendorBranches(ctx, after, status, limit)
		return exportPage(rows, err, func(x db.ExportVendorBranchesBatchRow) int64 { return x.ID }, func(x db.ExportVendorBranchesBatchRow) []string {
			return csvexport.VendorBranchCells(csvexport.VendorBranchRow(x))
		})
	}),
	ExportVendorVaults: spec(ExportVendorVaults, func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
		rows, err := r.VendorVaults(ctx, after, status, limit)
		return exportPage(rows, err, func(x db.ExportVendorVaultsBatchRow) int64 { return x.ID }, func(x db.ExportVendorVaultsBatchRow) []string {
			return csvexport.VendorVaultCells(csvexport.VendorVaultRow(x))
		})
	}),
	ExportVendorPICs: spec(ExportVendorPICs, func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
		rows, err := r.VendorPICs(ctx, after, status, limit)
		return exportPage(rows, err, func(x db.ExportVendorPicsBatchRow) int64 { return x.ID }, func(x db.ExportVendorPicsBatchRow) []string {
			return csvexport.VendorPICCells(csvexport.VendorPICRow(x))
		})
	}),
	ExportATMs: spec(ExportATMs, func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
		rows, err := r.ATMs(ctx, after, status, limit)
		return exportPage(rows, err, func(x db.ExportATMsBatchRow) int64 { return x.ID }, func(x db.ExportATMsBatchRow) []string {
			return csvexport.ATMCells(csvexport.ATMRow(x))
		})
	}),
	ExportATMAssignments: spec(ExportATMAssignments, func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
		rows, err := r.ATMAssignments(ctx, after, status, limit)
		return exportPage(rows, err, func(x db.ExportATMAssignmentsBatchRow) int64 { return x.ID }, func(x db.ExportATMAssignmentsBatchRow) []string {
			return csvexport.ATMAssignmentCells(csvexport.ATMAssignmentRow(x))
		})
	}),
}

// CSVSafe defuses spreadsheet formula injection (see csvexport.CSVSafe); kept
// for the other CSV writers in this module (replenishment report).
func CSVSafe(s string) string { return csvexport.CSVSafe(s) }

// MasterDataExporter streams master-data tables as CSV. Read-only; it never
// writes audit entries (same convention as the admin list endpoints).
type MasterDataExporter struct {
	repo      MasterDataExportRepo
	batchSize int32
}

// NewMasterDataExporter creates a MasterDataExporter over repo (the read-replica repository).
func NewMasterDataExporter(repo MasterDataExportRepo) *MasterDataExporter {
	return &MasterDataExporter{repo: repo, batchSize: masterDataExportBatchSize}
}

// Write streams entity as CSV to w and returns the number of data rows
// written. status is "active" | "disabled" | "all". The first page is fetched
// BEFORE anything is written, so a failure there leaves w untouched and the
// caller can still answer with a normal error response; after that, w has been
// written to and an error means a truncated file (the caller must abort the
// connection rather than present it as complete). flush, if non-nil, is called
// after every page so the client receives data as it is produced.
func (e *MasterDataExporter) Write(ctx context.Context, entity, status string, w io.Writer, flush func()) (int, error) {
	if err := authorizeMasterDataAdmin(ctx); err != nil {
		return 0, err
	}
	spec, ok := exportSpecs[entity]
	if !ok {
		return 0, ErrExportUnknownEntity
	}
	if status != "active" && status != "disabled" && status != "all" {
		return 0, &ValidationError{Field: "status", Message: "harus active, disabled, atau all"}
	}

	var cw *csvexport.Writer
	total, afterID := 0, int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		rows, lastID, err := spec.page(ctx, e.repo, afterID, status, e.batchSize)
		if err != nil {
			return total, fmt.Errorf("export %s after id %d: %w", entity, afterID, err)
		}

		if cw == nil { // BOM + header only once the first page is in hand
			if cw, err = csvexport.NewWriter(w, spec.header); err != nil {
				return total, err
			}
		}
		for _, row := range rows {
			if err := cw.Write(row); err != nil {
				return total, err
			}
		}
		total += len(rows)
		if err := cw.Flush(); err != nil {
			return total, err
		}
		if flush != nil {
			flush()
		}

		if int32(len(rows)) < e.batchSize { //nolint:gosec // G115: len(rows) <= batchSize (SQL LIMIT)
			return total, nil
		}
		afterID = lastID
	}
}
