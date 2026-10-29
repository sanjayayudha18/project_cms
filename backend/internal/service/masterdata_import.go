package service

import (
	"context"
	"errors"
	"fmt"
	"io"
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
}

// importEnv is the database state one dry-run validates against.
type importEnv struct {
	entity    string
	cur       map[int64]map[string]string // current rows by id (all statuses)
	natKeys   map[string]int64            // natural key -> id
	vendors   map[string]int64            // active vendor code -> id
	branches  map[string]int64            // active "vendor|branch" -> id
	atms      map[string]int64            // active terminal id -> id
	packages  map[string]int64            // active "vendor|branch|package" -> id
	labels    map[string]bool             // vendor-wide package labels with an open tariff, "vendor|label"
	locations map[int64]bool
	spans     map[string][]importSpan // terminal -> active periods (db rows not in the file + accepted file rows)
	seenKey   map[string]int
	seenID    map[int64]int
}

// importNatKey is the natural key of a row, "" when the entity has none (PICs).
func importNatKey(entity string, v map[string]string) string {
	switch entity {
	case ExportVendors:
		return v["code"]
	case ExportVendorBranches:
		return v["vendor_code"] + "|" + v["branch_code"]
	case ExportVendorVaults:
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
	ExportVendorVaults:   {"vendor_code", "branch_code", "vault_code"},
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

func (i *MasterDataImporter) loadAll(ctx context.Context, entity string) ([]map[string]string, error) {
	spec := exportSpecs[entity]
	var out []map[string]string
	var after int64
	for {
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

func (i *MasterDataImporter) buildEnv(ctx context.Context, entity string) (*importEnv, error) {
	env := &importEnv{entity: entity, cur: map[int64]map[string]string{}, natKeys: map[string]int64{},
		spans: map[string][]importSpan{}, seenKey: map[string]int{}, seenID: map[int64]int{}}
	rows, err := i.loadAll(ctx, entity)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		id, _ := strconv.ParseInt(r["id"], 10, 64)
		env.cur[id] = r
		if k := importNatKey(entity, r); k != "" {
			env.natKeys[k] = id
		}
		if entity == ExportATMAssignments && r["is_active"] == "true" {
			env.spans[r["terminal_id"]] = append(env.spans[r["terminal_id"]], importSpan{start: r["effective_start_date"], end: r["effective_end_date"], id: id})
		}
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
	case ExportVendorVaults, ExportVendorPICs:
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
	res := &ImportResult{Preview: ImportPreview{Entity: entity, Errors: []ImportRowError{}}}

	recs, fileErrs, err := readImportCSV(r, spec.header, spec.legacyHeaders, spec.legacyFill)
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
		row, errs := env.validateRow(spec.header, rec)
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
	if len(rec.cells) != len(header) {
		add("row", fmt.Sprintf("jumlah kolom %d, seharusnya %d", len(rec.cells), len(header)))
		return ImportRow{}, errs
	}
	v := make(map[string]string, len(header))
	for c, h := range header {
		v[h] = rec.cells[c]
	}
	if e.entity == ExportATMAssignments {
		v["package_source"] = assignmentSourceCell(v)
	}

	id, ok := e.rowID(v["id"], rec.line, add)
	if !ok {
		return ImportRow{}, errs
	}
	creating := id == 0
	if e.entity == ExportVendorBranches {
		for _, col := range []string{"region_code", "category"} {
			if v[col] == keepCurrentCell {
				v[col] = e.cur[id][col] // older file without the column: keep the current value ("" when creating)
			}
		}
	}

	if e.entity == ExportVendorPICs && !creating && strings.TrimSpace(v["roles"]) == "" {
		v["roles"] = e.cur[id]["roles"] // blank or older file without the column: roles stay as they are
	}

	e.validateFields(v, creating, add, fromErr)
	if len(errs) > 0 {
		return ImportRow{}, errs
	}

	if !e.checkNatKey(header[1], v, id, rec.line, add) {
		return ImportRow{}, errs
	}
	toggle := false
	if !creating {
		if toggle, ok = e.checkUpdate(header, v, id, add); !ok {
			return ImportRow{}, errs
		}
	}
	if e.entity == ExportATMAssignments && v["is_active"] == "true" && !e.checkSpan(v, id, rec.line, add) {
		return ImportRow{}, errs
	}
	return ImportRow{Row: rec.line, Op: e.rowOp(header, v, id), ID: id, Cells: v, Toggle: toggle}, nil
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
	nk := importNatKey(e.entity, v)
	if nk == "" {
		return true
	}
	if first, dup := e.seenKey[nk]; dup {
		add(col, fmt.Sprintf("duplikat dengan baris %d", first))
		return false
	}
	e.seenKey[nk] = line
	owner, exists := e.natKeys[nk]
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
		if v[col] != cur[col] {
			add(col, fmt.Sprintf("tidak boleh diubah (nilai saat ini: %q)", cur[col]))
			ok = false
		}
	}
	if !ok {
		return false, false
	}
	changedOther := false
	for _, h := range header[1:] {
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
	for _, h := range header[1:] {
		if v[h] != e.cur[id][h] {
			return ImportOpUpdate
		}
	}
	return ImportOpUnchanged
}
