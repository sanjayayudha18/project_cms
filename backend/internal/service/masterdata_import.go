package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// Import limits (plan.md T5.5 owns tuning them; the guards live here so the
// parser can never read an unbounded upload).
const (
	MasterDataImportMaxBytes = 5 << 20 // 5 MiB
	MasterDataImportMaxRows  = 2000    // data rows, empty lines excluded
	masterDataImportMaxErrs  = 500     // errors reported; the rest are counted, not listed
)

// Import ops a valid row resolves to.
const (
	ImportOpCreate    = "create"
	ImportOpUpdate    = "update"
	ImportOpUnchanged = "unchanged"
)

// ImportRowError is one problem, addressed the way the admin sees it in the
// spreadsheet: Row is the file line (header = 1, first data row = 2).
type ImportRowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ImportRow is a row that passed validation. Cells hold the normalized values
// keyed by header name; T5.4 turns these into change requests.
type ImportRow struct {
	Row   int
	Op    string
	ID    int64 // 0 for create
	Cells map[string]string
	// Toggle marks an update whose only change is is_active (enable/disable).
	Toggle bool
}

// ImportPreview is the dry-run answer: nothing has been written.
type ImportPreview struct {
	Entity          string           `json:"entity"`
	TotalRows       int              `json:"total_rows"`
	ValidRows       int              `json:"valid_rows"`
	Creates         int              `json:"creates"`
	Updates         int              `json:"updates"`
	Unchanged       int              `json:"unchanged"`
	Errors          []ImportRowError `json:"errors"`
	ErrorsTruncated bool             `json:"errors_truncated"`
	// Updates == DataUpdates + Enables + Disables (Updates keeps counting toggles).
	DataUpdates int `json:"data_updates"`
	Enables     int `json:"enables"`
	Disables    int `json:"disables"`
	// Changes is one entry per create/update/toggle row; always non-nil, empty when Errors is not.
	Changes []ImportChangePreview `json:"changes"`
}

// Preview ops beyond ImportOpCreate / ImportOpUpdate.
const (
	ImportOpEnable  = "enable"
	ImportOpDisable = "disable"
)

// ImportChangePreview is what one valid row would do; Row is the file line (header = 1).
type ImportChangePreview struct {
	Row    int                 `json:"row"`
	Op     string              `json:"op"` // create | update | enable | disable
	Key    string              `json:"key"`
	Fields []ImportFieldChange `json:"fields"`
}

// ImportFieldChange is one CSV column that changes; Before is "" for a create.
type ImportFieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// importKeyColumns are the columns that identify a row to the admin in the preview.
var importKeyColumns = map[string][]string{
	ExportVendors:        {"name"},
	ExportVendorBranches: {"vendor_code", "branch_code"},
	ExportVendorVaults:   {"branch_name", "vault_name"},
	ExportVendorPICs:     {"vendor_code", "branch_code", "name"},
	ExportATMs:           {"terminal_id"},
	ExportATMAssignments: {"terminal_id", "effective_start_date"},
}

// buildChangePreview describes one validated row; nil for an unchanged row. It
// compares the normalized cells (row.Cells) with cur over importDataColumns, like rowOp
// and checkUpdate, so normalization never shows up as a change.
// keepCurrentCols are the columns an older header leaves as KeepCurrentCell (csvexport LegacyFill).
var keepCurrentCols = map[string][]string{
	ExportVendorBranches: {"region_code", "category"},
	ExportVendorVaults:   {"alamat", "cis_limit_amount", "escrow_cash", "escrow_atm", "escrow_account"},
}

// importDataColumns returns the columns of header that carry editable/comparable
// entity fields (skipping row identifiers "id", display counter "no", and derived
// read-only "region" on vendor-vaults).
func importDataColumns(entity string, header []string) []string {
	out := make([]string, 0, len(header))
	for _, h := range header {
		if h == "id" || h == "no" {
			continue
		}
		if entity == ExportVendorVaults && h == "region" {
			continue
		}
		out = append(out, h)
	}
	return out
}

func previewKeyColumns(entity string, header []string, cells map[string]string) []string {
	switch entity {
	case ExportVendors:
		if slices.Contains(header, "code") || (cells["name"] == "" && cells["code"] != "") {
			return []string{"code"}
		}
		return []string{"name"}
	case ExportVendorVaults:
		if slices.Contains(header, "vault_code") || (cells["branch_name"] == "" && cells["vault_name"] == "" && cells["vault_code"] != "") {
			return []string{"vendor_code", "branch_code", "vault_code"}
		}
		return []string{"branch_name", "vault_name"}
	default:
		return importKeyColumns[entity]
	}
}

