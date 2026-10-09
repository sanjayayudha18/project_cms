package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// detailFakeDsrRepo serves one upload (id 5) to vendor "Vendor BIJAK" only;
// the SQL-side vendor scoping is mimicked by the Vendor check.
type detailFakeDsrRepo struct {
	scopeFakeDsrRepo
}

func mustNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		panic(err)
	}
	return n
}

func (detailFakeDsrRepo) GetDsrUploadByIDForVendor(_ context.Context, arg db.GetDsrUploadByIDForVendorParams) (db.GetDsrUploadByIDForVendorRow, error) {
	if arg.ID != 5 || arg.Vendor != "Vendor BIJAK" {
		return db.GetDsrUploadByIDForVendorRow{}, pgx.ErrNoRows
	}
	return db.GetDsrUploadByIDForVendorRow{ID: 5, Filename: "dsr.xlsx", Vendor: arg.Vendor, DailyStatus: "completed", RencanaIsiStatus: "completed"}, nil
}

func (detailFakeDsrRepo) ListDsrDailyRows(context.Context, int64) ([]db.ListDsrDailyRowsRow, error) {
	return []db.ListDsrDailyRowsRow{{RowNo: 1, Section: "d0", Flow: "saldo_akhir", LineLabel: "SALDO AKHIR", LineTotalIdr: mustNumeric("1500000")}}, nil
}

func (detailFakeDsrRepo) ListDsrRencanaIsiRows(context.Context, int64) ([]db.ListDsrRencanaIsiRowsRow, error) {
	return []db.ListDsrRencanaIsiRowsRow{{RowNo: 1, AtmTerminalID: "T001", Fill100kIdr: mustNumeric("2000000")}}, nil
}

func (detailFakeDsrRepo) ListDsrUploadsByVendor(context.Context, db.ListDsrUploadsByVendorParams) ([]db.ListDsrUploadsByVendorRow, error) {
	return []db.ListDsrUploadsByVendorRow{{ID: 5, ReportDate: pgtype.Date{Time: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Valid: true}, DailyStatus: "completed", RencanaIsiStatus: "failed"}}, nil
}

func (detailFakeDsrRepo) CountDsrUploadsByVendor(context.Context, db.CountDsrUploadsByVendorParams) (int64, error) {
	return 1, nil
}

func mountDsrDetailHandler(t *testing.T) (http.Handler, *pkgauth.TokenService, *miniredis.Miniredis) {
	t.Helper()
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc := service.NewDsrService(detailFakeDsrRepo{scopeFakeDsrRepo{code: "BIJAK", name: "Vendor BIJAK"}},
		rdb, http.DefaultClient, t.TempDir(), "http://127.0.0.1:0", "")
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/dsr", NewDsrUploadHandler(svc).Routes())
	return r, tokenSvc, mr
}

// Money amounts are asserted as digits only, so the check survives the planned
// float -> decimal string switch (QC F7).
func TestDsrUploadHandler_DetailAndList(t *testing.T) {
	router, ts, _ := mountDsrDetailHandler(t)
	token := tokenForVendor(t, ts, 1, 7)

	rec := doRequest(router, http.MethodGet, "/api/v1/dsr/uploads/daily/5", token, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"daily_rows"`) || !strings.Contains(rec.Body.String(), "1500000") {
		t.Fatalf("daily = %d %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(router, http.MethodGet, "/api/v1/dsr/uploads/rencana-isi/5", token, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"T001"`) || !strings.Contains(rec.Body.String(), "2000000") {
		t.Fatalf("rencana isi = %d %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(router, http.MethodGet, "/api/v1/dsr/uploads?page=1&page_size=10", token, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) || !strings.Contains(rec.Body.String(), `"failed"`) {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}

	for path, want := range map[string]int{
		"/api/v1/dsr/uploads/daily/6":        http.StatusNotFound, // not this vendor's upload
		"/api/v1/dsr/uploads/rencana-isi/6":  http.StatusNotFound,
		"/api/v1/dsr/uploads/daily/abc":      http.StatusBadRequest,
		"/api/v1/dsr/uploads/rencana-isi/0":  http.StatusBadRequest,
		"/api/v1/dsr/uploads?page=x":         http.StatusBadRequest,
		"/api/v1/dsr/uploads?page_size=x":    http.StatusBadRequest,
		"/api/v1/dsr/uploads?page_size=1000": http.StatusBadRequest,
	} {
		if rec := doRequest(router, http.MethodGet, path, token, ""); rec.Code != want {
			t.Errorf("GET %s = %d, want %d: %s", path, rec.Code, want, rec.Body.String())
		}
	}
	if rec := doRequest(router, http.MethodGet, "/api/v1/dsr/uploads", tokenForRole(t, ts, 1, "ATM-USER"), ""); rec.Code != http.StatusForbidden {
		t.Errorf("non-vendor list = %d, want 403", rec.Code)
	}
}

func dsrUploadRequest(t *testing.T, token, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(content)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dsr/uploads", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestDsrUploadHandler_Upload(t *testing.T) {
	router, ts, mr := mountDsrDetailHandler(t)
	token := tokenForVendor(t, ts, 1, 7)
	upload := func(filename string, content []byte) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, dsrUploadRequest(t, token, filename, content))
		return rec
	}
	xlsx := append([]byte("PK\x03\x04"), make([]byte, 16)...)

	for _, tc := range []struct {
		name     string
		filename string
		content  []byte
		want     int
	}{
		{"missing file field", "", nil, http.StatusBadRequest},
		{"empty file", "dsr.xlsx", nil, http.StatusBadRequest},
		{"not a workbook", "dsr.xlsx", []byte("hello"), http.StatusBadRequest},
		// Valid magic reaches the dry-run, which fails against the unreachable ETL.
		{"dry-run unavailable", "dsr.xlsx", xlsx, http.StatusBadGateway},
	} {
		if rec := upload(tc.filename, tc.content); rec.Code != tc.want {
			t.Errorf("%s = %d, want %d: %s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}

	// 10 uploads per vendor per hour; the 11th is refused with Retry-After.
	mr.FlushAll()
	for i := 1; i <= 10; i++ {
		if rec := upload("", nil); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("upload %d rate limited too early", i)
		}
	}
	rec := upload("", nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "3600" {
		t.Fatalf("11th upload = %d (Retry-After %q), want 429 / 3600", rec.Code, rec.Header().Get("Retry-After"))
	}
}
