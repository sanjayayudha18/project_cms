package handler

// OpenAPI contract + route RBAC for the cash-count parameter routes (ScDD 5.1, task 11.2).
// The cases plug into openapi_contract_test.go (TestOpenAPIContract_Responses / RouteCoverage);
// the RBAC test mirrors the RequireRoles groups of cmd/api/main.go.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

const (
	ccBase       = "/api/v1/cash-count"
	ccImportBase = ccBase + "/import/cash-count-pics"
)

// contractCC configures every cash-count fake of one case: err fails every call; empty returns no rows.
type contractCC struct {
	err      error
	empty    bool
	existing bool // confirm answers "already staged"
}

func (c *contractCC) staged(op string) (db.MasterDataChangeRequest, error) {
	if c.err != nil {
		return db.MasterDataChangeRequest{}, c.err
	}
	return db.MasterDataChangeRequest{ID: 90, Status: "pending", EntityType: "cash_count", Op: op}, nil
}

func ccTS() pgtype.Timestamptz { return pgtype.Timestamptz{Time: contractTime(), Valid: true} }

// -- params ---------------------------------------------------------------------

type ccParamsSvc struct{ *contractCC }

func ccContractNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(s)
	return n
}

func (s ccParamsSvc) List(context.Context) ([]db.CashCountCategoryParam, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return []db.CashCountCategoryParam{}, nil
	}
	set := db.CashCountCategoryParam{ID: 2, MediumMinAmount: ccContractNumeric("5000000000.00"), MediumMaxAmount: ccContractNumeric("15000000000.00"),
		CurrencyCode: "IDR", OnsiteHigh: 3, OnsiteMedium: 2, OnsiteLow: 1, PeriodMonths: 6, CreatedBy: 5, EffectiveFrom: ccTS()}
	old := set
	old.ID, old.SupersededAt = 1, ccTS()
	return []db.CashCountCategoryParam{set, old}, nil
}

func (s ccParamsSvc) Create(context.Context, int64, service.CashCountCategoryParamsPayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("create")
}

// -- anomaly categories -----------------------------------------------------------

type ccAnomalySvc struct{ *contractCC }

func (s ccAnomalySvc) List(context.Context, string) ([]db.CashCountAnomalyCategory, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return []db.CashCountAnomalyCategory{}, nil
	}
	desc := "kartu tertelan"
	return []db.CashCountAnomalyCategory{
		{ID: 1, Code: "CARD_STUCK", Name: "Kartu tertelan", Description: &desc, IsActive: true},
		{ID: 2, Code: "OLD", Name: "Lama", DeletedAt: ccTS()},
	}, nil
}
func (s ccAnomalySvc) Create(context.Context, int64, service.CashCountAnomalyCategoryPayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("create")
}
func (s ccAnomalySvc) Update(context.Context, int64, int64, service.CashCountAnomalyCategoryUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("update")
}
func (s ccAnomalySvc) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("disable")
}
func (s ccAnomalySvc) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("enable")
}

// -- checklist items --------------------------------------------------------------

type ccChecklistSvc struct{ *contractCC }

func (s ccChecklistSvc) List(context.Context, string, string) ([]db.CashCountChecklistItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return []db.CashCountChecklistItem{}, nil
	}
	day := "DAY1"
	return []db.CashCountChecklistItem{
		{ID: 1, Category: "SELECTED_MACHINE", Day: &day, PointNo: 1, ItemText: "Cek seal", IsActive: true},
		{ID: 2, Category: "NON_MACHINE", PointNo: 2, ItemText: "Cek brankas", DeletedAt: ccTS()},
	}, nil
}
func (s ccChecklistSvc) Create(context.Context, int64, service.CashCountChecklistItemPayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("create")
}
func (s ccChecklistSvc) Update(context.Context, int64, int64, service.CashCountChecklistItemUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("update")
}
func (s ccChecklistSvc) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("disable")
}
func (s ccChecklistSvc) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("enable")
}

// -- areas -------------------------------------------------------------------------

type ccAreaSvc struct{ *contractCC }