func buildChangePreview(entity string, header []string, row ImportRow, cur map[string]string) *ImportChangePreview {
	if row.Op == ImportOpUnchanged {
		return nil
	}
	p := &ImportChangePreview{Row: row.Row, Op: row.Op, Fields: []ImportFieldChange{}}
	var keys []string
	for _, col := range previewKeyColumns(entity, header, row.Cells) {
		if v := row.Cells[col]; v != "" {
			keys = append(keys, v)
		}
	}
	p.Key = strings.Join(keys, " / ")
	cols := importDataColumns(entity, header)

	switch {
	case row.Op == ImportOpCreate:
		for _, h := range cols {
			if v := row.Cells[h]; v != "" {
				p.Fields = append(p.Fields, ImportFieldChange{Field: h, After: v})
			}
		}
	case row.Toggle:
		p.Op = ImportOpDisable
		if row.Cells["is_active"] == "true" {
			p.Op = ImportOpEnable
		}
		p.Fields = append(p.Fields, ImportFieldChange{Field: "is_active", Before: cur["is_active"], After: row.Cells["is_active"]})
	default:
		for _, h := range cols {
			if row.Cells[h] != cur[h] {
				p.Fields = append(p.Fields, ImportFieldChange{Field: h, Before: cur[h], After: row.Cells[h]})
			}
		}
	}
	return p
}

// ImportResult is the preview plus the validated rows (not serialized).
type ImportResult struct {
	Preview ImportPreview
	Rows    []ImportRow
	env     *importEnv // reference ids + current rows, for the confirm step
}

// MasterDataImportRepo is the read surface the dry-run needs: the export pages
// (current state) plus the FK reference sets.
type MasterDataImportRepo interface {
	MasterDataExportRepo
	PackageKeys(ctx context.Context) ([]db.ImportPackageKeysRow, error)
	PackageLabels(ctx context.Context) ([]db.ImportVendorPackageLabelsRow, error)
	LocationIDs(ctx context.Context) ([]int64, error)
}

// MasterDataImporter validates a CSV against the current master data without
// writing anything (T5.3). Build it on the PRIMARY pool: it validates a write
// flow, so replica lag must not produce false "not found" errors.
type MasterDataImporter struct {
	repo    MasterDataImportRepo
	confirm *ImportConfirmDeps // set by WithConfirm; nil = dry-run only
}

// NewMasterDataImporter creates a MasterDataImporter over repo.
func NewMasterDataImporter(repo MasterDataImportRepo) *MasterDataImporter {
	return &MasterDataImporter{repo: repo}
}

// authorizeMasterDataAdmin re-checks (RBAC at service layer too) that the caller
// holds a master-data admin role.
func authorizeMasterDataAdmin(ctx context.Context) error {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok || authCtx == nil || !masterDataMakerRoles[strings.ToUpper(strings.TrimSpace(authCtx.Role))] {
		return ErrMasterDataForbidden
	}
	return nil
}

// Authorize is the service-layer role check for import endpoints.
func (i *MasterDataImporter) Authorize(ctx context.Context) error {
	return authorizeMasterDataAdmin(ctx)
}

type importRecord struct {
	line  int
	cells []string
	// badCount > 0: the row's cell count did not match its file's header
	// (Export_Header rows); validateRow rejects it, expecting badCount cells.
	badCount int
}

// importEnv is the database state one dry-run validates against.
type importEnv struct {
	entity            string
	cur               map[int64]map[string]string // current rows by id (all statuses)
	natKeys           map[string]int64            // natural key -> id
	legacyVaultKeys   map[string]int64            // legacy vault_code -> id
	vendorsByName     map[string]int64            // vendor name -> id (all statuses, for 7-col vendor import)
	usedVendorCodes   map[string]bool             // vendor codes in DB or generated in this file
	vendors           map[string]int64            // active vendor code -> id
	branches          map[string]int64            // active "vendor|branch" -> id
	branchesByName    map[string]int64            // active branch_name -> id
	allBranchesByName map[string]int64            // all branch_name -> id
	branchVendorCode  map[int64]string            // branch id -> vendor_code
	branchCodeByID    map[int64]string            // branch id -> branch_code
	atms              map[string]int64            // active terminal id -> id
	packages          map[string]int64            // active "vendor|branch|package" -> id
	labels            map[string]bool             // vendor-wide package labels with an open tariff, "vendor|label"
	locations         map[int64]bool
	spans             map[string][]importSpan // terminal -> active periods (db rows not in the file + accepted file rows)
	seenKey           map[string]int
	seenID            map[int64]int
	// PIC directors: active director id per vendor code in the DB, and the file
	// line of the first active director row per vendor code (lazy, PIC import only).
	dbDirectors   map[string]int64
	fileDirectors map[string]int
}

