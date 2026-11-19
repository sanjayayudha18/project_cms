package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Spec vendor-pks-list task 5.3: HTTP wiring of GET /admin/pks and PUT /admin/pks/{id}/reminder,
// and the Property 7 role matrix on the vendorRead group (mounted exactly as in cmd/api/main.go).

type fakePksListServicer struct {
	err     error
	calls   int
	gotRole string
	gotP    service.PksListParams
}

func (f *fakePksListServicer) List(_ context.Context, role string, p service.PksListParams) (*service.PksListPage, error) {
	f.calls++
	f.gotRole, f.gotP = role, p
	if f.err != nil {
		return nil, f.err
	}
	return &service.PksListPage{Items: []service.PksListItem{}, Total: 0, Page: 1, PageSize: 20}, nil
}

type fakeReminderServicer struct {
	err              error
	calls            int
	gotActor, gotPks int64
	gotRole          string
	gotDays          int32
}

func (f *fakeReminderServicer) UpdateReminder(_ context.Context, actor int64, role string, pksID int64, days int32, _ string) (db.UpdateVendorPksRenewalDaysRow, error) {
	f.calls++
	f.gotActor, f.gotRole, f.gotPks, f.gotDays = actor, role, pksID, days
	if f.err != nil {
		return db.UpdateVendorPksRenewalDaysRow{}, f.err
	}
	return db.UpdateVendorPksRenewalDaysRow{ID: pksID, RenewalDays: days}, nil
}

// mountVendorReadGroup mirrors main.go: RequireAuth + RequireRoles(PksViewerRoles) over the vendor
// list/detail and Daftar PKS; handlers narrow every write to ADMIN/ADMIN_PARAM.
func mountVendorReadGroup(vendors *fakeVendorAdminServicer, list *fakePksListServicer, rem *fakeReminderServicer) (http.Handler, *pkgauth.TokenService) {
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	g := r.With(custommw.RequireAuth(ts), custommw.RequireRoles(reports.PksViewerRoles...))
	g.Mount("/api/v1/admin/vendors", NewAdminVendorHandler(vendors).Routes())
	g.Mount("/api/v1/admin/pks", NewAdminPksHandler(list, rem).Routes())
	return r, ts
}

func TestAdminPksHandler_List_PassesQueryAndRole(t *testing.T) {
	list := &fakePksListServicer{}
	router, ts := mountVendorReadGroup(&fakeVendorAdminServicer{}, list, &fakeReminderServicer{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/pks?status=expired&pks_type=CIT&vendor=ACM&q=wod&page=2&page_size=5",
		tokenForRole(t, ts, 1, "ATM-SPV"), "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	p := list.gotP
	if list.gotRole != "ATM-SPV" || p.Status != "expired" || p.PksType != "CIT" || p.Vendor != "ACM" || p.Q != "wod" || p.Page != 2 || p.PageSize != 5 {
		t.Errorf("role/params = %s/%+v", list.gotRole, p)
	}
}

func TestAdminPksHandler_List_Errors(t *testing.T) {
	tests := []struct {
		name  string
		query string
		err   error
		want  int
	}{
		{"bad page", "?page=abc", nil, http.StatusBadRequest},
		{"bad page_size", "?page_size=x", nil, http.StatusBadRequest},
		{"validation", "?status=zzz", &service.ValidationError{Field: "filter", Message: "bad"}, http.StatusUnprocessableEntity},
		{"forbidden", "", service.ErrMasterDataForbidden, http.StatusForbidden},
		{"unexpected", "", errors.New("pq: secret-detail"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, ts := mountVendorReadGroup(&fakeVendorAdminServicer{}, &fakePksListServicer{err: tt.err}, &fakeReminderServicer{})
			rec := doRequest(router, http.MethodGet, "/api/v1/admin/pks"+tt.query, tokenForRole(t, ts, 1, "ADMIN"), "")
			if rec.Code != tt.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.want, rec.Body.String())
			}
			if tt.want == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "secret-detail") {
				t.Error("500 leaks error detail")
			}
		})
	}
}

