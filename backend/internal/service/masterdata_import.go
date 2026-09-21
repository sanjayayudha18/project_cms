package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
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

// cleanCell undoes the export's formula-injection guard (strips ONE leading
// apostrophe that precedes = @ + - TAB CR) and trims spaces.
func cleanCell(s string) string {
	if len(s) >= 2 && s[0] == '\'' && strings.ContainsRune("=@+-\t\r", rune(s[1])) {
		s = s[1:]
	}
	return strings.TrimSpace(s)
}

// readImportCSV parses the upload: size guard, BOM strip, `,` or `;` delimiter
// (whichever the header line uses -- Excel in an Indonesian locale writes `;`),
// header validation, empty-row skipping. A non-empty errs means the file is
// unusable as a whole (bad CSV / wrong header) and no row was processed.
func readImportCSV(r io.Reader, header []string) (recs []importRecord, errs []ImportRowError, err error) {
	data, err := io.ReadAll(io.LimitReader(r, MasterDataImportMaxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > MasterDataImportMaxBytes {
		return nil, nil, &ValidationError{Field: "file", Message: fmt.Sprintf("ukuran file melebihi %d MB", MasterDataImportMaxBytes>>20)}
	}
	data = bytes.TrimPrefix(data, []byte(utf8BOM))

	firstLine := data
	if n := bytes.IndexByte(data, '\n'); n >= 0 {
		firstLine = data[:n]
	}
	comma := ','
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		comma = ';'
	}
	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = comma
	cr.FieldsPerRecord = -1

	gotHeader := false
	for {
		rec, rerr := cr.Read()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			line := 0
			var pe *csv.ParseError
			if errors.As(rerr, &pe) {
				line = pe.Line
			}
			return nil, []ImportRowError{{Row: line, Field: "file", Message: "format CSV tidak valid: " + rerr.Error()}}, nil
		}
		line, _ := cr.FieldPos(0)
		if !gotHeader {
			gotHeader = true
			if !headerMatches(rec, header) {
				return nil, []ImportRowError{{Row: 1, Field: "header", Message: "kolom header tidak sesuai; diharapkan persis: " + strings.Join(header, ",")}}, nil
			}
			continue
		}
		cells := make([]string, len(rec))
		empty := true
		for i, c := range rec {
			cells[i] = cleanCell(c)
			if cells[i] != "" {
				empty = false
			}
		}
		if empty {
			continue
		}
		if len(recs) >= MasterDataImportMaxRows {
			return nil, nil, &ValidationError{Field: "file", Message: fmt.Sprintf("jumlah baris melebihi %d", MasterDataImportMaxRows)}
		}
		recs = append(recs, importRecord{line: line, cells: cells})
	}
	if !gotHeader {
		return nil, []ImportRowError{{Row: 1, Field: "header", Message: "file kosong"}}, nil
	}
	return recs, nil, nil
}

func headerMatches(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if strings.ToLower(strings.TrimSpace(got[i])) != want[i] {
			return false
		}
	}
	return true
}

// importSpan is one active kelolaan period of an ATM ("" end = open-ended).
type importSpan struct {
	start, end string
	id         int64
	row        int
}