// deriveVendorCode generates a unique vendor code from name when importing the
// 7-column vendors CSV (which omits id and code).
func (e *importEnv) deriveVendorCode(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() >= 32 {
				break
			}
		}
	}
	base := b.String()
	if base == "" {
		base = "VND"
	}
	if e.usedVendorCodes == nil {
		e.usedVendorCodes = map[string]bool{}
	}
	code := base
	for seq := 1; e.usedVendorCodes[code]; seq++ {
		code = fmt.Sprintf("%s%d", base, seq)
	}
	e.usedVendorCodes[code] = true
	return code
}

// importNatKey is the natural key of a row, "" when the entity has none (PICs).
func importNatKey(entity string, v map[string]string) string {
	switch entity {
	case ExportVendors:
		if v["code"] != "" {
			return v["code"]
		}
		return v["name"]
	case ExportVendorBranches:
		return v["vendor_code"] + "|" + v["branch_code"]
	case ExportVendorVaults:
		if v["branch_name"] != "" || v["vault_name"] != "" {
			if v["vault_name"] == "" {
				return ""
			}
			return v["branch_name"] + "|" + v["vault_name"]
		}
		return v["vault_code"]
	case ExportATMs:
		return v["terminal_id"]
	case ExportATMAssignments:
		return strings.Join([]string{v["terminal_id"], v["vendor_code"], v["branch_code"], assignmentSourceCell(v), v["package_code"], v["effective_start_date"]}, "|")
	}
	return ""
}

// importImmutable lists the columns an update may not change.
var importImmutable = map[string][]string{
	ExportVendors:        {"code"},
	ExportVendorBranches: {"vendor_code", "branch_code"},
	ExportVendorVaults:   {"branch_name", "vendor_code", "branch_code", "vault_code"},
	ExportVendorPICs:     {"vendor_code"},
	ExportATMs:           {"terminal_id"},
	ExportATMAssignments: {"terminal_id", "package_source"},
}