func (s ccAreaSvc) List(context.Context, string) ([]db.ListCashCountAreasAdminRow, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return []db.ListCashCountAreasAdminRow{}, nil
	}
	return []db.ListCashCountAreasAdminRow{
		{ID: 1, Code: "JABO", Name: "Jabodetabek", IsActive: true, RegionCodes: []string{"JKT", "BDG"}},
		{ID: 2, Code: "SUM", Name: "Sumatera", DeletedAt: ccTS()}, // nil region slice must render as []
	}, nil
}
func (s ccAreaSvc) Create(context.Context, int64, service.CashCountAreaPayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("create")
}
func (s ccAreaSvc) Update(context.Context, int64, int64, service.CashCountAreaUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("update")
}
func (s ccAreaSvc) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("disable")
}
func (s ccAreaSvc) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("enable")
}

// -- PICs ----------------------------------------------------------------------------

type ccPicSvc struct{ *contractCC }

func (s ccPicSvc) List(context.Context, string, string) ([]db.ListCashCountPicsAdminRow, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return []db.ListCashCountPicsAdminRow{}, nil
	}
	region, areaID, areaCode, areaName := "JKT", int64(1), "JABO", "Jabodetabek"
	return []db.ListCashCountPicsAdminRow{
		{ID: 1, PicRole: "COORDINATOR", UserID: 8, Username: "coord", FullName: "Coord", RegionCode: &region, IsActive: true},
		{ID: 2, PicRole: "EXECUTOR", UserID: 9, Username: "exec", FullName: "Exec", AreaID: &areaID, AreaCode: &areaCode, AreaName: &areaName,
			DeletedAt: ccTS()},
	}, nil
}
func (s ccPicSvc) Create(context.Context, int64, service.CashCountPicPayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("create")
}
func (s ccPicSvc) Update(context.Context, int64, int64, service.CashCountPicUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("update")
}
func (s ccPicSvc) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("disable")
}
func (s ccPicSvc) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("enable")
}

// -- import ----------------------------------------------------------------------------

type ccImportSvc struct{ *contractCC }

func (s ccImportSvc) DryRun(context.Context, string, io.Reader) (*service.ImportResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &service.ImportResult{Preview: service.ImportPreview{
		Entity: "cash-count-pics", TotalRows: 2, ValidRows: 1, Creates: 1, Disables: 1, Updates: 1,
		Errors: []service.ImportRowError{{Row: 3, Field: "username", Message: "user tidak ditemukan"}},
		Changes: []service.ImportChangePreview{
			{Row: 2, Op: "create", Key: "COORDINATOR/coord/JKT", Fields: []service.ImportFieldChange{{Field: "username", After: "coord"}}},
			{Row: 0, Op: "disable", Key: "EXECUTOR/exec/JABO"},
		},
	}}, nil
}

func (s ccImportSvc) Confirm(context.Context, int64, string, string, io.Reader, string) (*service.ImportConfirmResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	approval := int64(77)
	return &service.ImportConfirmResult{BatchID: 5, Rows: 2, ApprovalRequestID: &approval, Existing: s.existing}, nil
}

// -- mounting -----------------------------------------------------------------------------

// mountCashCount mounts the five handlers + the import handler as cmd/api/main.go does: RequireAuth,
// then (when guard) the RequireRoles group of each. Contract cases run unguarded (RBAC is its own test).
func mountCashCount(r chi.Router, ts *pkgauth.TokenService, cc *contractCC, guard bool) {
	if cc == nil {
		cc = &contractCC{}
	}
	mount := func(prefix string, h http.Handler, roles ...string) {
		g := r.With(custommw.RequireAuth(ts))
		if guard {
			g = g.With(custommw.RequireRoles(roles...))
		}
		g.Mount(prefix, h)
	}
	mount(ccBase+"/category-params", NewCashCountCategoryParamsHandler(ccParamsSvc{cc}).Routes(), "CC-CWM", "ADMIN")
	mount(ccBase+"/anomaly-categories", NewCashCountAnomalyCategoryHandler(ccAnomalySvc{cc}).Routes(), "CC-CCR", "ADMIN")
	mount(ccBase+"/checklist-items", NewCashCountChecklistItemHandler(ccChecklistSvc{cc}).Routes(), "CC-CCR", "ADMIN")
	mount(ccBase+"/areas", NewCashCountAreaHandler(ccAreaSvc{cc}).Routes(), "CC-CWM", "ADMIN")
	mount(ccBase+"/pics", NewCashCountPicHandler(ccPicSvc{cc}).Routes(), "CC-CWM", "ADMIN")
	mount(ccBase+"/import", NewAdminMasterDataImportHandler(ccImportSvc{cc}).Routes(), "CC-CWM")
}