func spansOverlap(a, b importSpan) bool {
	return (b.end == "" || a.start <= b.end) && (a.end == "" || b.start <= a.end)
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
		return strings.Join([]string{v["terminal_id"], v["vendor_code"], v["branch_code"], v["package_code"], v["effective_start_date"]}, "|")
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
	ExportATMAssignments: {"terminal_id"},
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
			out = append(out, m)
		}
		if int32(len(rows)) < masterDataExportBatchSize {
			return out, nil
		}
		after = last
	}
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
		keys, err := i.repo.PackageKeys(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading packages for import: %w", err)
		}
		env.packages = make(map[string]int64, len(keys))
		for _, k := range keys {
			env.packages[k.VendorCode+"|"+k.BranchCode+"|"+k.PackageCode] = k.ID
		}
	}
	return env, nil
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

	recs, fileErrs, err := readImportCSV(r, spec.header)
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

	var id int64
	if v["id"] != "" {
		n, err := strconv.ParseInt(v["id"], 10, 64)
		if err != nil || n <= 0 {
			add("id", "harus bilangan bulat positif, atau kosong untuk data baru")
			return ImportRow{}, errs
		}
		id = n
		if _, ok := e.cur[id]; !ok {
			add("id", "id tidak ditemukan")
			return ImportRow{}, errs
		}
		if first, dup := e.seenID[id]; dup {
			add("id", fmt.Sprintf("duplikat dengan baris %d", first))
			return ImportRow{}, errs
		}
		e.seenID[id] = rec.line
	}
	creating := id == 0

	e.validateFields(v, creating, add, fromErr)
	if len(errs) > 0 {
		return ImportRow{}, errs
	}

	nk := importNatKey(e.entity, v)
	if nk != "" {
		if first, dup := e.seenKey[nk]; dup {
			add(header[1], fmt.Sprintf("duplikat dengan baris %d", first))
			return ImportRow{}, errs
		}
		e.seenKey[nk] = rec.line
		if owner, exists := e.natKeys[nk]; exists && owner != id {
			if creating {
				add(header[1], fmt.Sprintf("sudah ada (id %d); isi kolom id untuk mengubahnya", owner))
			} else {
				add(header[1], fmt.Sprintf("bentrok dengan data lain (id %d)", owner))
			}
			return ImportRow{}, errs
		}
	}
	if !creating {
		for _, col := range importImmutable[e.entity] {
			if v[col] != e.cur[id][col] {
				add(col, fmt.Sprintf("tidak boleh diubah (nilai saat ini: %q)", e.cur[id][col]))
			}
		}
		if len(errs) > 0 {
			return ImportRow{}, errs
		}
	}
	toggle := false
	if !creating {
		changedOther := false
		for _, h := range header[1:] {
			if h != "is_active" && v[h] != e.cur[id][h] {
				changedOther = true
			}
		}
		toggle = v["is_active"] != e.cur[id]["is_active"]
		switch {
		case toggle && changedOther:
			add("is_active", "data dan status aktif tidak bisa diubah dalam satu baris; pisahkan menjadi dua impor")
		case changedOther && e.cur[id]["is_active"] == "false":
			add("is_active", "data nonaktif tidak bisa diubah; aktifkan dulu lewat impor terpisah")
		}
		if len(errs) > 0 {
			return ImportRow{}, errs
		}
	}
	if e.entity == ExportATMAssignments && v["is_active"] == "true" {
		s := importSpan{start: v["effective_start_date"], end: v["effective_end_date"], id: id, row: rec.line}
		for _, other := range e.spans[v["terminal_id"]] {
			if spansOverlap(s, other) {
				where := fmt.Sprintf("id %d", other.id)
				if other.row > 0 {
					where = fmt.Sprintf("baris %d", other.row)
				}
				add("effective_start_date", "periode tumpang tindih dengan kelolaan aktif lain ("+where+")")
				return ImportRow{}, errs
			}
		}
		e.spans[v["terminal_id"]] = append(e.spans[v["terminal_id"]], s)
	}

	op := ImportOpCreate
	if !creating {
		op = ImportOpUnchanged
		for _, h := range header[1:] {
			if v[h] != e.cur[id][h] {
				op = ImportOpUpdate
				break
			}
		}
	}
	return ImportRow{Row: rec.line, Op: op, ID: id, Cells: v, Toggle: toggle}, nil
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// boolCol normalizes a true/false column in place. Blank takes def when
// creating and is an error when updating (an update must say what it means).
func boolCol(v map[string]string, col, def string, creating bool, add func(string, string)) {
	switch s := strings.ToLower(v[col]); s {
	case "true", "false":
		v[col] = s
	case "":
		if creating {
			v[col] = def
		} else {
			add(col, "wajib true atau false")
		}
	default:
		add(col, "harus true atau false")
	}
}

func requiredCols(v map[string]string, add func(string, string), cols ...string) {
	for _, c := range cols {
		if v[c] == "" {
			add(c, "wajib diisi")
		}
	}
}

func (e *importEnv) location(v map[string]string, add func(string, string), required bool) {
	if v["location_id"] == "" {
		if required {
			add("location_id", "wajib diisi")
		}
		return
	}
	n, err := strconv.ParseInt(v["location_id"], 10, 64)
	if err != nil || n <= 0 {
		add("location_id", "harus bilangan bulat positif")
		return
	}
	if !e.locations[n] {
		add("location_id", "lokasi tidak ditemukan")
	}
}