// assignmentSourceCell is the package_source of a kelolaan row; blank (a file
// that predates the column) means the branch package.
func assignmentSourceCell(v map[string]string) string {
	if v["package_source"] == "" {
		return AssignmentSourceBranch
	}
	return v["package_source"]
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func (i *MasterDataImporter) loadAll(ctx context.Context, entity string) ([]map[string]string, error) {
	spec := exportSpecs[entity]
	var out []map[string]string
	var after int64
	for {
		switch entity {
		case ExportVendors:
			rows, err := i.repo.Vendors(ctx, after, "all", masterDataExportBatchSize)
			if err != nil {
				return nil, fmt.Errorf("loading %s for import: %w", entity, err)
			}
			for _, x := range rows {
				cells := csvexport.VendorCells(vendorExportRow(x))
				m := make(map[string]string, len(spec.header)+2)
				for c, h := range spec.header {
					m[h] = cells[c]
				}
				m["id"] = strconv.FormatInt(x.ID, 10)
				m["code"] = x.Code
				out = append(out, m)
				after = x.ID
			}
			if int32(len(rows)) < masterDataExportBatchSize { //nolint:gosec // G115: len(rows) <= batch size (SQL LIMIT)
				return out, nil
			}
		case ExportVendorVaults:
			rows, err := i.repo.VendorVaults(ctx, after, "all", masterDataExportBatchSize)
			if err != nil {
				return nil, fmt.Errorf("loading %s for import: %w", entity, err)
			}
			for idx, x := range rows {
				cells := csvexport.VendorVaultCells(vendorVaultExportRow(int64(len(out)+idx+1), x))
				m := make(map[string]string, len(spec.header)+10)
				for c, h := range spec.header {
					m[h] = cells[c]
				}
				m["id"] = strconv.FormatInt(x.ID, 10)
				m["vendor_code"] = x.VendorCode
				m["branch_code"] = x.BranchCode
				m["vault_code"] = x.VaultCode
				m["min_capacity_amount"] = x.MinBalance
				m["max_capacity_amount"] = x.MaxBalance
				m["latitude"] = x.Latitude
				m["longitude"] = x.Longitude
				m["location_id"] = x.LocationID
				m["escrow_account"] = firstNonEmpty(x.EscrowCash, x.EscrowAtm)
				out = append(out, m)
				after = x.ID
			}
			if int32(len(rows)) < masterDataExportBatchSize { //nolint:gosec // G115: len(rows) <= batch size (SQL LIMIT)
				return out, nil
			}
		default:
			rows, last, err := spec.page(ctx, i.repo, after, "all", masterDataExportBatchSize)
			if err != nil {
				return nil, fmt.Errorf("loading %s for import: %w", entity, err)
			}
			for _, r := range rows {
				m := make(map[string]string, len(spec.header))
				for c, h := range spec.header {
					m[h] = r[c]
				}
				if entity == ExportVendorPICs {
					m["roles"] = canonicalPicRolesCell(m["roles"])
				}
				out = append(out, m)
			}
			if int32(len(rows)) < masterDataExportBatchSize { //nolint:gosec // G115: len(rows) <= batch size (SQL LIMIT)
				return out, nil
			}
			after = last
		}
	}
}

// canonicalPicRolesCell sorts a stored roles cell the way the file side is
// normalized (validatePicRow -> normalizePicRoles), so a PIC whose roles are
// stored unsorted (migration 032 default {replenish,vault,cashcount}) compares
// unchanged against its own unedited export. The export bytes keep the DB order.
func canonicalPicRolesCell(cell string) string {
	roles, err := parsePicRoles(cell)
	if err != nil || roles == nil {
		return cell
	}
	return csvexport.PICRolesCell(*roles)
}

// activeSet loads entity and returns the set of key(row) for its active rows.
func (i *MasterDataImporter) activeSet(ctx context.Context, entity string, key func(map[string]string) string) (map[string]int64, error) {
	rows, err := i.loadAll(ctx, entity)
	if err != nil {
		return nil, err
	}
	set := make(map[string]int64, len(rows))
	for _, r := range rows {
		if r["is_active"] == "true" {
			id, _ := strconv.ParseInt(r["id"], 10, 64)
			set[key(r)] = id
		}
	}
	return set, nil
}

func (e *importEnv) indexCurrentRow(id int64, r map[string]string) {
	e.cur[id] = r
	switch e.entity {
	case ExportVendors:
		if r["code"] != "" {
			e.natKeys[r["code"]] = id
			e.usedVendorCodes[r["code"]] = true
		}
		if r["name"] != "" {
			e.vendorsByName[r["name"]] = id
		}
	case ExportVendorVaults:
		if r["branch_name"] != "" && r["vault_name"] != "" {
			e.natKeys[r["branch_name"]+"|"+r["vault_name"]] = id
		}
		if r["vault_code"] != "" {
			e.legacyVaultKeys[r["vault_code"]] = id
		}
	default:
		if k := importNatKey(e.entity, r); k != "" {
			e.natKeys[k] = id
		}
	}
	if e.entity == ExportATMAssignments && r["is_active"] == "true" {
		e.spans[r["terminal_id"]] = append(e.spans[r["terminal_id"]], importSpan{start: r["effective_start_date"], end: r["effective_end_date"], id: id})
	}
}

func (i *MasterDataImporter) loadVaultBranches(ctx context.Context, env *importEnv) error {
	brows, err := i.loadAll(ctx, ExportVendorBranches)
	if err != nil {
		return err
	}
	env.branches = make(map[string]int64, len(brows))
	for _, br := range brows {
		bid, _ := strconv.ParseInt(br["id"], 10, 64)
		if br["branch_name"] != "" {
			env.allBranchesByName[br["branch_name"]] = bid
		}
		env.branchVendorCode[bid] = br["vendor_code"]
		env.branchCodeByID[bid] = br["branch_code"]
		if br["is_active"] == "true" {
			env.branches[br["vendor_code"]+"|"+br["branch_code"]] = bid
			if br["branch_name"] != "" {
				env.branchesByName[br["branch_name"]] = bid
			}
		}
	}
	return nil
}

func (i *MasterDataImporter) buildEnv(ctx context.Context, entity string) (*importEnv, error) {
	env := &importEnv{
		entity:            entity,
		cur:               map[int64]map[string]string{},
		natKeys:           map[string]int64{},
		legacyVaultKeys:   map[string]int64{},
		vendorsByName:     map[string]int64{},
		usedVendorCodes:   map[string]bool{},
		branchesByName:    map[string]int64{},
		allBranchesByName: map[string]int64{},
		branchVendorCode:  map[int64]string{},
		branchCodeByID:    map[int64]string{},
		spans:             map[string][]importSpan{},
		seenKey:           map[string]int{},
		seenID:            map[int64]int{},
	}
	rows, err := i.loadAll(ctx, entity)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		id, _ := strconv.ParseInt(r["id"], 10, 64)
		env.indexCurrentRow(id, r)
	}

	vendorKey := func(r map[string]string) string { return r["code"] }
	branchKey := func(r map[string]string) string { return r["vendor_code"] + "|" + r["branch_code"] }
	switch entity {
	case ExportVendorBranches, ExportVendorVaults, ExportVendorPICs:
		if env.vendors, err = i.activeSet(ctx, ExportVendors, vendorKey); err != nil {
			return nil, err
		}
	}
	switch entity {
	case ExportVendorVaults:
		if err := i.loadVaultBranches(ctx, env); err != nil {
			return nil, err
		}
	case ExportVendorPICs:
		if env.branches, err = i.activeSet(ctx, ExportVendorBranches, branchKey); err != nil {
			return nil, err
		}
	}
	switch entity {
	case ExportVendorBranches, ExportVendorVaults, ExportATMs:
		ids, err := i.repo.LocationIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading locations for import: %w", err)
		}
		env.locations = make(map[int64]bool, len(ids))
		for _, id := range ids {
			env.locations[id] = true
		}
	case ExportATMAssignments:
		if env.atms, err = i.activeSet(ctx, ExportATMs, func(r map[string]string) string { return r["terminal_id"] }); err != nil {
			return nil, err
		}
		if env.vendors, err = i.activeSet(ctx, ExportVendors, vendorKey); err != nil {
			return nil, err
		}
		if env.branches, err = i.activeSet(ctx, ExportVendorBranches, branchKey); err != nil {
			return nil, err
		}
		if err := i.loadPackages(ctx, env); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// loadPackages fills the kelolaan package lookups: vendor-wide tariff labels
// and branch special packages.
func (i *MasterDataImporter) loadPackages(ctx context.Context, env *importEnv) error {
	labels, err := i.repo.PackageLabels(ctx)
	if err != nil {
		return fmt.Errorf("loading package labels for import: %w", err)
	}
	env.labels = make(map[string]bool, len(labels))
	for _, l := range labels {
		env.labels[l.VendorCode+"|"+l.Package] = true
	}
	keys, err := i.repo.PackageKeys(ctx)
	if err != nil {
		return fmt.Errorf("loading packages for import: %w", err)
	}
	env.packages = make(map[string]int64, len(keys))
	for _, k := range keys {
		env.packages[k.VendorCode+"|"+k.BranchCode+"|"+k.PackageCode] = k.ID
	}
	return nil
}

// peekCSVHeader extracts the first CSV record to detect whether a file uses a
// legacy schema that has columns removed/renamed in migration 040.
func peekCSVHeader(data []byte) []string {
	trimmed := bytes.TrimPrefix(data, []byte(utf8BOM))
	firstLine := trimmed
	if n := bytes.IndexByte(trimmed, '\n'); n >= 0 {
		firstLine = trimmed[:n]
	}
	comma := ','
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		comma = ';'
	}
	cr := csv.NewReader(bytes.NewReader(trimmed))
	cr.Comma = comma
	cr.FieldsPerRecord = -1
	rec, err := cr.Read()
	if err != nil {
		return nil
	}
	return rec
}

