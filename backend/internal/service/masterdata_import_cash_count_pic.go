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

// ExportCashCountPICs is the CSV import/export entity of mapping PIC cash count (ScDD 5.1).
const ExportCashCountPICs = csvexport.CashCountPICs

// CashCountPicImportRepo is the PRIMARY read surface of the replace-all PIC import.
// *repository.CashCountPicAdminRepository satisfies it (build it on the primary for both pools).
type CashCountPicImportRepo interface {
	ActivePics(ctx context.Context) ([]db.ListCashCountPicsAdminRow, error)
	ActiveAreas(ctx context.Context) ([]db.ListCashCountAreasAdminRow, error)
	UserIDByUsername(ctx context.Context, username string) (int64, bool, error)
	UserEligible(ctx context.Context, userID int64, picRole string) (bool, error)
	RegionInUse(ctx context.Context, regionCode string) (bool, error)
}

// WithCashCountPics enables the mapping PIC import (AC-5.1-11).
func (i *MasterDataImporter) WithCashCountPics(repo CashCountPicImportRepo) *MasterDataImporter {
	i.cashCountPics = repo
	return i
}

// importEntityType is the change-request entity type whose maker roles guard an
// import of entity; "" = the default master-data makers (ADMIN/ADMIN_PARAM).
func importEntityType(entity string) string {
	if entity == ExportCashCountPICs {
		return cashCountPicEntity
	}
	return ""
}

// authorizeMasterDataImport re-checks the caller's role for importing entity
// (service layer, independent of the route guard): CC-CWM for mapping PIC,
// ADMIN/ADMIN_PARAM for the other entities.
func authorizeMasterDataImport(ctx context.Context, entity string) error {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok || authCtx == nil || !makerRolesFor(importEntityType(entity))[strings.ToUpper(strings.TrimSpace(authCtx.Role))] {
		return ErrMasterDataForbidden
	}
	return nil
}

// picImportKey identifies a mapping in the file and in the DB.
func picImportKey(role, username, region, area string) string {
	return role + "|" + username + "|" + region + "|" + area
}

// picImport is the state of one replace-all dry run: active mappings, areas,
// and per-run caches of the user/region checks.
type picImport struct {
	repo     CashCountPicImportRepo
	active   map[string]int64 // key -> id of the active mapping
	areas    map[string]int64 // active area code -> id
	users    map[string]int64 // username -> id (0 = unknown)
	eligible map[string]bool  // role|user id -> eligible
	regions  map[string]bool  // region_code -> used by an active vendor branch
	seenKey  map[string]int   // key -> first file line
}

