package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeExportServicer exercises HTTP wiring only; CSV shaping, paging and the
// injection guard are covered by internal/service/masterdata_export_test.go.
type fakeExportServicer struct {
	authErr error
	write   func(entity, status string, w io.Writer, flush func()) (int, error)

	gotEntity, gotStatus string
	writeCalled          bool
}

func (f *fakeExportServicer) Authorize(context.Context) error { return f.authErr }

func (f *fakeExportServicer) Write(_ context.Context, entity, status string, w io.Writer, flush func()) (int, error) {
	f.writeCalled, f.gotEntity, f.gotStatus = true, entity, status
	if f.write != nil {
		return f.write(entity, status, w, flush)
	}
	_, _ = io.WriteString(w, "id,code\n1,V1\n")
	flush()
	return 1, nil
}

func mountExportHandler(svc MasterDataExportServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/master-data/export", NewAdminMasterDataExportHandler(svc).Routes())
	return r, tokenSvc
}

func TestAdminMasterDataExportHandler_StreamsCSVWithDownloadHeaders(t *testing.T) {
	svc := &fakeExportServicer{}
	router, tokenSvc := mountExportHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/vendors", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if ct := h.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := h.Get("Content-Disposition"); !regexp.MustCompile(`^attachment; filename="master-data-vendors-\d{8}\.csv"$`).MatchString(cd) {
		t.Errorf("Content-Disposition = %q, want attachment with dated filename", cd)
	}
	if h.Get("Cache-Control") != "no-store" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("want no-store + nosniff (PII must not be cached/sniffed), got %v", h)
	}
	if rec.Body.String() != "id,code\n1,V1\n" {
		t.Errorf("body = %q", rec.Body.String())
	}
	if !rec.Flushed {
		t.Error("the handler must hand the exporter a working flush (streaming)")
	}
	if svc.gotEntity != "vendors" || svc.gotStatus != "active" {
		t.Errorf("exporter got entity=%q status=%q, want vendors/active (default)", svc.gotEntity, svc.gotStatus)
	}
}

func TestAdminMasterDataExportHandler_EveryEntityRouteAndStatusPassThrough(t *testing.T) {
	for _, entity := range []string{"vendors", "vendor-branches", "vendor-vaults", "vendor-pics", "atms", "atm-assignments"} {
		for _, status := range []string{"active", "disabled", "all"} {
			svc := &fakeExportServicer{}
			router, tokenSvc := mountExportHandler(svc)

			rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/"+entity+"?status="+status, tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM"), "")

			if rec.Code != http.StatusOK || svc.gotEntity != entity || svc.gotStatus != status {
				t.Errorf("%s?status=%s: code=%d entity=%q status=%q", entity, status, rec.Code, svc.gotEntity, svc.gotStatus)
			}
		}
	}
}

func TestAdminMasterDataExportHandler_RejectsBeforeStreaming(t *testing.T) {
	cases := []struct {
		name string
		path string
		svc  *fakeExportServicer
		role string
		want int
	}{
		{"unknown entity", "/api/v1/admin/master-data/export/users", &fakeExportServicer{}, "ADMIN", http.StatusNotFound},
		{"bad status", "/api/v1/admin/master-data/export/vendors?status=bogus", &fakeExportServicer{}, "ADMIN", http.StatusBadRequest},
		{"service-layer forbidden", "/api/v1/admin/master-data/export/vendors", &fakeExportServicer{authErr: service.ErrMasterDataForbidden}, "ADMIN", http.StatusForbidden},
		{"route-guard forbidden (wrong role)", "/api/v1/admin/master-data/export/vendors", &fakeExportServicer{}, "ATM-USER", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := mountExportHandler(tc.svc)

			rec := doRequest(router, http.MethodGet, tc.path, tokenForRole(t, tokenSvc, 1, tc.role), "")

			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
			if tc.svc.writeCalled {
				t.Error("nothing may be streamed on a rejected request")
			}
			if cd := rec.Header().Get("Content-Disposition"); cd != "" {
				t.Errorf("an error response must not carry a download header, got %q", cd)
			}
		})
	}
}

func TestAdminMasterDataExportHandler_Template_IsBOMPlusExportHeaderOnly(t *testing.T) {
	for _, entity := range []string{"vendors", "vendor-branches", "vendor-vaults", "vendor-pics", "atms", "atm-assignments"} {
		svc := &fakeExportServicer{}
		router, tokenSvc := mountExportHandler(svc)

		rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/"+entity+"/template", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", entity, rec.Code)
		}
		header, _ := service.MasterDataExportHeader(entity)
		if want := "\xEF\xBB\xBF" + strings.Join(header, ",") + "\n"; rec.Body.String() != want {
			t.Errorf("%s: body = %q, want %q (BOM + header, no data rows)", entity, rec.Body.String(), want)
		}
		if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="master-data-`+entity+`-template.csv"` {
			t.Errorf("%s: Content-Disposition = %q", entity, cd)
		}
		if svc.writeCalled {
			t.Errorf("%s: a template must not touch the data exporter", entity)
		}
	}
}

func TestAdminMasterDataExportHandler_Template_Rejections(t *testing.T) {
	router, tokenSvc := mountExportHandler(&fakeExportServicer{})
	if rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/users/template", tokenForRole(t, tokenSvc, 1, "ADMIN"), ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown entity: expected 404, got %d", rec.Code)
	}
	router, tokenSvc = mountExportHandler(&fakeExportServicer{authErr: service.ErrMasterDataForbidden})
	if rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/vendors/template", tokenForRole(t, tokenSvc, 1, "ADMIN"), ""); rec.Code != http.StatusForbidden {
		t.Errorf("service forbidden: expected 403, got %d", rec.Code)
	}
	if rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/vendors/template", tokenForRole(t, tokenSvc, 1, "ATM-USER"), ""); rec.Code != http.StatusForbidden {
		t.Errorf("route guard: expected 403, got %d", rec.Code)
	}
}

func TestAdminMasterDataExportHandler_Anonymous_401(t *testing.T) {
	router, _ := mountExportHandler(&fakeExportServicer{})
	if rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/vendors", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// Failure before any byte was sent (e.g. the first page query fails) must still
// produce a normal JSON error -- not a "download" of an error message.
func TestAdminMasterDataExportHandler_FailureBeforeFirstByte_JSON500(t *testing.T) {
	svc := &fakeExportServicer{write: func(string, string, io.Writer, func()) (int, error) {
		return 0, errors.New("replica down")
	}}
	router, tokenSvc := mountExportHandler(svc)

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/atms", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != "" {
		t.Errorf("Content-Disposition must be removed on an early error, got %q", cd)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") || strings.Contains(rec.Body.String(), "replica down") {
		t.Errorf("want a JSON error that does not leak the internal cause, got %q / %q", rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

// Failure after bytes were sent would otherwise leave a truncated file that
// looks complete: the handler must abort the connection instead.
func TestAdminMasterDataExportHandler_FailureMidStream_AbortsConnection(t *testing.T) {
	svc := &fakeExportServicer{write: func(_ string, _ string, w io.Writer, flush func()) (int, error) {
		_, _ = io.WriteString(w, "id,code\n1,V1\n")
		flush()
		return 1, errors.New("replica connection lost on page 2")
	}}
	router, tokenSvc := mountExportHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		doRequest(router, http.MethodGet, "/api/v1/admin/master-data/export/atm-assignments", token, "")
	}()

	if recovered != http.ErrAbortHandler {
		t.Fatalf("recovered = %v, want http.ErrAbortHandler (abort, never present a truncated CSV as complete)", recovered)
	}
}