func parseEntityImportCSV(entity string, spec exportSpec, r io.Reader) (activeHeader []string, recs []importRecord, errs []ImportRowError, err error) {
	data, err := io.ReadAll(io.LimitReader(r, MasterDataImportMaxBytes+1))
	if err != nil {
		return nil, nil, nil, err
	}
	if len(data) > MasterDataImportMaxBytes {
		return nil, nil, nil, &ValidationError{Field: "file", Message: fmt.Sprintf("ukuran file melebihi %d MB", MasterDataImportMaxBytes>>20)}
	}
	firstRec := peekCSVHeader(data)
	switch entity {
	case ExportVendors:
		if len(spec.legacyHeaders) >= 2 && (headerMatches(firstRec, spec.legacyHeaders[0]) || headerMatches(firstRec, spec.legacyHeaders[1])) {
			recs, errs, err = readImportCSV(bytes.NewReader(data), spec.legacyHeaders[0], nil, nil, csvexport.VendorReadOnlyColumns())
			return spec.legacyHeaders[0], recs, errs, err
		}
	case ExportVendorVaults:
		if len(spec.legacyHeaders) >= 2 && (headerMatches(firstRec, spec.legacyHeaders[0]) || headerMatches(firstRec, spec.legacyHeaders[1])) {
			recs, errs, err = readImportCSV(
				bytes.NewReader(data),
				spec.legacyHeaders[0],
				[][]string{spec.legacyHeaders[1]},
				map[string]string{"escrow_account": keepCurrentCell},
				nil,
			)
			return spec.legacyHeaders[0], recs, errs, err
		}
	}
	recs, errs, err = readImportCSV(bytes.NewReader(data), spec.header, spec.legacyHeaders, spec.legacyFill, spec.readOnly)
	if len(errs) == 1 && errs[0].Field == "header" && len(spec.legacyHeaders) > 0 && strings.Contains(errs[0].Message, "kolom header tidak sesuai") {
		errs[0].Message += " (atau legacy: " + strings.Join(spec.legacyHeaders[0], ",") + ")"
	}
	return spec.header, recs, errs, err
}