// ccMultipart builds the upload body of the import routes.
func ccMultipart(t *testing.T) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "pic.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("pic_role,username,region_code,area_code\nCOORDINATOR,coord,JKT,\n"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), buf.String()
}

// -- contract cases ---------------------------------------------------------------------------

func contractCashCountCases(t *testing.T) []contractCase {
	t.Helper()
	ctype, upload := ccMultipart(t)
	var cases []contractCase
	add := func(name, method, path, body, role string, cc *contractCC, want int) {
		cases = append(cases, contractCase{name: "cc " + name, method: method, path: path, body: body, role: role, cc: cc, wantStatus: want})
	}
	fail := func(err error) *contractCC { return &contractCC{err: err} }
	validation := &service.ValidationError{Field: "name", Message: "wajib diisi"}

	// category params
	p := ccBase + "/category-params"
	paramsBody := `{"medium_min_amount":"5000000000.00","medium_max_amount":"15000000000.00","currency_code":"IDR","onsite_high":3,"onsite_medium":2,"onsite_low":1}`
	add("params get", http.MethodGet, p, "", "CC-CWM", nil, 200)
	add("params get empty", http.MethodGet, p, "", "ADMIN", &contractCC{empty: true}, 200)
	add("params get error", http.MethodGet, p, "", "CC-CWM", fail(errors.New("boom")), 500)
	add("params get no token", http.MethodGet, p, "", "", nil, 401)
	add("params post", http.MethodPost, p, paramsBody, "CC-CWM", nil, 202)
	add("params post bad json", http.MethodPost, p, "{", "CC-CWM", nil, 400)
	add("params post forbidden", http.MethodPost, p, paramsBody, "ADMIN", fail(service.ErrMasterDataForbidden), 403)
	add("params post pending", http.MethodPost, p, paramsBody, "CC-CWM", fail(service.ErrMasterDataChangePending), 409)
	add("params post invalid", http.MethodPost, p, paramsBody, "CC-CWM", fail(validation), 422)
	add("params post error", http.MethodPost, p, paramsBody, "CC-CWM", fail(errors.New("boom")), 500)

	// the four CRUD entities share list / create / update / disable / enable
	crud := []struct {
		name, base, role, createBody, updateBody string
		notFound, conflict, extraConflict        error
	}{
		{"anomaly", ccBase + "/anomaly-categories", "CC-CCR", `{"code":"CARD_STUCK","name":"Kartu tertelan"}`, `{"name":"Kartu"}`,
			service.ErrCashCountAnomalyCategoryNotFound, service.ErrCashCountAnomalyCategoryConflict, nil},
		{"checklist", ccBase + "/checklist-items", "CC-CCR", `{"category":"SELECTED_MACHINE","day":"DAY1","point_no":1,"item_text":"Cek seal"}`,
			`{"day":"DAY1","point_no":1,"item_text":"Cek seal"}`,
			service.ErrCashCountChecklistItemNotFound, service.ErrCashCountChecklistItemConflict, nil},
		{"area", ccBase + "/areas", "CC-CWM", `{"code":"JABO","name":"Jabodetabek","region_codes":["JKT"]}`, `{"name":"Jabo","region_codes":["JKT"]}`,
			service.ErrCashCountAreaNotFound, service.ErrCashCountAreaConflict, service.ErrCashCountAreaHasExecutors},
		{"pic", ccBase + "/pics", "CC-CWM", `{"pic_role":"COORDINATOR","user_id":8,"region_code":"JKT"}`, `{"user_id":8,"region_code":"JKT"}`,
			service.ErrCashCountPicNotFound, service.ErrCashCountPicConflict, nil},
	}
	for _, e := range crud {
		add(e.name+" list", http.MethodGet, e.base+"?status=all", "", e.role, nil, 200)
		add(e.name+" list admin", http.MethodGet, e.base, "", "ADMIN", nil, 200)
		add(e.name+" list empty", http.MethodGet, e.base, "", e.role, &contractCC{empty: true}, 200)
		add(e.name+" list bad status", http.MethodGet, e.base+"?status=x", "", e.role, nil, 400)
		add(e.name+" list error", http.MethodGet, e.base, "", e.role, fail(errors.New("boom")), 500)
		add(e.name+" list no token", http.MethodGet, e.base, "", "", nil, 401)
		add(e.name+" create", http.MethodPost, e.base, e.createBody, e.role, nil, 202)
		add(e.name+" create bad json", http.MethodPost, e.base, "{", e.role, nil, 400)
		add(e.name+" create forbidden", http.MethodPost, e.base, e.createBody, "ADMIN", fail(service.ErrMasterDataForbidden), 403)
		add(e.name+" create conflict", http.MethodPost, e.base, e.createBody, e.role, fail(e.conflict), 409)
		add(e.name+" create pending", http.MethodPost, e.base, e.createBody, e.role, fail(service.ErrMasterDataChangePending), 409)
		add(e.name+" create invalid", http.MethodPost, e.base, e.createBody, e.role, fail(validation), 422)
		add(e.name+" create error", http.MethodPost, e.base, e.createBody, e.role, fail(errors.New("boom")), 500)
		add(e.name+" update", http.MethodPut, e.base+"/4", e.updateBody, e.role, nil, 202)
		add(e.name+" update bad id", http.MethodPut, e.base+"/0", e.updateBody, e.role, nil, 400)
		add(e.name+" update not found", http.MethodPut, e.base+"/4", e.updateBody, e.role, fail(e.notFound), 404)
		add(e.name+" disable", http.MethodPost, e.base+"/4/disable", "", e.role, nil, 202)
		add(e.name+" disable not found", http.MethodPost, e.base+"/4/disable", "", e.role, fail(e.notFound), 404)
		add(e.name+" enable", http.MethodPost, e.base+"/4/enable", "", e.role, nil, 202)
		add(e.name+" enable conflict", http.MethodPost, e.base+"/4/enable", "", e.role, fail(e.conflict), 409)
		if e.extraConflict != nil {
			add(e.name+" disable has executors", http.MethodPost, e.base+"/4/disable", "", e.role, fail(e.extraConflict), 409)
		}
	}
	add("pic list by role", http.MethodGet, ccBase+"/pics?pic_role=EXECUTOR", "", "CC-CWM", nil, 200)
	add("pic list bad role", http.MethodGet, ccBase+"/pics?pic_role=X", "", "CC-CWM", nil, 400)
	add("checklist list by category", http.MethodGet, ccBase+"/checklist-items?category=NON_MACHINE", "", "CC-CCR", nil, 200)
	add("checklist list bad category", http.MethodGet, ccBase+"/checklist-items?category=X", "", "CC-CCR", nil, 400)

	// CSV import (multipart)
	for i, op := range []string{"dry-run", "confirm"} {
		path := ccImportBase + "/" + op
		okStatus := []int{200, 202}[i]
		imp := func(name, p string, cc *contractCC, want int) {
			cases = append(cases, contractCase{name: "cc import " + op + " " + name, method: http.MethodPost, path: p, body: upload,
				ctype: ctype, role: "CC-CWM", cc: cc, wantStatus: want})
		}
		imp("ok", path, nil, okStatus)
		imp("unknown entity", ccBase+"/import/nope/"+op, nil, 404)
		imp("forbidden", path, fail(service.ErrMasterDataForbidden), 403)
		imp("row errors", path, fail(&service.ImportInvalidError{Errors: []service.ImportRowError{{Row: 2, Field: "username", Message: "x"}}}), 422)
		imp("nothing to import", path, fail(service.ErrImportNothingToDo), 422)
		imp("error", path, fail(errors.New("boom")), 500)
		cases = append(cases,
			contractCase{name: "cc import " + op + " no file", method: http.MethodPost, path: path, body: "{}", role: "CC-CWM", wantStatus: 422},
			contractCase{name: "cc import " + op + " no token", method: http.MethodPost, path: path, body: upload, ctype: ctype, wantStatus: 401})
	}
	confirm := func(name string, cc *contractCC, want int) {
		cases = append(cases, contractCase{name: "cc import confirm " + name, method: http.MethodPost, path: ccImportBase + "/confirm",
			body: upload, ctype: ctype, role: "CC-CWM", cc: cc, wantStatus: want})
	}
	confirm("existing batch", &contractCC{existing: true}, 200)
	confirm("pending", fail(service.ErrMasterDataChangePending), 409)
	return cases
}