func (e *importEnv) validateFields(v map[string]string, creating bool, add func(string, string), fromErr func(error)) {
	boolCol(v, "is_active", "true", creating, add)
	if creating && v["is_active"] == "false" {
		add("is_active", "data baru harus true (nonaktifkan lewat perubahan terpisah)")
	}
	switch e.entity {
	case ExportVendors:
		requiredCols(v, add, "code", "name")
		if v["contact_email"] != "" && !isValidEmail(v["contact_email"]) {
			add("contact_email", "format email tidak valid")
		}
		if n, err := normalizeNPWP(v["npwp"]); err != nil {
			fromErr(err)
		} else {
			v["npwp"] = n
		}
	case ExportVendorBranches:
		requiredCols(v, add, "vendor_code", "branch_code", "branch_name")
		if v["vendor_code"] != "" && e.vendors[v["vendor_code"]] == 0 {
			add("vendor_code", "vendor tidak ditemukan atau nonaktif")
		}
		e.location(v, add, false)
	case ExportVendorVaults:
		requiredCols(v, add, "vendor_code", "branch_code", "vault_code")
		if v["vendor_code"] != "" && v["branch_code"] != "" && e.branches[v["vendor_code"]+"|"+v["branch_code"]] == 0 {
			add("branch_code", "cabang tidak ditemukan atau nonaktif")
		}
		p := VendorVaultUpdatePayload{Category: v["category"], CurrencyCode: v["currency_code"],
			MinCapacityAmount: optStr(v["min_capacity_amount"]), MaxCapacityAmount: optStr(v["max_capacity_amount"]),
			Latitude: optStr(v["latitude"]), Longitude: optStr(v["longitude"])}
		if err := validateVaultFields(&p); err != nil {
			fromErr(err)
		} else {
			v["category"], v["currency_code"] = p.Category, p.CurrencyCode
		}
		e.location(v, add, false)
	case ExportVendorPICs:
		requiredCols(v, add, "vendor_code", "name")
		if v["vendor_code"] != "" && e.vendors[v["vendor_code"]] == 0 {
			add("vendor_code", "vendor tidak ditemukan atau nonaktif")
		}
		if v["branch_code"] != "" && e.branches[v["vendor_code"]+"|"+v["branch_code"]] == 0 {
			add("branch_code", "cabang tidak ditemukan atau nonaktif untuk vendor ini")
		}
		// Same rules as VendorPicAdminService.validate.
		if v["email"] != "" && (!isValidEmail(v["email"]) || strings.ContainsAny(v["email"], "<> ")) {
			add("email", "format email tidak valid")
		}
		if v["phone"] != "" && !phoneRe.MatchString(strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(v["phone"])) {
			add("phone", "format telepon tidak valid (6-15 digit, boleh diawali +)")
		}
		boolCol(v, "is_notification_recipient", "false", creating, add)
	case ExportATMs:
		requiredCols(v, add, "terminal_id")
		e.location(v, add, true)
		locID, _ := strconv.ParseInt(v["location_id"], 10, 64)
		if _, _, _, _, _, err := validateATMEditableFields(locID, v["machine_type"], v["brand"], v["model"], v["operation_hours"], v["deployment_type"],
			optStr(v["priority_class"]), optStr(v["capacity_amount"]), optStr(v["low_threshold_amount"]), optStr(v["critical_threshold_amount"])); err != nil {
			fromErr(err)
		}
		boolCol(v, "blacklisted", "false", creating, add)
	case ExportATMAssignments:
		requiredCols(v, add, "terminal_id", "vendor_code", "branch_code", "package_code")
		if v["terminal_id"] != "" && e.atms[v["terminal_id"]] == 0 {
			add("terminal_id", "ATM tidak ditemukan atau nonaktif")
		}
		if v["package_code"] != "" && e.packages[v["vendor_code"]+"|"+v["branch_code"]+"|"+v["package_code"]] == 0 {
			add("package_code", "paket tidak ditemukan atau nonaktif")
		}
		start, serr := time.Parse(assignmentDateLayout, v["effective_start_date"])
		if serr != nil {
			add("effective_start_date", "wajib format YYYY-MM-DD")
		}
		if v["effective_end_date"] != "" {
			end, eerr := time.Parse(assignmentDateLayout, v["effective_end_date"])
			switch {
			case eerr != nil:
				add("effective_end_date", "wajib format YYYY-MM-DD")
			case serr == nil && end.Before(start):
				add("effective_end_date", "tidak boleh sebelum tanggal mulai")
			}
		}
	}
}