// DryRun validates the CSV read from r against current master data and returns
// what an import would do. It never writes. Problems with the data come back in
// Preview.Errors; only infrastructure failures and oversize files are Go errors.
func (i *MasterDataImporter) DryRun(ctx context.Context, entity string, r io.Reader) (*ImportResult, error) {
	if err := i.Authorize(ctx); err != nil {
		return nil, err
	}
	spec, ok := exportSpecs[entity]
	if !ok {
		return nil, ErrExportUnknownEntity
	}
	res := &ImportResult{Preview: ImportPreview{Entity: entity, Errors: []ImportRowError{}, Changes: []ImportChangePreview{}}}

	activeHeader, recs, fileErrs, err := parseEntityImportCSV(entity, spec, r)
	if err != nil {
		return nil, err
	}
	if len(fileErrs) > 0 {
		res.Preview.Errors = fileErrs
		return res, nil
	}
	res.Preview.TotalRows = len(recs)

	env, err := i.buildEnv(ctx, entity)
	if err != nil {
		return nil, err
	}
	res.env = env
	// Rows of the file replace their database versions for the overlap check.
	if entity == ExportATMAssignments {
		inFile := map[int64]bool{}
		for _, rec := range recs {
			if id, err := strconv.ParseInt(rec.cells[0], 10, 64); err == nil {
				inFile[id] = true
			}
		}
		for term, ss := range env.spans {
			kept := ss[:0]
			for _, s := range ss {
				if !inFile[s.id] {
					kept = append(kept, s)
				}
			}
			env.spans[term] = kept
		}
	}

	dropped := 0
	for _, rec := range recs {
		row, errs := env.validateRow(activeHeader, rec)
		if len(errs) == 0 {
			res.Rows = append(res.Rows, row)
			continue
		}
		for _, e := range errs {
			if len(res.Preview.Errors) < masterDataImportMaxErrs {
				res.Preview.Errors = append(res.Preview.Errors, e)
			} else {
				dropped++
			}
		}
	}
	res.Preview.ErrorsTruncated = dropped > 0
	res.Preview.ValidRows = len(res.Rows)
	for _, row := range res.Rows {
		switch row.Op {
		case ImportOpCreate:
			res.Preview.Creates++
		case ImportOpUpdate:
			res.Preview.Updates++
		default:
			res.Preview.Unchanged++
		}
		ch := buildChangePreview(entity, activeHeader, row, env.cur[row.ID])
		if ch == nil {
			continue
		}
		switch ch.Op {
		case ImportOpEnable:
			res.Preview.Enables++
		case ImportOpDisable:
			res.Preview.Disables++
		case ImportOpUpdate:
			res.Preview.DataUpdates++
		}
		if len(res.Preview.Errors) == 0 {
			res.Preview.Changes = append(res.Preview.Changes, *ch)
		}
	}
	return res, nil
}

