package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// HTTP wiring only; the rules + audit run against Postgres in holiday_admin_integration_test.go.
type fakeHolidays struct {
	err      error
	date     time.Time
	name     string
	imported string
}

func (f *fakeHolidays) List(context.Context, service.Actor, *int32, string, int, int) (*service.HolidayList, error) {
	return &service.HolidayList{Items: []service.HolidayView{}}, f.err
}
func (f *fakeHolidays) Create(_ context.Context, _ service.Actor, d time.Time, name string) (*service.HolidayView, error) {
	f.date, f.name = d, name
	return &service.HolidayView{ID: 1, Name: name, IsActive: true}, f.err
}
func (f *fakeHolidays) UpdateName(_ context.Context, _ service.Actor, _ int64, name string) (*service.HolidayView, error) {
	f.name = name
	return &service.HolidayView{ID: 1, Name: name}, f.err
}
func (f *fakeHolidays) SetActive(context.Context, service.Actor, int64, bool) (*service.HolidayView, error) {
	return &service.HolidayView{ID: 1}, f.err
}
func (f *fakeHolidays) Import(_ context.Context, _ service.Actor, r io.Reader) (*service.HolidayImportResult, error) {
	b, _ := io.ReadAll(r)
	f.imported = string(b)
	return &service.HolidayImportResult{Created: 1, Skipped: []string{}}, f.err
}

func TestAdminHolidayHandler(t *testing.T) {
	const base = "/api/v1/admin/holidays"
	cases := []struct {
		name, method, path, body, role string
		svcErr                         error
		wantCode                       int
	}{
		{"list", http.MethodGet, base + "?year=2026&status=active", "", "ADMIN", nil, http.StatusOK},
		{"non-admin denied at route", http.MethodGet, base, "", "ADMIN_PARAM", nil, http.StatusForbidden},
		{"bad year", http.MethodGet, base + "?year=26", "", "ADMIN", nil, http.StatusUnprocessableEntity},
		{"create", http.MethodPost, base, `{"holiday_date":"2026-12-25","name":"Natal"}`, "ADMIN", nil, http.StatusCreated},
		{"create bad date", http.MethodPost, base, `{"holiday_date":"25-12-2026","name":"Natal"}`, "ADMIN", nil, http.StatusUnprocessableEntity},
		{"create duplicate", http.MethodPost, base, `{"holiday_date":"2026-12-25","name":"Natal"}`, "ADMIN", service.ErrHolidayDateConflict, http.StatusConflict},
		{"update", http.MethodPut, base + "/1", `{"name":"Natal 2026"}`, "ADMIN", nil, http.StatusOK},
		{"update missing", http.MethodPut, base + "/9", `{"name":"x"}`, "ADMIN", service.ErrHolidayNotFound, http.StatusNotFound},
		{"disable twice", http.MethodPost, base + "/1/disable", "", "ADMIN", service.ErrHolidayStatusUnchanged, http.StatusConflict},
		{"enable bad id", http.MethodPost, base + "/x/enable", "", "ADMIN", nil, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			fake := &fakeHolidays{err: tc.svcErr}
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).Mount(base, NewAdminHolidayHandler(fake).Routes())
			rec := doRequest(r, tc.method, tc.path, tokenForRole(t, ts, 1, tc.role), tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.name == "create" && (fake.date.Format("2006-01-02") != "2026-12-25" || fake.name != "Natal") {
				t.Fatalf("service got %v %q", fake.date, fake.name)
			}
		})
	}
}

func TestAdminHolidayHandler_Import(t *testing.T) {
	ts := testTokenService()
	fake := &fakeHolidays{}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(ts)).Mount("/h", NewAdminHolidayHandler(fake).Routes())

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "libur.csv")
	_, _ = fw.Write([]byte("holiday_date,name\n2026-12-25,Natal\n"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/h/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tokenForRole(t, ts, 1, "ADMIN"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(fake.imported, "2026-12-25,Natal") {
		t.Fatalf("code = %d body %s, service got %q", rec.Code, rec.Body.String(), fake.imported)
	}

	// no file field -> 422
	if rec := doRequest(r, http.MethodPost, "/h/import", tokenForRole(t, ts, 1, "ADMIN"), "x"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no file: code = %d", rec.Code)
	}
}
