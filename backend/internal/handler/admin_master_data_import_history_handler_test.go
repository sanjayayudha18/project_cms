package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

const historyBase = "/api/v1/admin/master-data/import"

type fakeHistoryServicer struct {
	page service.ImportBatchPage
	det  service.ImportBatchDetail
	err  error

	listCalls, getCalls int
	gotFilter           repository.ImportBatchFilter
	gotPage, gotSize    int
	gotID               int64
}

func (f *fakeHistoryServicer) List(_ context.Context, fl repository.ImportBatchFilter, page, size int) (service.ImportBatchPage, error) {
	f.listCalls++
	f.gotFilter, f.gotPage, f.gotSize = fl, page, size
	return f.page, f.err
}

func (f *fakeHistoryServicer) Get(_ context.Context, id int64, page, size int) (service.ImportBatchDetail, error) {
	f.getCalls++
	f.gotID, f.gotPage, f.gotSize = id, page, size
	return f.det, f.err
}

// historyRouter mirrors the production guard (main.go masterDataAdmin:
// RequireAuth + RequireRoles ADMIN/ADMIN_PARAM). hs == nil leaves /batches
// unwired; rdb != nil turns the import rate limit on.
func historyRouter(hs MasterDataImportHistoryServicer, rdb redis.Cmdable) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	h := NewAdminMasterDataImportHandler(&fakeImportServicer{err: errors.New("x")})
	if hs != nil {
		h.WithHistory(hs)
	}
	if rdb != nil {
		h.WithRateLimit(rdb)
	}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), custommw.RequireRoles("ADMIN", "ADMIN_PARAM")).Mount(historyBase, h.Routes())
	return r, tokenSvc
}

func historyGET(router http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not a JSON object: %v: %s", err, rec.Body.String())
	}
	return m
}

