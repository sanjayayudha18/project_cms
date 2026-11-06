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

// fakeVendorPicAdminServicer exercises HTTP wiring only; logic is covered
// by internal/service/vendor_pic_admin_test.go.
type fakeVendorPicAdminServicer struct {
	listResult []service.VendorPic
	warnings   []string
	getResult  *service.VendorPic
	createErr  error
	updateBody *service.VendorPicUpdatePayload
	createBody *service.VendorPicUpdatePayload
	enableErr  error
	rolesErr   error
	rolesGot   []string
}

func (f *fakeVendorPicAdminServicer) List(context.Context, db.ListVendorPicsAdminParams) ([]service.VendorPic, error) {
	return f.listResult, nil
}
func (f *fakeVendorPicAdminServicer) Count(context.Context, db.CountVendorPicsAdminParams) (int64, error) {
	return int64(len(f.listResult)), nil
}
func (f *fakeVendorPicAdminServicer) Get(context.Context, int64, int64) (*service.VendorPic, error) {
	return f.getResult, nil
}
func (f *fakeVendorPicAdminServicer) Warnings(context.Context, int64) ([]string, error) {
	return f.warnings, nil
}
func (f *fakeVendorPicAdminServicer) Create(_ context.Context, _, _ int64, req service.VendorPicUpdatePayload, _ string) (db.MasterDataChangeRequest, error) {
	f.createBody = &req
	return db.MasterDataChangeRequest{ID: 7, EntityType: "vendor_pic", Op: "create", Status: "pending"}, f.createErr
}
func (f *fakeVendorPicAdminServicer) Update(_ context.Context, _, _, _ int64, req service.VendorPicUpdatePayload, _ string) (db.MasterDataChangeRequest, error) {
	f.updateBody = &req
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, nil
}
func (f *fakeVendorPicAdminServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, nil
}
func (f *fakeVendorPicAdminServicer) Enable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, f.enableErr
}

func (f *fakeVendorPicAdminServicer) SetRoles(_ context.Context, _, _, _ int64, roles []string, _ string) ([]string, error) {
	f.rolesGot = roles
	return roles, f.rolesErr
}

func mountAdminVendorPicHandler(svc *fakeVendorPicAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/pics", NewAdminVendorPicHandler(svc, svc).Routes())
	return r, tokenSvc
}

