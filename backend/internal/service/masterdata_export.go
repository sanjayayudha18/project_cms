package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Export entities (the {entity} path segment of the export endpoint).
const (
	ExportVendors        = "vendors"
	ExportVendorBranches = "vendor-branches"
	ExportVendorVaults   = "vendor-vaults"
	ExportVendorPICs     = "vendor-pics"
	ExportATMs           = "atms"
	ExportATMAssignments = "atm-assignments"
)

// masterDataExportBatchSize is the keyset page size: how many rows are held in
// memory (and flushed to the client) at a time, whatever the table size.
const masterDataExportBatchSize int32 = 1000

// utf8BOM lets Excel open the file as UTF-8 (Indonesian names/addresses).
// The T5.3 importer must strip it.
const utf8BOM = "\xEF\xBB\xBF"

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
	page   func(ctx context.Context, r MasterDataExportRepo, afterID int64, status string, limit int32) (rows [][]string, lastID int64, err error)
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

func boolCell(b bool) string { return strconv.FormatBool(b) }

// exportSpecs is the contract with the T5.3 importer and T5.2 templates: the
// header order is stable and part of the API. Columns use natural keys
// (vendor_code, branch_code, terminal_id, package_code) so a file round-trips
// without depending on database ids; `id` is kept as the first column for
// reference. Empty cell = NULL; amounts/dates are the exact stored text.
var exportSpecs = map[string]exportSpec{
	ExportVendors: {
		header: []string{"id", "code", "name", "legal_name", "npwp", "contact_email", "contact_phone", "hq_address", "is_active"},
		page: func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
			rows, err := r.Vendors(ctx, after, status, limit)
			return exportPage(rows, err, func(x db.ExportVendorsBatchRow) int64 { return x.ID }, func(x db.ExportVendorsBatchRow) []string {
				return []string{strconv.FormatInt(x.ID, 10), x.Code, x.Name, x.LegalName, x.Npwp, x.ContactEmail, x.ContactPhone, x.HqAddress, boolCell(x.IsActive)}
			})
		},
	},
	ExportVendorBranches: {
		header: []string{"id", "vendor_code", "branch_code", "branch_name", "region", "location_id", "is_active"},
		page: func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
			rows, err := r.VendorBranches(ctx, after, status, limit)
			return exportPage(rows, err, func(x db.ExportVendorBranchesBatchRow) int64 { return x.ID }, func(x db.ExportVendorBranchesBatchRow) []string {
				return []string{strconv.FormatInt(x.ID, 10), x.VendorCode, x.BranchCode, x.BranchName, x.Region, x.LocationID, boolCell(x.IsActive)}
			})
		},
	},
	ExportVendorVaults: {
		header: []string{"id", "vendor_code", "branch_code", "vault_code", "category", "currency_code", "min_capacity_amount", "max_capacity_amount", "latitude", "longitude", "operating_hours", "location_id", "is_active"},
		page: func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
			rows, err := r.VendorVaults(ctx, after, status, limit)
			return exportPage(rows, err, func(x db.ExportVendorVaultsBatchRow) int64 { return x.ID }, func(x db.ExportVendorVaultsBatchRow) []string {
				return []string{strconv.FormatInt(x.ID, 10), x.VendorCode, x.BranchCode, x.VaultCode, x.Category, x.CurrencyCode,
					x.MinCapacityAmount, x.MaxCapacityAmount, x.Latitude, x.Longitude, x.OperatingHours, x.LocationID, boolCell(x.IsActive)}
			})
		},
	},
	ExportVendorPICs: {
		header: []string{"id", "vendor_code", "branch_code", "name", "position", "phone", "email", "is_notification_recipient", "is_active"},
		page: func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
			rows, err := r.VendorPICs(ctx, after, status, limit)
			return exportPage(rows, err, func(x db.ExportVendorPicsBatchRow) int64 { return x.ID }, func(x db.ExportVendorPicsBatchRow) []string {
				return []string{strconv.FormatInt(x.ID, 10), x.VendorCode, x.BranchCode, x.Name, x.Position, x.Phone, x.Email, boolCell(x.IsNotificationRecipient), boolCell(x.IsActive)}
			})
		},
	},
	ExportATMs: {
		header: []string{"id", "terminal_id", "location_id", "machine_type", "brand", "model", "operation_hours", "deployment_type", "capacity_amount", "low_threshold_amount", "critical_threshold_amount", "blacklisted", "escrow_account", "priority_class", "is_active"},
		page: func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
			rows, err := r.ATMs(ctx, after, status, limit)
			return exportPage(rows, err, func(x db.ExportATMsBatchRow) int64 { return x.ID }, func(x db.ExportATMsBatchRow) []string {
				return []string{strconv.FormatInt(x.ID, 10), x.TerminalID, strconv.FormatInt(x.LocationID, 10), x.MachineType, x.Brand, x.Model, x.OperationHours, x.DeploymentType,
					x.CapacityAmount, x.LowThresholdAmount, x.CriticalThresholdAmount, boolCell(x.Blacklisted), x.EscrowAccount, x.PriorityClass, boolCell(x.IsActive)}
			})
		},
	},
	ExportATMAssignments: {
		header: []string{"id", "terminal_id", "vendor_code", "branch_code", "package_code", "effective_start_date", "effective_end_date", "is_active"},
		page: func(ctx context.Context, r MasterDataExportRepo, after int64, status string, limit int32) ([][]string, int64, error) {
			rows, err := r.ATMAssignments(ctx, after, status, limit)
			return exportPage(rows, err, func(x db.ExportATMAssignmentsBatchRow) int64 { return x.ID }, func(x db.ExportATMAssignmentsBatchRow) []string {
				return []string{strconv.FormatInt(x.ID, 10), x.TerminalID, x.VendorCode, x.BranchCode, x.PackageCode, x.EffectiveStartDate, x.EffectiveEndDate, boolCell(x.IsActive)}
			})
		},
	},
}