func TestAdminPksHandler_Reminder(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		err  error
		want int
	}{
		{"ok", "/5/reminder", `{"renewal_days":120}`, nil, http.StatusOK},
		{"bad pks id", "/abc/reminder", `{"renewal_days":120}`, nil, http.StatusBadRequest},
		{"bad json", "/5/reminder", `{`, nil, http.StatusBadRequest},
		{"validation 422", "/5/reminder", `{"renewal_days":1}`, &service.ValidationError{Field: "renewal_days", Message: "x"}, http.StatusUnprocessableEntity},
		{"not found 404", "/5/reminder", `{"renewal_days":120}`, service.ErrVendorPksNotFound, http.StatusNotFound},
		{"forbidden from service", "/5/reminder", `{"renewal_days":120}`, service.ErrMasterDataForbidden, http.StatusForbidden},
		{"unexpected 500", "/5/reminder", `{"renewal_days":120}`, errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rem := &fakeReminderServicer{err: tt.err}
			router, ts := mountVendorReadGroup(&fakeVendorAdminServicer{}, &fakePksListServicer{}, rem)
			rec := doRequest(router, http.MethodPut, "/api/v1/admin/pks"+tt.path, tokenForRole(t, ts, 9, "ADMIN"), tt.body)
			if rec.Code != tt.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.want, rec.Body.String())
			}
			if tt.want == http.StatusOK {
				if rem.gotActor != 9 || rem.gotPks != 5 || rem.gotDays != 120 || rem.gotRole != "ADMIN" {
					t.Errorf("service got actor/pks/days/role = %d/%d/%d/%s", rem.gotActor, rem.gotPks, rem.gotDays, rem.gotRole)
				}
				if !strings.Contains(rec.Body.String(), `"pks"`) {
					t.Errorf("body = %s", rec.Body.String())
				}
			}
		})
	}
}

// Property 7: the role matrix over the vendorRead group.
func TestVendorReadGroup_RoleMatrix(t *testing.T) {
	type ep struct{ method, path, body string }
	reads := []ep{
		{http.MethodGet, "/api/v1/admin/vendors", ""},
		{http.MethodGet, "/api/v1/admin/vendors/1", ""},
		{http.MethodGet, "/api/v1/admin/pks", ""},
	}
	writes := []ep{
		{http.MethodPost, "/api/v1/admin/vendors", `{}`},
		{http.MethodPut, "/api/v1/admin/vendors/1", `{}`},
		{http.MethodPost, "/api/v1/admin/vendors/1/disable", `{}`},
		{http.MethodPost, "/api/v1/admin/vendors/1/enable", `{}`},
		{http.MethodPut, "/api/v1/admin/pks/5/reminder", `{"renewal_days":120}`},
	}
	roles := []struct {
		role        string
		read, write bool
	}{
		{"ADMIN", true, true}, {"ADMIN_PARAM", true, true}, {"ATM-SPV", true, false},
		{"APPACCESS", false, false}, {"ATM-USER", false, false}, {"BRANCH-ATM-SPV", false, false},
		{"ACM-SPV", false, false}, {"VENDOR-USER", false, false}, {"EXT-CLIENT", false, false},
	}
	all := append(append([]ep{}, reads...), writes...)
	for _, r := range roles {
		for _, e := range all {
			allowed := r.read
			if e.method != http.MethodGet {
				allowed = r.write
			}
			t.Run(r.role+" "+e.method+" "+e.path, func(t *testing.T) {
				vendors := &fakeVendorAdminServicer{getResult: &db.GetVendorAdminByIDRow{ID: 1, Code: "ACM", Name: "Acme"}}
				list, rem := &fakePksListServicer{}, &fakeReminderServicer{}
				router, ts := mountVendorReadGroup(vendors, list, rem)
				rec := doRequest(router, e.method, e.path, tokenForRole(t, ts, 1, r.role), e.body)
				if !allowed {
					if rec.Code != http.StatusForbidden {
						t.Fatalf("status %d, want 403", rec.Code)
					}
					if vendors.createCalled || vendors.updateCalled || vendors.disableCalled || vendors.enableCalled || rem.calls != 0 || list.calls != 0 {
						t.Error("a forbidden role reached a service")
					}
					return
				}
				if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
					t.Fatalf("allowed role got %d: %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
	router, _ := mountVendorReadGroup(&fakeVendorAdminServicer{}, &fakePksListServicer{}, &fakeReminderServicer{})
	for _, e := range all {
		if rec := doRequest(router, e.method, e.path, "", e.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token = %d, want 401", e.method, e.path, rec.Code)
		}
	}
}
