package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeImportServicer struct {
	res *service.ImportResult
	err error

	confirmRes *service.ImportConfirmResult
	gotMaker   int64

	gotEntity, gotBody string
}

func (f *fakeImportServicer) Confirm(_ context.Context, makerID int64, entity string, r io.Reader, _ string) (*service.ImportConfirmResult, error) {
	b, _ := io.ReadAll(r)
	f.gotMaker, f.gotEntity, f.gotBody = makerID, entity, string(b)
	return f.confirmRes, f.err
}

func (f *fakeImportServicer) DryRun(_ context.Context, entity string, r io.Reader) (*service.ImportResult, error) {
	b, _ := io.ReadAll(r)
	f.gotEntity, f.gotBody = entity, string(b)
	return f.res, f.err
}

func importRouter(svc MasterDataImportServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/master-data/import", NewAdminMasterDataImportHandler(svc).Routes())
	return r, tokenSvc
}

// upload POSTs a multipart body with the CSV under field (empty field = no file part).
func upload(router http.Handler, path, token, field, content string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if field != "" {
		fw, _ := mw.CreateFormFile(field, "vendors.csv")
		_, _ = fw.Write([]byte(content))
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

const importPath = "/api/v1/admin/master-data/import/vendors/dry-run"

func TestAdminMasterDataImportHandler_DryRun_ReturnsPreview(t *testing.T) {
	svc := &fakeImportServicer{res: &service.ImportResult{Preview: service.ImportPreview{
		Entity: "vendors", TotalRows: 2, ValidRows: 1, Creates: 1,
		Errors: []service.ImportRowError{{Row: 3, Field: "name", Message: "wajib diisi"}},
	}}}
	router, tokenSvc := importRouter(svc)

	rec := upload(router, importPath, tokenForRole(t, tokenSvc, 1, "ADMIN"), "file", "id,code\n")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		ValidRows int `json:"valid_rows"`
		Errors    []struct {
			Row     int    `json:"row"`
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ValidRows != 1 || len(got.Errors) != 1 || got.Errors[0].Row != 3 || got.Errors[0].Field != "name" {
		t.Errorf("preview shape = %s", rec.Body.String())
	}
	if svc.gotEntity != "vendors" || svc.gotBody != "id,code\n" {
		t.Errorf("service got entity=%q body=%q", svc.gotEntity, svc.gotBody)
	}
}

func TestAdminMasterDataImportHandler_DryRun_Rejections(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		field string
		svc   *fakeImportServicer
		role  string
		want  int
	}{
		{"unknown entity", "/api/v1/admin/master-data/import/users/dry-run", "file", &fakeImportServicer{}, "ADMIN", http.StatusNotFound},
		{"no file part", importPath, "", &fakeImportServicer{}, "ADMIN", http.StatusUnprocessableEntity},
		{"wrong field name", importPath, "upload", &fakeImportServicer{}, "ADMIN", http.StatusUnprocessableEntity},
		{"service forbidden", importPath, "file", &fakeImportServicer{err: service.ErrMasterDataForbidden}, "ADMIN", http.StatusForbidden},
		{"route guard forbidden", importPath, "file", &fakeImportServicer{}, "ATM-USER", http.StatusForbidden},
		{"oversize/too many rows", importPath, "file", &fakeImportServicer{err: &service.ValidationError{Field: "file", Message: "terlalu besar"}}, "ADMIN", http.StatusUnprocessableEntity},
		{"internal error", importPath, "file", &fakeImportServicer{err: errors.New("db down")}, "ADMIN", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := importRouter(tc.svc)

			rec := upload(router, tc.path, tokenForRole(t, tokenSvc, 1, tc.role), tc.field, "id,code\n")

			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "db down") {
				t.Errorf("internal cause leaked: %s", rec.Body.String())
			}
		})
	}
}

func TestAdminMasterDataImportHandler_Anonymous_401(t *testing.T) {
	router, _ := importRouter(&fakeImportServicer{})
	if rec := upload(router, importPath, "", "file", "x"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAdminMasterDataImportHandler_OversizedBody_413(t *testing.T) {
	router, tokenSvc := importRouter(&fakeImportServicer{})
	big := strings.Repeat("a", service.MasterDataImportMaxBytes+multipartOverhead+1)

	rec := upload(router, importPath, tokenForRole(t, tokenSvc, 1, "ADMIN"), "file", big)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

const confirmPath = "/api/v1/admin/master-data/import/vendors/confirm"

func TestAdminMasterDataImportHandler_Confirm_202ThenExisting200(t *testing.T) {
	approval := int64(9)
	for _, tc := range []struct {
		existing bool
		want     int
	}{{false, http.StatusAccepted}, {true, http.StatusOK}} {
		svc := &fakeImportServicer{confirmRes: &service.ImportConfirmResult{BatchID: 4, Rows: 3, ApprovalRequestID: &approval, Existing: tc.existing}}
		router, tokenSvc := importRouter(svc)

		rec := upload(router, confirmPath, tokenForRole(t, tokenSvc, 7, "ADMIN"), "file", "id,code\n")

		if rec.Code != tc.want {
			t.Fatalf("existing=%v: expected %d, got %d: %s", tc.existing, tc.want, rec.Code, rec.Body.String())
		}
		var got map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["batch_id"] != float64(4) || got["existing"] != tc.existing || got["approval_request_id"] != float64(9) {
			t.Errorf("body = %s", rec.Body.String())
		}
		if svc.gotMaker != 7 || svc.gotEntity != "vendors" {
			t.Errorf("service got maker=%d entity=%q, want the token's user and the path entity", svc.gotMaker, svc.gotEntity)
		}
	}
}

func TestAdminMasterDataImportHandler_Confirm_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"row errors", &service.ImportInvalidError{Errors: []service.ImportRowError{{Row: 2, Field: "code", Message: "wajib diisi"}}}, http.StatusUnprocessableEntity},
		{"nothing to import", service.ErrImportNothingToDo, http.StatusUnprocessableEntity},
		{"pending conflict", fmt.Errorf("baris 5: %w", service.ErrMasterDataChangePending), http.StatusConflict},
		{"forbidden", service.ErrMasterDataForbidden, http.StatusForbidden},
		{"internal", errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, tokenSvc := importRouter(&fakeImportServicer{err: tc.err})

			rec := upload(router, confirmPath, tokenForRole(t, tokenSvc, 7, "ADMIN"), "file", "x")

			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
			if tc.name == "row errors" && !strings.Contains(rec.Body.String(), `"row":2`) {
				t.Errorf("422 must carry the row errors: %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "db down") {
				t.Errorf("internal cause leaked: %s", rec.Body.String())
			}
		})
	}
}

func TestAdminMasterDataImportHandler_Confirm_RouteGuard(t *testing.T) {
	router, tokenSvc := importRouter(&fakeImportServicer{})
	if rec := upload(router, confirmPath, tokenForRole(t, tokenSvc, 7, "ATM-USER"), "file", "x"); rec.Code != http.StatusForbidden {
		t.Errorf("wrong role: expected 403, got %d", rec.Code)
	}
	if rec := upload(router, confirmPath, "", "file", "x"); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: expected 401, got %d", rec.Code)
	}
}