// ParseExportEntity validates the {entity} path segment.
func ParseExportEntity(name string) (string, error) {
	if _, ok := exportSpecs[name]; !ok {
		return "", ErrExportUnknownEntity
	}
	return name, nil
}

// MasterDataExportHeader returns entity's stable CSV header (a copy), for the
// blank-template download (T5.2) and the importer's header validation (T5.3).
func MasterDataExportHeader(entity string) ([]string, error) {
	spec, ok := exportSpecs[entity]
	if !ok {
		return nil, ErrExportUnknownEntity
	}
	return append([]string(nil), spec.header...), nil
}

// WriteExportTemplate writes entity's blank CSV template: BOM + header row only
// (no DB access). The `id` column is kept for parity with the export, so a
// filled template and an edited export share one importer format (T5.3):
// empty id = create, id present = update.
func WriteExportTemplate(w io.Writer, entity string) error {
	header, err := MasterDataExportHeader(entity)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// plainNumberRe matches a bare number/phone ("+62 812-345", "-6.2", "(021) 555"),
// which csvSafe leaves alone even though it starts with + or -.
var plainNumberRe = regexp.MustCompile(`^[+-]?[0-9][0-9 ().-]*$`)

// csvSafe defuses spreadsheet formula injection: a text cell starting with
// = @ TAB CR (or + / - when it is not a plain number or phone) gets a leading
// apostrophe, so opening an export in Excel cannot execute a formula that an
// admin typed into a vendor name or address. The importer (T5.3) strips one
// leading apostrophe that precedes such a character.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '@', '\t', '\r':
		return "'" + s
	case '+', '-':
		if !plainNumberRe.MatchString(s) {
			return "'" + s
		}
	}
	return s
}

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

// Authorize re-checks, independent of the route guard (RBAC at middleware AND
// service), that the caller holds a master-data admin role. An export is a
// bulk read of vendor/ATM master data.
func (e *MasterDataExporter) Authorize(ctx context.Context) error {
	return authorizeMasterDataAdmin(ctx)
}

// Write streams entity as CSV to w and returns the number of data rows
// written. status is "active" | "disabled" | "all". The first page is fetched
// BEFORE anything is written, so a failure there leaves w untouched and the
// caller can still answer with a normal error response; after that, w has been
// written to and an error means a truncated file (the caller must abort the
// connection rather than present it as complete). flush, if non-nil, is called
// after every page so the client receives data as it is produced.
func (e *MasterDataExporter) Write(ctx context.Context, entity, status string, w io.Writer, flush func()) (int, error) {
	if err := e.Authorize(ctx); err != nil {
		return 0, err
	}
	spec, ok := exportSpecs[entity]
	if !ok {
		return 0, ErrExportUnknownEntity
	}
	if status != "active" && status != "disabled" && status != "all" {
		return 0, &ValidationError{Field: "status", Message: "harus active, disabled, atau all"}
	}

	cw := csv.NewWriter(w)
	total, afterID, started := 0, int64(0), false
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		rows, lastID, err := spec.page(ctx, e.repo, afterID, status, e.batchSize)
		if err != nil {
			return total, fmt.Errorf("export %s after id %d: %w", entity, afterID, err)
		}

		if !started {
			if _, err := io.WriteString(w, utf8BOM); err != nil {
				return total, err
			}
			if err := cw.Write(spec.header); err != nil {
				return total, err
			}
			started = true
		}
		for _, row := range rows {
			safe := make([]string, len(row))
			for i, cell := range row {
				safe[i] = csvSafe(cell)
			}
			if err := cw.Write(safe); err != nil {
				return total, err
			}
		}
		total += len(rows)
		cw.Flush()
		if err := cw.Error(); err != nil {
			return total, err
		}
		if flush != nil {
			flush()
		}

		if int32(len(rows)) < e.batchSize {
			return total, nil
		}
		afterID = lastID
	}
}
