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
func (f *fakeVendorPicAdminServicer) Create(context.Context, int64, int64, service.VendorPicUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 7, EntityType: "vendor_pic", Op: "create", Status: "pending"}, f.createErr
}
func (f *fakeVendorPicAdminServicer) Update(context.Context, int64, int64, int64, service.VendorPicUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 8, Status: "pending"}, nil
}
func (f *fakeVendorPicAdminServicer) Disable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 9, Status: "pending"}, nil
}
func (f *fakeVendorPicAdminServicer) Enable(context.Context, int64, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return db.MasterDataChangeRequest{ID: 10, Status: "pending"}, nil
}

func mountAdminVendorPicHandler(svc VendorPicAdminServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/pics", NewAdminVendorPicHandler(svc).Routes())
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