func TestAdminVendorPicHandler_List_IncludesWarnings(t *testing.T) {
	svc := &fakeVendorPicAdminServicer{
		listResult: []service.VendorPic{{ID: 1, VendorID: 3, Name: "Budi", IsActive: true}},
		warnings:   []string{service.NoNotificationRecipientWarning},
	}
	router, tokenSvc := mountAdminVendorPicHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"name":"Budi"`) || !strings.Contains(body, service.NoNotificationRecipientWarning) {
		t.Errorf("expected pic row + warning, got: %s", body)
	}
}

func TestAdminVendorPicHandler_Get_NotFound(t *testing.T) {
	router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/pics/99", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdminVendorPicHandler_Create_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{})
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ADMIN"),
		`{"name":"Budi","email":"budi@example.com","is_notification_recipient":true}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id":7`) {
		t.Fatalf("expected 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminVendorPicHandler_Create_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&service.ValidationError{Field: "email", Message: "format email tidak valid"}, http.StatusUnprocessableEntity},
		{service.ErrVendorPicNotFound, http.StatusNotFound},
		{service.ErrMasterDataChangePending, http.StatusConflict},
	}
	for _, tc := range cases {
		router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{createErr: tc.err})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"name":"Budi"}`)
		if rec.Code != tc.want {
			t.Errorf("err %v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
	}
}

func TestAdminVendorPicHandler_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ATM-USER"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminVendorPicHandler_Toggle_Returns202(t *testing.T) {
	router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{})
	for _, path := range []string{"/disable", "/enable"} {
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics/1"+path, tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), "")
		if rec.Code != http.StatusAccepted {
			t.Errorf("%s: expected 202, got %d", path, rec.Code)
		}
	}
}

func TestAdminVendorPicHandler_SetRoles_Returns200(t *testing.T) {
	svc := &fakeVendorPicAdminServicer{}
	router, tokenSvc := mountAdminVendorPicHandler(svc)
	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/pics/1/roles", tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), `{"roles":["vault"]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"roles":["vault"]`) {
		t.Fatalf("expected 200 with roles, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(svc.rolesGot) != 1 || svc.rolesGot[0] != "vault" {
		t.Errorf("service got roles %v, want [vault]", svc.rolesGot)
	}
}

func TestAdminVendorPicHandler_SetRoles_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		body string
		err  error
		want int
	}{
		{"roles missing", `{}`, nil, http.StatusUnprocessableEntity},
		{"bad json", `{`, nil, http.StatusBadRequest},
		{"unknown role", `{"roles":["x"]}`, &service.ValidationError{Field: "roles", Message: "tidak dikenal"}, http.StatusUnprocessableEntity},
		{"not found", `{"roles":[]}`, service.ErrVendorPicNotFound, http.StatusNotFound},
		{"forbidden", `{"roles":[]}`, service.ErrMasterDataForbidden, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{rolesErr: tc.err})
			rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/pics/1/roles", tokenForRole(t, tokenSvc, 1, "ADMIN"), tc.body)
			if rec.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAdminVendorPicHandler_SetRoles_NonAdminForbidden(t *testing.T) {
	router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{})
	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/pics/1/roles", tokenForRole(t, tokenSvc, 1, "ATM-SPV"), `{"roles":[]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// The form update must not carry roles (Req 5.7), even when the client sends them.
func TestAdminVendorPicHandler_Update_DropsRoles(t *testing.T) {
	svc := &fakeVendorPicAdminServicer{}
	router, tokenSvc := mountAdminVendorPicHandler(svc)
	rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/pics/1", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"name":"Budi","roles":["vault"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.updateBody == nil || svc.updateBody.Roles != nil {
		t.Fatalf("update payload roles = %v, want nil", svc.updateBody)
	}
}

func TestAdminVendorPicHandler_Get_IncludesRoles(t *testing.T) {
	router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{getResult: &service.VendorPic{ID: 1, VendorID: 3, Roles: []string{"replenish"}}})
	rec := doRequest(router, http.MethodGet, "/api/v1/admin/vendors/3/pics/1", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"roles":["replenish"]`) {
		t.Fatalf("expected 200 with roles, got %d: %s", rec.Code, rec.Body.String())
	}
}

// vendor-pks Req 1.1, 1.10: is_director is read from the create body and returned by list/detail.
func TestAdminVendorPicHandler_Create_PassesIsDirector(t *testing.T) {
	svc := &fakeVendorPicAdminServicer{}
	router, tokenSvc := mountAdminVendorPicHandler(svc)
	rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"name":"Dirut","is_director":true}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.createBody == nil || svc.createBody.IsDirector == nil || !*svc.createBody.IsDirector {
		t.Fatalf("create payload is_director = %+v, want true", svc.createBody)
	}
}

func TestAdminVendorPicHandler_Responses_IncludeIsDirector(t *testing.T) {
	svc := &fakeVendorPicAdminServicer{
		getResult:  &service.VendorPic{ID: 1, VendorID: 3, IsDirector: true},
		listResult: []service.VendorPic{{ID: 1, VendorID: 3, IsDirector: true}},
	}
	router, tokenSvc := mountAdminVendorPicHandler(svc)
	for _, path := range []string{"/api/v1/admin/vendors/3/pics/1", "/api/v1/admin/vendors/3/pics"} {
		rec := doRequest(router, http.MethodGet, path, tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"is_director":true`) {
			t.Errorf("%s: expected 200 with is_director true, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}

// vendor-pks Req 1.5, 1.6: director conflicts are 409 with an Indonesian message,
// director-shape violations are 422.
func TestAdminVendorPicHandler_DirectorErrorMapping(t *testing.T) {
	t.Run("create: second director 409", func(t *testing.T) {
		router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{createErr: service.ErrVendorDirectorExists})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"name":"Dirut","is_director":true}`)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), service.ErrVendorDirectorExists.Error()) {
			t.Fatalf("expected 409 with message, got %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("create: director with branch 422", func(t *testing.T) {
		err := &service.ValidationError{Field: "vendor_branch_id", Message: "PIC Direktur Utama harus tingkat vendor (tanpa cabang)"}
		router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{createErr: err})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"name":"Dirut","is_director":true,"vendor_branch_id":9}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("enable: another director active 409", func(t *testing.T) {
		router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{enableErr: service.ErrVendorDirectorExists})
		rec := doRequest(router, http.MethodPost, "/api/v1/admin/vendors/3/pics/1/enable", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("roles: director cannot hold roles 409", func(t *testing.T) {
		router, tokenSvc := mountAdminVendorPicHandler(&fakeVendorPicAdminServicer{rolesErr: service.ErrVendorDirectorNoRoles})
		rec := doRequest(router, http.MethodPut, "/api/v1/admin/vendors/3/pics/1/roles", tokenForRole(t, tokenSvc, 1, "ADMIN"), `{"roles":["vault"]}`)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), service.ErrVendorDirectorNoRoles.Error()) {
			t.Fatalf("expected 409 with message, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