// validateRow validates one record: per-field rules (reusing the same pure
// validators the single-record services use), foreign keys, duplicates within
// the file, and -- for kelolaan -- period overlap. On success it resolves the op.
func (e *importEnv) validateRow(header []string, rec importRecord) (ImportRow, []ImportRowError) {
	var errs []ImportRowError
	add := func(field, msg string) {
		errs = append(errs, ImportRowError{Row: rec.line, Field: field, Message: msg})
	}
	fromErr := func(err error) {
		var ve *ValidationError
		if errors.As(err, &ve) {
			add(ve.Field, ve.Message)
			return
		}
		add("row", err.Error())
	}
	if rec.badCount > 0 {
		add("row", fmt.Sprintf("jumlah kolom %d, seharusnya %d", len(rec.cells), rec.badCount))
		return ImportRow{}, errs
	}
	if len(rec.cells) != len(header) {
		add("row", fmt.Sprintf("jumlah kolom %d, seharusnya %d", len(rec.cells), len(header)))
		return ImportRow{}, errs
	}
	v := make(map[string]string, len(header)+6)
	for c, h := range header {
		v[h] = rec.cells[c]
	}
	if e.entity == ExportATMAssignments {
		v["package_source"] = assignmentSourceCell(v)
	}

	hasIDCol := len(header) > 0 && header[0] == "id"
	var id int64
	if hasIDCol {
		var ok bool
		id, ok = e.rowID(v["id"], rec.line, add)
		if !ok {
			return ImportRow{}, errs
		}
	} else {
		var ok bool
		id, ok = e.resolveRowByNaturalKey(v, rec.line, add)
		if !ok {
			return ImportRow{}, errs
		}
	}
	creating := id == 0
	for _, col := range keepCurrentCols[e.entity] {
		if v[col] == keepCurrentCell {
			v[col] = e.cur[id][col] // older file without the column: keep the current value ("" when creating)
		}
	}
	if hasIDCol && !creating && e.entity == ExportVendorVaults {
		cur := e.cur[id]
		for _, col := range []string{"branch_name", "vault_name", "alamat", "cis_limit_amount", "escrow_cash", "escrow_atm"} {
			if _, ok := v[col]; !ok {
				v[col] = cur[col]
			}
		}
	}

	if e.entity == ExportVendorPICs {
		e.resolvePICKeeps(v, id, creating)
	}

	e.validateFields(v, creating, hasIDCol, add, fromErr)
	if len(errs) > 0 {
		return ImportRow{}, errs
	}
	if e.entity == ExportVendorPICs {
		e.checkPICDirector(v, id, rec.line, add)
		if len(errs) > 0 {
			return ImportRow{}, errs
		}
	}

	if hasIDCol {
		natCol := header[1]
		if e.entity == ExportVendorVaults {
			natCol = "vault_code"
		}
		if !e.checkNatKey(natCol, v, id, rec.line, add) {
			return ImportRow{}, errs
		}
	}
	toggle := false
	if !creating {
		var ok bool
		if toggle, ok = e.checkUpdate(header, v, id, add); !ok {
			return ImportRow{}, errs
		}
	}
	if e.entity == ExportATMAssignments && v["is_active"] == "true" && !e.checkSpan(v, id, rec.line, add) {
		return ImportRow{}, errs
	}
	return ImportRow{Row: rec.line, Op: e.rowOp(header, v, id), ID: id, Cells: v, Toggle: toggle}, nil
}

// resolveRowByNaturalKey resolves the target row ID for schemas that do not carry
// an explicit "id" column (revamped 7-column vendors and 14-column vendor-vaults).
func (e *importEnv) resolveRowByNaturalKey(v map[string]string, line int, add func(string, string)) (int64, bool) {
	switch e.entity {
	case ExportVendors:
		name := strings.TrimSpace(v["name"])
		if name == "" {
			return 0, true
		}
		if first, dup := e.seenKey[name]; dup {
			add("name", fmt.Sprintf("duplikat dengan baris %d", first))
			return 0, false
		}
		e.seenKey[name] = line
		if existingID, ok := e.vendorsByName[name]; ok {
			v["id"] = strconv.FormatInt(existingID, 10)
			v["code"] = e.cur[existingID]["code"]
			return existingID, true
		}
		v["code"] = e.deriveVendorCode(name)
		return 0, true
	case ExportVendorVaults:
		branchName := strings.TrimSpace(v["branch_name"])
		vaultName := strings.TrimSpace(v["vault_name"])
		if branchName == "" || vaultName == "" {
			return 0, true
		}
		nk := branchName + "|" + vaultName
		if first, dup := e.seenKey[nk]; dup {
			add("vault_name", fmt.Sprintf("duplikat dengan baris %d", first))
			return 0, false
		}
		e.seenKey[nk] = line
		if existingID, ok := e.natKeys[nk]; ok {
			cur := e.cur[existingID]
			v["id"] = strconv.FormatInt(existingID, 10)
			v["vendor_code"] = cur["vendor_code"]
			v["branch_code"] = cur["branch_code"]
			v["vault_code"] = cur["vault_code"]
			v["location_id"] = cur["location_id"]
			v["latitude"] = cur["latitude"]
			v["longitude"] = cur["longitude"]
			return existingID, true
		}
		return 0, true
	}
	return 0, true
}