func TestListBatches_PassesFiltersAndPaging(t *testing.T) {
	fake := &fakeHistoryServicer{}
	router, ts := historyRouter(fake, nil)

	rec := historyGET(router, historyBase+"/batches?entity=vendors&status=applied&mine=true&page=3&page_size=100", tokenForRole(t, ts, 42, "ADMIN"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	f := fake.gotFilter
	if f.Entity == nil || *f.Entity != "vendors" || f.Status == nil || *f.Status != "applied" {
		t.Errorf("filter = %+v", f)
	}
	if f.MakerID == nil || *f.MakerID != 42 {
		t.Errorf("mine=true must set MakerID to the caller (42), got %v", f.MakerID)
	}
	if fake.gotPage != 3 || fake.gotSize != 100 {
		t.Errorf("paging = %d/%d, want 3/100", fake.gotPage, fake.gotSize)
	}
}

func TestListBatches_DefaultsAndNoFilter(t *testing.T) {
	fake := &fakeHistoryServicer{}
	router, ts := historyRouter(fake, nil)

	if rec := historyGET(router, historyBase+"/batches", tokenForRole(t, ts, 1, "ADMIN_PARAM")); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	f := fake.gotFilter
	if f.Entity != nil || f.Status != nil || f.MakerID != nil {
		t.Errorf("no query must mean no filter, got %+v", f)
	}
	if fake.gotPage != 1 || fake.gotSize != 20 {
		t.Errorf("defaults = %d/%d, want 1/20", fake.gotPage, fake.gotSize)
	}
}

func TestListBatches_BadParameters400(t *testing.T) {
	tests := []struct {
		query, field string
	}{
		{"entity=nope", "entity"},
		{"status=bogus", "status"},
		{"mine=false", "mine"},
		{"mine=1", "mine"},
		{"page=0", "page"},
		{"page=abc", "page"},
		{"page_size=0", "page_size"},
		{"page_size=101", "page_size"},
		{"page_size=x", "page_size"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			fake := &fakeHistoryServicer{}
			router, ts := historyRouter(fake, nil)
			rec := historyGET(router, historyBase+"/batches?"+tt.query, tokenForRole(t, ts, 1, "ADMIN"))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"field":"`+tt.field+`"`) {
				t.Errorf("400 must name field %q: %s", tt.field, rec.Body.String())
			}
			if fake.listCalls != 0 {
				t.Errorf("service called %d times despite a bad parameter", fake.listCalls)
			}
		})
	}
}

func TestListBatches_JSONShape(t *testing.T) {
	router, ts := historyRouter(&fakeHistoryServicer{page: service.ImportBatchPage{Total: 0, Page: 1, PageSize: 20}}, nil)
	rec := historyGET(router, historyBase+"/batches", tokenForRole(t, ts, 1, "ADMIN"))
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("empty list must serialize items as [], got %s", rec.Body.String())
	}

	name := "vendors.csv"
	fake := &fakeHistoryServicer{page: service.ImportBatchPage{Total: 1, Page: 1, PageSize: 20, Items: []service.ImportBatchSummary{
		{ID: 9, Entity: "atms", FileName: &name, RowCount: 3, MakerID: 2, MakerName: "Siti", Status: "applied"},
	}}}
	router, ts = historyRouter(fake, nil)
	m := decodeMap(t, historyGET(router, historyBase+"/batches", tokenForRole(t, ts, 1, "ADMIN")))
	for _, k := range []string{"items", "total", "page", "page_size"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing top-level key %q", k)
		}
	}
	item := m["items"].([]any)[0].(map[string]any)
	for _, k := range []string{"id", "entity", "file_name", "row_count", "maker_id", "maker_name", "created_at", "status",
		"approval_request_id", "error", "decided_at", "decided_by_name"} {
		if _, ok := item[k]; !ok {
			t.Errorf("missing item key %q", k)
		}
	}
}

func TestGetBatch_Defaults_NotFound_BadID(t *testing.T) {
	fake := &fakeHistoryServicer{}
	router, ts := historyRouter(fake, nil)
	tok := tokenForRole(t, ts, 1, "ADMIN")

	if rec := historyGET(router, historyBase+"/batches/7", tok); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if fake.gotID != 7 || fake.gotPage != 1 || fake.gotSize != 50 {
		t.Errorf("got id %d page %d size %d, want 7/1/50", fake.gotID, fake.gotPage, fake.gotSize)
	}

	for _, id := range []string{"abc", "0", "-3", "1.5"} {
		if rec := historyGET(router, historyBase+"/batches/"+id, tok); rec.Code != http.StatusBadRequest {
			t.Errorf("id %q: status %d, want 400", id, rec.Code)
		}
	}
	for _, q := range []string{"page=0", "page_size=0", "page_size=2001"} {
		rec := historyGET(router, historyBase+"/batches/7?"+q, tok)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, rec.Code)
		}
	}

	nf := &fakeHistoryServicer{err: service.ErrImportBatchNotFound}
	router, ts = historyRouter(nf, nil)
	if rec := historyGET(router, historyBase+"/batches/99", tokenForRole(t, ts, 1, "ADMIN")); rec.Code != http.StatusNotFound {
		t.Errorf("unknown batch: status %d, want 404", rec.Code)
	}
}

func TestGetBatch_JSONShape(t *testing.T) {
	router, ts := historyRouter(&fakeHistoryServicer{det: service.ImportBatchDetail{
		ImportBatchSummary: service.ImportBatchSummary{ID: 5, Entity: "vendors", MakerID: 2, Status: "stale"},
		Page:               1, PageSize: 50,
	}}, nil)
	rec := historyGET(router, historyBase+"/batches/5", tokenForRole(t, ts, 1, "ADMIN"))
	if !strings.Contains(rec.Body.String(), `"changes":[]`) {
		t.Fatalf("no rows must serialize changes as [], got %s", rec.Body.String())
	}
	m := decodeMap(t, rec)
	for _, k := range []string{"id", "maker_id", "error", "status", "counts", "changes", "total", "page", "page_size"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}

	cause := "boom"
	router, ts = historyRouter(&fakeHistoryServicer{det: service.ImportBatchDetail{
		ImportBatchSummary: service.ImportBatchSummary{ID: 5, Error: &cause},
		Counts:             service.ImportBatchCounts{Create: 1},
		Changes:            []service.ImportBatchChange{{ID: 1, Op: "create", Error: &cause, Failed: true}},
		Total:              1,
	}}, nil)
	m = decodeMap(t, historyGET(router, historyBase+"/batches/5", tokenForRole(t, ts, 1, "ADMIN")))
	if m["error"] != "boom" {
		t.Errorf("error = %v", m["error"])
	}
	if c := m["counts"].(map[string]any); c["create"] != float64(1) {
		t.Errorf("counts = %v", c)
	}
	row := m["changes"].([]any)[0].(map[string]any)
	if row["failed"] != true {
		t.Errorf("failed flag missing/false: %v", row)
	}
}

func TestHistory_ServiceErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"forbidden at service layer", service.ErrMasterDataForbidden, http.StatusForbidden},
		{"internal", errors.New("pq: secret detail"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, ts := historyRouter(&fakeHistoryServicer{err: tt.err}, nil)
			for _, p := range []string{"/batches", "/batches/1"} {
				rec := historyGET(router, historyBase+p, tokenForRole(t, ts, 1, "ADMIN"))
				if rec.Code != tt.want {
					t.Errorf("%s: status %d, want %d", p, rec.Code, tt.want)
				}
				if strings.Contains(rec.Body.String(), "secret detail") {
					t.Errorf("%s: internal cause leaked: %s", p, rec.Body.String())
				}
			}
		})
	}
}

// Property 8 (history part), route level: the guard in front of the handler
// stops other roles before the service is reached. main.go builds the same
// guard (masterDataAdmin) as a local variable, so the production wiring itself
// is only exercisable in task 8's integration test.
func TestHistory_RouteGuard(t *testing.T) {
	fake := &fakeHistoryServicer{}
	router, ts := historyRouter(fake, nil)
	for _, p := range []string{"/batches", "/batches/1"} {
		for _, role := range []string{"ATM-USER", "ATM-SPV", "VENDOR-USER", "ACM-USER"} {
			if rec := historyGET(router, historyBase+p, tokenForRole(t, ts, 7, role)); rec.Code != http.StatusForbidden {
				t.Errorf("%s as %s: status %d, want 403", p, role, rec.Code)
			}
		}
		if rec := historyGET(router, historyBase+p, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s anonymous: status %d, want 401", p, rec.Code)
		}
	}
	if fake.listCalls+fake.getCalls != 0 {
		t.Errorf("service reached %d times through the guard", fake.listCalls+fake.getCalls)
	}
}

func TestHistory_NotWired404(t *testing.T) {
	router, ts := historyRouter(nil, nil)
	for _, p := range []string{"/batches", "/batches/1"} {
		if rec := historyGET(router, historyBase+p, tokenForRole(t, ts, 1, "ADMIN")); rec.Code != http.StatusNotFound {
			t.Errorf("%s without WithHistory: status %d, want 404", p, rec.Code)
		}
	}
}

func TestHistory_NotRateLimited(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router, ts := historyRouter(&fakeHistoryServicer{}, rdb)
	tok := tokenForRole(t, ts, 1, "ADMIN")

	for i := 0; i < 3*importRateLimit; i++ {
		for _, p := range []string{"/batches", "/batches/1"} {
			if rec := historyGET(router, historyBase+p, tok); rec.Code != http.StatusOK {
				t.Fatalf("call %d %s: status %d (history must never be limited)", i, p, rec.Code)
			}
		}
	}
	if keys := mr.Keys(); len(keys) != 0 {
		t.Fatalf("history calls touched the rate-limit counter: %v", keys)
	}

	// dry-run and confirm still share the one counter: 10 + 10 pass, the 21st is 429.
	post := func(path string) int {
		req := httptest.NewRequest(http.MethodPost, historyBase+path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < importRateLimit/2; i++ {
		if c := post("/vendors/dry-run"); c == http.StatusTooManyRequests {
			t.Fatalf("dry-run %d limited too early", i)
		}
		if c := post("/vendors/confirm"); c == http.StatusTooManyRequests {
			t.Fatalf("confirm %d limited too early", i)
		}
	}
	if c := post("/vendors/confirm"); c != http.StatusTooManyRequests {
		t.Errorf("confirm over the cap: status %d, want 429", c)
	}
	if c := post("/vendors/dry-run"); c != http.StatusTooManyRequests {
		t.Errorf("dry-run over the cap: status %d, want 429", c)
	}
	if c := historyGET(router, historyBase+"/batches", tok).Code; c != http.StatusOK {
		t.Errorf("history while import is limited: status %d, want 200", c)
	}
}