// dryRunCashCountPics validates a mapping PIC file in replace-all mode: each
// file row is kept (already active) or created, and every active mapping that
// is not in the file becomes a disable in the same batch. Nothing is written.
func (i *MasterDataImporter) dryRunCashCountPics(ctx context.Context, r io.Reader) (*ImportResult, error) {
	if i.cashCountPics == nil {
		return nil, errors.New("cash count pic import is not configured")
	}
	e, _ := csvexport.Lookup(ExportCashCountPICs)
	res := &ImportResult{Preview: ImportPreview{Entity: ExportCashCountPICs, Errors: []ImportRowError{}, Changes: []ImportChangePreview{}}}
	_, recs, fileErrs, err := parseEntityImportCSV(ExportCashCountPICs, exportSpec{header: e.Header}, r)
	if err != nil {
		return nil, err
	}
	if len(fileErrs) > 0 {
		res.Preview.Errors = fileErrs
		return res, nil
	}
	if len(recs) == 0 {
		// Replace-all of an empty file would disable every mapping: refuse it.
		res.Preview.Errors = []ImportRowError{{Row: 1, Field: "file", Message: "file tidak berisi data; impor replace-all tidak boleh menonaktifkan semua mapping PIC"}}
		return res, nil
	}
	res.Preview.TotalRows = len(recs)

	p, cur, activeRows, err := i.loadPicImport(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	dropped := 0
	for _, rec := range recs {
		row, errs, err := p.validate(ctx, e.Header, rec)
		if err != nil {
			return nil, err
		}
		if len(errs) == 0 {
			if row.Op == ImportOpUnchanged {
				seen[row.ID] = true
			}
			res.Rows = append(res.Rows, row)
			continue
		}
		for _, re := range errs {
			if len(res.Preview.Errors) < masterDataImportMaxErrs {
				res.Preview.Errors = append(res.Preview.Errors, re)
			} else {
				dropped++
			}
		}
	}
	res.Preview.ErrorsTruncated = dropped > 0
	res.Preview.ValidRows = len(res.Rows)
	for _, a := range activeRows {
		if seen[a.ID] {
			continue
		}
		cells := picCells(a.PicRole, a.Username, picDeref(a.RegionCode), picDeref(a.AreaCode))
		cells["is_active"] = "false"
		// Row 0: the mapping is not in the file (disabled by replace-all).
		res.Rows = append(res.Rows, ImportRow{Row: 0, Op: ImportOpUpdate, ID: a.ID, Cells: cells, Toggle: true})
	}
	res.tally(ExportCashCountPICs, e.Header, cur)
	return res, nil
}

func picCells(role, username, region, area string) map[string]string {
	return map[string]string{"pic_role": role, "username": username, "region_code": region, "area_code": area}
}

// loadPicImport reads the active mappings and areas; cur holds each active
// mapping's cells (is_active "true") for the preview's before values.
func (i *MasterDataImporter) loadPicImport(ctx context.Context) (*picImport, map[int64]map[string]string, []db.ListCashCountPicsAdminRow, error) {
	repo := i.cashCountPics
	rows, err := repo.ActivePics(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading cash count pics for import: %w", err)
	}
	areas, err := repo.ActiveAreas(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading cash count areas for import: %w", err)
	}
	p := &picImport{repo: repo, active: map[string]int64{}, areas: map[string]int64{}, users: map[string]int64{},
		eligible: map[string]bool{}, regions: map[string]bool{}, seenKey: map[string]int{}}
	cur := make(map[int64]map[string]string, len(rows))
	for _, a := range rows {
		region, area := picDeref(a.RegionCode), picDeref(a.AreaCode)
		p.active[picImportKey(a.PicRole, a.Username, region, area)] = a.ID
		c := picCells(a.PicRole, a.Username, region, area)
		c["is_active"] = "true"
		cur[a.ID] = c
	}
	for _, a := range areas {
		p.areas[a.Code] = a.ID
	}
	return p, cur, rows, nil
}

// validate checks one file row (same rules as CashCountPicAdminService.validate) and
// resolves it to unchanged (already active) or create. err is an infrastructure failure.
func (p *picImport) validate(ctx context.Context, header []string, rec importRecord) (ImportRow, []ImportRowError, error) {
	var errs []ImportRowError
	add := func(field, msg string) {
		errs = append(errs, ImportRowError{Row: rec.line, Field: field, Message: msg})
	}
	if rec.badCount > 0 || len(rec.cells) != len(header) {
		want := len(header)
		if rec.badCount > 0 {
			want = rec.badCount
		}
		add("row", fmt.Sprintf("jumlah kolom %d, seharusnya %d", len(rec.cells), want))
		return ImportRow{}, errs, nil
	}
	v := make(map[string]string, len(header)+2)
	for c, h := range header {
		v[h] = strings.TrimSpace(rec.cells[c])
	}
	v["pic_role"] = strings.ToUpper(v["pic_role"])
	v["region_code"] = strings.ToUpper(v["region_code"])
	v["area_code"] = strings.ToUpper(v["area_code"])
	role := v["pic_role"]
	switch role {
	case picRoleCoordinator:
		requiredCols(v, add, "region_code")
		if v["area_code"] != "" {
			add("area_code", "harus kosong untuk COORDINATOR")
		}
	case picRoleExecutor:
		requiredCols(v, add, "area_code")
		if v["region_code"] != "" {
			add("region_code", "harus kosong untuk EXECUTOR")
		}
	default:
		add("pic_role", "harus COORDINATOR atau EXECUTOR")
	}
	requiredCols(v, add, "username")
	if len(errs) > 0 {
		return ImportRow{}, errs, nil
	}
	key := picImportKey(role, v["username"], v["region_code"], v["area_code"])
	if first, dup := p.seenKey[key]; dup {
		add("username", fmt.Sprintf("duplikat dengan baris %d", first))
		return ImportRow{}, errs, nil
	}
	p.seenKey[key] = rec.line

	if err := p.checkRefs(ctx, v, add); err != nil {
		return ImportRow{}, nil, err
	}
	if len(errs) > 0 {
		return ImportRow{}, errs, nil
	}
	if id, ok := p.active[key]; ok {
		return ImportRow{Row: rec.line, Op: ImportOpUnchanged, ID: id, Cells: v}, nil, nil
	}
	return ImportRow{Row: rec.line, Op: ImportOpCreate, Cells: v}, nil, nil
}

// checkRefs resolves the user and the region/area, storing user_id / area_id in v.
func (p *picImport) checkRefs(ctx context.Context, v map[string]string, add func(string, string)) error {
	role := v["pic_role"]
	uid, err := p.userID(ctx, v["username"])
	if err != nil {
		return err
	}
	if uid == 0 {
		add("username", "user tidak ditemukan")
	} else {
		ok, err := p.userEligible(ctx, uid, role)
		if err != nil {
			return err
		}
		if !ok {
			ccRole := "CC-EXECUTOR"
			if role == picRoleCoordinator {
				ccRole = "CC-COORDINATOR"
			}
			add("username", "user harus aktif, login LDAP, dan berperan "+ccRole)
		}
		v["user_id"] = strconv.FormatInt(uid, 10)
	}
	if role == picRoleCoordinator {
		inUse, err := p.regionInUse(ctx, v["region_code"])
		if err != nil {
			return err
		}
		if !inUse {
			add("region_code", "region_code tidak dipakai cabang vendor aktif")
		}
		return nil
	}
	areaID := p.areas[v["area_code"]]
	if areaID == 0 {
		add("area_code", "area tidak ditemukan atau nonaktif")
		return nil
	}
	v["area_id"] = strconv.FormatInt(areaID, 10)
	return nil
}

func (p *picImport) userID(ctx context.Context, username string) (int64, error) {
	if id, ok := p.users[username]; ok {
		return id, nil
	}
	id, found, err := p.repo.UserIDByUsername(ctx, username)
	if err != nil {
		return 0, fmt.Errorf("looking up user %q for import: %w", username, err)
	}
	if !found {
		id = 0
	}
	p.users[username] = id
	return id, nil
}

func (p *picImport) userEligible(ctx context.Context, userID int64, role string) (bool, error) {
	k := role + "|" + strconv.FormatInt(userID, 10)
	if ok, cached := p.eligible[k]; cached {
		return ok, nil
	}
	ok, err := p.repo.UserEligible(ctx, userID, role)
	if err != nil {
		return false, fmt.Errorf("checking pic user for import: %w", err)
	}
	p.eligible[k] = ok
	return ok, nil
}

func (p *picImport) regionInUse(ctx context.Context, region string) (bool, error) {
	if ok, cached := p.regions[region]; cached {
		return ok, nil
	}
	ok, err := p.repo.RegionInUse(ctx, region)
	if err != nil {
		return false, fmt.Errorf("checking region code for import: %w", err)
	}
	p.regions[region] = ok
	return ok, nil
}

// stageCashCountPic runs one replace-all row through CashCountPicAdminService (which
// re-validates and records its change on the collector): create, or disable of an
// active mapping missing from the file.
func stageCashCountPic(ctx context.Context, maker int64, row ImportRow, s ImportServices, ip string) error {
	if s.CashCountPICs == nil {
		return errors.New("cash count pic import service is not configured")
	}
	if row.Toggle {
		_, err := s.CashCountPICs.Disable(ctx, maker, row.ID, ip)
		return err
	}
	v := row.Cells
	userID, _ := strconv.ParseInt(v["user_id"], 10, 64) // set by the dry-run
	_, err := s.CashCountPICs.Create(ctx, maker, CashCountPicPayload{PicRole: v["pic_role"], UserID: userID,
		RegionCode: optStr(v["region_code"]), AreaID: optInt64(v["area_id"])}, ip)
	return err
}