// -- route RBAC ---------------------------------------------------------------------------------

// TestCashCountRoutes_RBAC walks every cash-count route: no token = 401, a role outside the group's
// RequireRoles = 403 (the handler is never reached), an allowed role passes the guard. The maker-only
// rule for ADMIN (read yes, write no) lives in MasterDataChangeService.Submit and its own tests.
func TestCashCountRoutes_RBAC(t *testing.T) {
	ts := testTokenService()
	r := chi.NewRouter()
	mountCashCount(r, ts, &contractCC{}, true)

	groups := []struct {
		prefix  string
		allowed []string
		denied  []string
	}{
		{"/category-params", []string{"CC-CWM", "ADMIN"}, []string{"CC-CCR", "CC-EXECUTOR", "CC-COORDINATOR", "ATM-SPV", "ADMIN_PARAM"}},
		{"/areas", []string{"CC-CWM", "ADMIN"}, []string{"CC-CCR", "CC-EXECUTOR", "CC-COORDINATOR", "ATM-SPV", "ADMIN_PARAM"}},
		{"/pics", []string{"CC-CWM", "ADMIN"}, []string{"CC-CCR", "CC-EXECUTOR", "CC-COORDINATOR", "ATM-SPV", "ADMIN_PARAM"}},
		{"/anomaly-categories", []string{"CC-CCR", "ADMIN"}, []string{"CC-CWM", "CC-EXECUTOR", "CC-COORDINATOR", "ATM-SPV", "ADMIN_PARAM"}},
		{"/checklist-items", []string{"CC-CCR", "ADMIN"}, []string{"CC-CWM", "CC-EXECUTOR", "CC-COORDINATOR", "ATM-SPV", "ADMIN_PARAM"}},
		{"/import", []string{"CC-CWM"}, []string{"ADMIN", "ADMIN_PARAM", "CC-CCR", "CC-EXECUTOR", "ATM-SPV"}},
	}
	// routes come from the router, so a new route can't dodge the guard
	type route struct{ method, path string }
	var routes []route
	_ = chi.Walk(r, func(method, p string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, route{method, strings.ReplaceAll(p, "/*", "")})
		return nil
	})
	seen := 0
	for _, g := range groups {
		for _, rt := range routes {
			if !strings.HasPrefix(rt.path, ccBase+g.prefix) {
				continue
			}
			seen++
			t.Run(rt.method+" "+rt.path, func(t *testing.T) {
				path := concretePath(rt.path)
				do := func(role string) int {
					req := httptest.NewRequest(rt.method, path, nil)
					if role != "" {
						req.Header.Set("Authorization", "Bearer "+tokenForRole(t, ts, 5, role))
					}
					rec := httptest.NewRecorder()
					r.ServeHTTP(rec, req)
					return rec.Code
				}
				if got := do(""); got != http.StatusUnauthorized {
					t.Errorf("no token: status = %d, want 401", got)
				}
				for _, role := range g.denied {
					if got := do(role); got != http.StatusForbidden {
						t.Errorf("role %s: status = %d, want 403", role, got)
					}
				}
				for _, role := range g.allowed {
					if got := do(role); got == http.StatusUnauthorized || got == http.StatusForbidden {
						t.Errorf("role %s: status = %d, want the guard to pass", role, got)
					}
				}
			})
		}
	}
	if seen == 0 {
		t.Fatal("no cash-count routes found")
	}
}

// concretePath fills chi path parameters with a valid value.
func concretePath(p string) string {
	var out strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] != '{' {
			out.WriteByte(p[i])
			continue
		}
		end := strings.IndexByte(p[i:], '}')
		if p[i+1:i+end] == "entity" {
			out.WriteString("cash-count-pics")
		} else {
			out.WriteString("4")
		}
		i += end
	}
	return out.String()
}
