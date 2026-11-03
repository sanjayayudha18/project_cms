package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeAcmAreaServicer exercises HTTP wiring only; logic is covered by
// internal/service/acm_area_admin_integration_test.go.
type fakeAcmAreaServicer struct{ err error }

func (f *fakeAcmAreaServicer) List(context.Context, string) ([]db.ListAcmAreasAdminRow, error) {
	return []db.ListAcmAreasAdminRow{{ID: 1, Name: "Jabodetabek", IsActive: true, BranchCount: 3}}, nil
}
func (f *fakeAcmAreaServicer) Get(context.Context, int64) (*service.AcmAreaDetail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &service.AcmAreaDetail{Area: db.AcmArea{ID: 1, Name: "Jabodetabek"}}, nil
}
func (f *fakeAcmAreaServicer) EligibleUsers(context.Context) ([]db.ListAcmEligibleUsersRow, error) {
	return nil, nil
}
func (f *fakeAcmAreaServicer) BranchOptions(context.Context) ([]db.ListAcmBranchOptionsRow, error) {
	return nil, nil
}
func (f *fakeAcmAreaServicer) RegionOptions(context.Context) ([]db.ListAcmRegionOptionsRow, error) {
	return []db.ListAcmRegionOptionsRow{{RegionCode: "JKT", BranchCount: 4}}, nil
}
func (f *fakeAcmAreaServicer) Warnings(context.Context) ([]db.ListVaultPlanRegionWarningsRow, error) {
	return []db.ListVaultPlanRegionWarningsRow{{VendorRequestID: 9, RequestNumber: "REP-BJK-20261008-001"}}, nil
}
func (f *fakeAcmAreaServicer) SetRegions(_ context.Context, _ int64, _ string, _ int64, codes []string, _ string) (*service.AcmAreaDetail, error) {
	return &service.AcmAreaDetail{Regions: codes}, f.err
}
func (f *fakeAcmAreaServicer) Create(context.Context, int64, string, string, string) (db.AcmArea, error) {
	return db.AcmArea{ID: 2, Name: "Makassar"}, f.err
}
func (f *fakeAcmAreaServicer) Rename(context.Context, int64, string, int64, string, string) (db.AcmArea, error) {
	return db.AcmArea{ID: 1}, f.err
}
func (f *fakeAcmAreaServicer) SetActive(context.Context, int64, string, int64, bool, string) (db.AcmArea, error) {
	return db.AcmArea{ID: 1}, f.err
}
func (f *fakeAcmAreaServicer) SetBranches(context.Context, int64, string, int64, []int64, string) (*service.AcmAreaDetail, error) {
	return &service.AcmAreaDetail{}, f.err
}
func (f *fakeAcmAreaServicer) SetMembers(context.Context, int64, string, int64, []int64, string) (*service.AcmAreaDetail, error) {
	return &service.AcmAreaDetail{}, f.err
}

func TestAdminAcmAreaHandler(t *testing.T) {
	const base = "/api/v1/admin/acm-areas"
	conflict := &service.AcmAreaBranchConflictError{Conflicts: []db.ListBranchesInOtherAcmAreasRow{{VendorBranchID: 5, AcmAreaID: 1, AcmAreaName: "Jabodetabek"}}}
	cases := []struct {
		name, role, method, path, body string
		svcErr                         error
		wantCode                       int
		wantBody                       string
	}{
		{"list", "ADMIN", http.MethodGet, base, "", nil, http.StatusOK, `"branch_count":3`},
		{"warnings", "ADMIN", http.MethodGet, base + "/warnings", "", nil, http.StatusOK, `"requests_without_area":[{"region_code":null`},
		{"region options", "ADMIN", http.MethodGet, base + "/region-options", "", nil, http.StatusOK, `"region_code":"JKT"`},
		{"get regions", "ADMIN", http.MethodGet, base + "/1/regions", "", nil, http.StatusOK, `"region_codes"`},
		{"set regions", "ADMIN", http.MethodPut, base + "/1/regions", `{"region_codes":["JKT"]}`, nil, http.StatusOK, `"region_codes":["JKT"]`},
		{"regions field required", "ADMIN", http.MethodPut, base + "/1/regions", `{}`, nil, http.StatusUnprocessableEntity, `region_codes`},
		{"region conflict lists the holder", "ADMIN", http.MethodPut, base + "/2/regions", `{"region_codes":["JKT"]}`,
			&service.AcmAreaRegionConflictError{Conflicts: []db.ListRegionsInOtherAcmAreasRow{{RegionCode: "JKT", AcmAreaID: 1, AcmAreaName: "Jabodetabek"}}},
			http.StatusConflict, `"acm_area_name":"Jabodetabek"`},
		{"create 201", "ADMIN", http.MethodPost, base, `{"name":"Makassar"}`, nil, http.StatusCreated, `"name":"Makassar"`},
		{"ADMIN_PARAM denied at route", "ADMIN_PARAM", http.MethodPost, base, `{"name":"x"}`, nil, http.StatusForbidden, ""},
		{"ACM-USER denied at route", "ACM-USER", http.MethodGet, base, "", nil, http.StatusForbidden, ""},
		{"service RBAC 403", "ADMIN", http.MethodPost, base, `{"name":"x"}`, service.ErrNotAuthorized, http.StatusForbidden, ""},
		{"duplicate name 409", "ADMIN", http.MethodPost, base, `{"name":"x"}`, service.ErrAcmAreaNameConflict, http.StatusConflict, ""},
		{"not found 404", "ADMIN", http.MethodGet, base + "/7", "", service.ErrAcmAreaNotFound, http.StatusNotFound, ""},
		{"branch conflict lists the holder", "ADMIN", http.MethodPut, base + "/2/branches", `{"vendor_branch_ids":[5]}`, conflict, http.StatusConflict, `"acm_area_name":"Jabodetabek"`},
		{"links field required", "ADMIN", http.MethodPut, base + "/2/members", `{}`, nil, http.StatusUnprocessableEntity, `user_ids`},
		{"members ok", "ADMIN", http.MethodPut, base + "/2/members", `{"user_ids":[]}`, nil, http.StatusOK, ""},
		{"disable", "ADMIN", http.MethodPost, base + "/1/disable", "", nil, http.StatusOK, ""},
		{"eligible users", "ADMIN", http.MethodGet, base + "/eligible-users", "", nil, http.StatusOK, `"users"`},
		{"branch options", "ADMIN", http.MethodGet, base + "/branch-options", "", nil, http.StatusOK, `"branches"`},
		{"rename", "ADMIN", http.MethodPut, base + "/1", `{"name":"Jakarta"}`, nil, http.StatusOK, `"id":1`},
		{"rename bad body", "ADMIN", http.MethodPut, base + "/1", `{bad`, nil, http.StatusBadRequest, ""},
		{"rename bad id", "ADMIN", http.MethodPut, base + "/x", `{"name":"Jakarta"}`, nil, http.StatusBadRequest, ""},
		{"rename duplicate 409", "ADMIN", http.MethodPut, base + "/1", `{"name":"Jakarta"}`, service.ErrAcmAreaNameConflict, http.StatusConflict, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
				SecretKey: []byte("test-secret-minimum-32-bytes-long!!"), AccessTokenExpiry: 15 * time.Minute, SessionMaxLifetime: time.Hour,
			}, noopBlacklist{})
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN")).
				Mount(base, NewAdminAcmAreaHandler(&fakeAcmAreaServicer{err: tc.svcErr}).Routes())

			rec := doRequest(r, tc.method, tc.path, tokenForRole(t, tokenSvc, 1, tc.role), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %s does not contain %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