// rowID parses and checks the id cell; blank means create (0).
func (e *importEnv) rowID(raw string, line int, add func(string, string)) (int64, bool) {
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		add("id", "harus bilangan bulat positif, atau kosong untuk data baru")
		return 0, false
	}
	if _, ok := e.cur[id]; !ok {
		add("id", "id tidak ditemukan")
		return 0, false
	}
	if first, dup := e.seenID[id]; dup {
		add("id", fmt.Sprintf("duplikat dengan baris %d", first))
		return 0, false
	}
	e.seenID[id] = line
	return id, true
}

// checkNatKey rejects a natural key repeated in the file or owned by another row.
func (e *importEnv) checkNatKey(col string, v map[string]string, id int64, line int, add func(string, string)) bool {
	var nk string
	natMap := e.natKeys
	if e.entity == ExportVendorVaults && v["vault_code"] != "" && v["branch_name"] == "" {
		nk = v["vault_code"]
		natMap = e.legacyVaultKeys
	} else {
		nk = importNatKey(e.entity, v)
	}
	if nk == "" {
		return true
	}
	if first, dup := e.seenKey[nk]; dup {
		add(col, fmt.Sprintf("duplikat dengan baris %d", first))
		return false
	}
	e.seenKey[nk] = line
	owner, exists := natMap[nk]
	switch {
	case !exists || owner == id:
		return true
	case id == 0:
		add(col, fmt.Sprintf("sudah ada (id %d); isi kolom id untuk mengubahnya", owner))
	default:
		add(col, fmt.Sprintf("bentrok dengan data lain (id %d)", owner))
	}
	return false
}

// checkUpdate enforces the update rules for an existing row: immutable columns,
// and data vs is_active changes never mixed nor applied to an inactive row.
func (e *importEnv) checkUpdate(header []string, v map[string]string, id int64, add func(string, string)) (toggle, ok bool) {
	cur := e.cur[id]
	ok = true
	for _, col := range importImmutable[e.entity] {
		if !slices.Contains(header, col) {
			continue
		}
		if v[col] != cur[col] {
			add(col, fmt.Sprintf("tidak boleh diubah (nilai saat ini: %q)", cur[col]))
			ok = false
		}
	}
	if !ok {
		return false, false
	}
	changedOther := false
	for _, h := range importDataColumns(e.entity, header) {
		if h != "is_active" && v[h] != cur[h] {
			changedOther = true
		}
	}
	toggle = v["is_active"] != cur["is_active"]
	switch {
	case toggle && changedOther:
		add("is_active", "data dan status aktif tidak bisa diubah dalam satu baris; pisahkan menjadi dua impor")
		return toggle, false
	case changedOther && cur["is_active"] == "false":
		add("is_active", "data nonaktif tidak bisa diubah; aktifkan dulu lewat impor terpisah")
		return toggle, false
	}
	return toggle, true
}

// checkSpan rejects an active kelolaan overlapping another one of the same ATM
// (current or earlier in the file), then records its span.
func (e *importEnv) checkSpan(v map[string]string, id int64, line int, add func(string, string)) bool {
	s := importSpan{start: v["effective_start_date"], end: v["effective_end_date"], id: id, row: line}
	for _, other := range e.spans[v["terminal_id"]] {
		if spansOverlap(s, other) {
			where := fmt.Sprintf("id %d", other.id)
			if other.row > 0 {
				where = fmt.Sprintf("baris %d", other.row)
			}
			add("effective_start_date", "periode tumpang tindih dengan kelolaan aktif lain ("+where+")")
			return false
		}
	}
	e.spans[v["terminal_id"]] = append(e.spans[v["terminal_id"]], s)
	return true
}

func (e *importEnv) rowOp(header []string, v map[string]string, id int64) string {
	if id == 0 {
		return ImportOpCreate
	}
	for _, h := range importDataColumns(e.entity, header) {
		if v[h] != e.cur[id][h] {
			return ImportOpUpdate
		}
	}
	return ImportOpUnchanged
}
