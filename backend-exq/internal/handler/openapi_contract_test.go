package handler

// OpenAPI contract test (vendor-data-export task 10.2, Req 13.4, 15.1, 15.2):
// every route of ExportsHandler is in backend-exq/api/openapi.yaml and vice
// versa, and every response the handler writes validates against it.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend-exq/api"
	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

const exportsBase = "/api/v1/exports"

type contractSvc struct {
	job      exportjob.Job
	created  bool
	err      error
	startErr error
	chunks   [][]byte

	previewErr error
}

func (f *contractSvc) Create(context.Context, exportjob.Actor, exportjob.Params) (exportjob.Job, bool, error) {
	return f.job, f.created, f.err
}
func (f *contractSvc) List(context.Context, exportjob.Actor, int) ([]exportjob.Job, error) {
	return []exportjob.Job{f.job, f.job}, f.err
}
func (f *contractSvc) Get(context.Context, exportjob.Actor, int64) (exportjob.Job, error) {
	return f.job, f.err
}
func (f *contractSvc) StartDownload(context.Context, exportjob.Actor, int64) (exportjob.Job, error) {
	return f.job, f.startErr
}
func (f *contractSvc) OpenChunks(context.Context, int64, int64) exportjob.ChunkIterator {
	return func(yield func([]byte, error) bool) {
		for _, c := range f.chunks {
			if !yield(c, nil) {
				return
			}
		}
	}
}
func (f *contractSvc) CompleteDownload(context.Context, int64) error { return nil }
func (f *contractSvc) AbortDownload(context.Context, int64) error    { return nil }

// Preview makes contractSvc double as the PreviewService; previewErr drives the error cases.
func (f *contractSvc) Preview(context.Context, exportjob.Actor, exportjob.Params, int64, int) (exportjob.Preview, error) {
	if f.previewErr != nil {
		return exportjob.Preview{}, f.previewErr
	}
	next := int64(2)
	return exportjob.Preview{Columns: []string{"id", "code"}, Rows: [][]string{{"1", "A"}, {"2", "B"}}, Total: 5, NextAfter: &next, Limit: 2}, nil
}

// contractRouter mounts the handler exactly as cmd/api/main.go does.
func contractRouter(svc ExportsService, role string) http.Handler {
	pv, _ := svc.(PreviewService)
	h := NewExportsHandler(svc, pv)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if role != "" {
				ctx := middleware.WithAuthContext(req.Context(), &middleware.AuthContext{UserID: 7, Username: "adm", Role: role})
				req = req.WithContext(ctx)
			}
			next.ServeHTTP(w, req)
		})
	})
	r.Route(exportsBase, func(g chi.Router) {
		g.Mount("/", h.Routes())
		g.Get("/jobs/{jobID}/file", h.Download)
	})
	return r
}

func loadContract(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromData(api.Spec)
	if err != nil {
		t.Fatalf("load openapi.yaml: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi.yaml invalid: %v", err)
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("openapi router: %v", err)
	}
	return doc, router
}

func validateContract(router routers.Router, req *http.Request, status int, header http.Header, body []byte) error {
	route, params, err := router.FindRoute(req)
	if err != nil {
		return err
	}
	opts := &openapi3filter.Options{IncludeResponseStatus: true, AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}
	return openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route, Options: opts},
		Status:                 status,
		Header:                 header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                opts,
	})
}

func TestOpenAPIContract_SpecValid(t *testing.T) { loadContract(t) }

func TestOpenAPIContract_RouteCoverage(t *testing.T) {
	doc, _ := loadContract(t)
	var code []string
	err := chi.Walk(contractRouter(&contractSvc{}, "ADMIN").(chi.Router), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// Mount("/") makes chi.Walk report "/*" between the group prefix and the route.
		code = append(code, method+" "+strings.TrimSuffix(strings.ReplaceAll(route, "/*", ""), "/"))
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	var spec []string
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			spec = append(spec, method+" /api/v1"+path)
		}
	}
	sort.Strings(code)
	sort.Strings(spec)
	if strings.Join(code, "\n") != strings.Join(spec, "\n") {
		t.Errorf("routes and spec differ\nroutes:\n%s\n\nspec:\n%s", strings.Join(code, "\n"), strings.Join(spec, "\n"))
	}
}

func contractJob() exportjob.Job {
	vid, bid, size := int64(3), int64(5), int64(42)
	pl, errMsg := "all", "boom"
	now := time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)
	return exportjob.Job{
		ID: 9, Params: exportjob.Params{Kind: exportjob.KindHargaPaket, Entity: "package-prices", VendorID: &vid, BranchID: &bid, StatusFilter: "active", PriceLevel: &pl},
		VendorCode: "V1", BranchCode: "B1", FileName: "harga-paket-V1-20261029.csv", Status: exportjob.StatusSucceeded,
		RowCount: 2, ByteSize: &size, ErrorMessage: &errMsg, CreatedAt: now, StartedAt: &now, FinishedAt: &now, ExpiresAt: &now, DownloadedAt: &now,
	}
}

func contractReportJob() exportjob.Job {
	m, st := "2026-10", "late"
	return exportjob.Job{ID: 2, Params: exportjob.Params{Kind: exportjob.KindDsrLateDaily, Entity: "daily", Report: &exportjob.ReportParams{Month: &m, Status: &st}},
		FileName: "dsr-late-daily-202610.csv", Status: exportjob.StatusQueued, CreatedAt: time.Now()}
}

func contractQueuedJob() exportjob.Job {
	return exportjob.Job{ID: 1, Params: exportjob.Params{Kind: exportjob.KindMasterDataGlobal, Entity: "atms", StatusFilter: "active"},
		FileName: "master-data-atms-20261029.csv", Status: exportjob.StatusQueued, CreatedAt: time.Now()}
}

func TestOpenAPIContract_Responses(t *testing.T) {
	doc, router := loadContract(t)
	csvJob := contractJob()
	chunkCount := int64(2)
	csvJob.ChunkCount = &chunkCount
	size := int64(8)
	csvJob.ByteSize = &size
	ok := &contractSvc{job: contractJob(), created: true}
	okCSV := &contractSvc{job: csvJob, chunks: [][]byte{[]byte("a,b\n"), []byte("1,2\n")}}

	type tc struct {
		name, method, path, role string
		svc                      *contractSvc
		want                     int
	}
	cases := []tc{
		{"vendor 202", "POST", "/vendors/3/branches", "ADMIN", ok, 202},
		{"vendor 200 existing", "POST", "/vendors/3/vaults?status=all", "ADMIN", &contractSvc{job: contractQueuedJob()}, 200},
		{"vendor 400 id", "POST", "/vendors/x/vaults", "ADMIN", ok, 400},
		{"vendor 401 no token", "POST", "/vendors/3/vaults", "", ok, 401},
		{"vendor 403 role", "POST", "/vendors/3/vaults", "VENDOR-USER", ok, 403},
		{"vendor 403 service", "POST", "/vendors/3/vaults", "ADMIN", &contractSvc{err: exportjob.ErrForbidden}, 403},
		{"vendor 401 service", "POST", "/vendors/3/vaults", "ADMIN", &contractSvc{err: exportjob.ErrUnauthenticated}, 401},
		{"vendor 404 entity", "POST", "/vendors/3/nope", "ADMIN", ok, 404},
		{"vendor 429", "POST", "/vendors/3/vaults", "ADMIN", &contractSvc{err: exportjob.ErrTooManyActive}, 429},
		{"vendor 500", "POST", "/vendors/3/vaults", "ADMIN", &contractSvc{err: errors.New("boom")}, 500},
		{"branch 202", "POST", "/vendors/3/branches/5/pics", "ADMIN", ok, 202},
		{"branch 400", "POST", "/vendors/3/branches/0/pics", "ADMIN", ok, 400},
		{"prices 202", "POST", "/vendors/3/package-prices?branch_id=5&price_level=atm", "ADMIN", ok, 202},
		{"prices 400", "POST", "/vendors/3/package-prices?branch_id=x", "ADMIN", ok, 400},
		{"global 202", "POST", "/master-data/atms", "ADMIN_PARAM", ok, 202},
		{"global 400 status", "POST", "/master-data/atms?status=zzz", "ADMIN", ok, 400},
		{"global 404", "POST", "/master-data/zzz", "ADMIN", ok, 404},
		{"report daily 202", "POST", "/reports/dsr-late-daily?month=2026-10&status=late", "ATM-USER", &contractSvc{job: contractReportJob(), created: true}, 202},
		{"report daily 200 existing", "POST", "/reports/dsr-late-daily?month=2026-10", "ADMIN", &contractSvc{job: contractReportJob()}, 200},
		{"report daily 400 month", "POST", "/reports/dsr-late-daily?month=nope", "ADMIN", ok, 400},
		{"report daily 401", "POST", "/reports/dsr-late-daily?month=2026-10", "", ok, 401},
		{"report daily 403", "POST", "/reports/dsr-late-daily?month=2026-10", "BRANCH-ATM-USER", ok, 403},
		{"report daily 429", "POST", "/reports/dsr-late-daily?month=2026-10", "ADMIN", &contractSvc{err: exportjob.ErrTooManyActive}, 429},
		{"report daily 500", "POST", "/reports/dsr-late-daily?month=2026-10", "ADMIN", &contractSvc{err: errors.New("boom")}, 500},
		{"report summary 202", "POST", "/reports/dsr-late-summary?month=2026-10", "ATM-SPV", ok, 202},
		{"report mismatch 202", "POST", "/reports/replenishment-amount-mismatch?date_from=2026-10-01&date_to=2026-10-07", "BRANCH-ATM-USER", ok, 202},
		{"report mismatch 400 range", "POST", "/reports/replenishment-amount-mismatch?date_from=2026-09-01&date_to=2026-10-07", "ADMIN", ok, 400},
		{"report trips 202", "POST", "/reports/replenishment-trips?date_from=2026-10-01&date_to=2026-10-07", "ATM-USER", ok, 202},
		{"report trips 403", "POST", "/reports/replenishment-trips?date_from=2026-10-01&date_to=2026-10-07", "ADMIN_PARAM", ok, 403},
		{"preview vendor 200", "GET", "/preview/vendors/3/vaults?after=1&limit=2", "ADMIN", ok, 200},
		{"preview vendor 400 id", "GET", "/preview/vendors/x/vaults", "ADMIN", ok, 400},
		{"preview vendor 400 paging", "GET", "/preview/vendors/3/vaults?limit=0", "ADMIN", ok, 400},
		{"preview vendor 401", "GET", "/preview/vendors/3/vaults", "", ok, 401},
		{"preview vendor 403", "GET", "/preview/vendors/3/vaults", "VENDOR-USER", ok, 403},
		{"preview vendor 404", "GET", "/preview/vendors/3/nope", "ADMIN", ok, 404},
		{"preview vendor 500", "GET", "/preview/vendors/3/vaults", "ADMIN", &contractSvc{previewErr: errors.New("x")}, 500},
		{"preview vendor 503", "GET", "/preview/vendors/3/vaults", "ADMIN", &contractSvc{previewErr: exportjob.ErrUnavailable}, 503},
		{"preview branch 200", "GET", "/preview/vendors/3/branches/5/pics", "ADMIN", ok, 200},
		{"preview branch 400", "GET", "/preview/vendors/3/branches/0/pics", "ADMIN", ok, 400},
		{"preview prices 200", "GET", "/preview/vendors/3/package-prices?branch_id=5&price_level=atm", "ADMIN", ok, 200},
		{"preview prices 400", "GET", "/preview/vendors/3/package-prices?branch_id=x", "ADMIN", ok, 400},
		{"preview global 200", "GET", "/preview/master-data/atms", "ADMIN_PARAM", ok, 200},
		{"preview global 400 status", "GET", "/preview/master-data/atms?status=zzz", "ADMIN", ok, 400},
		{"preview global 404", "GET", "/preview/master-data/zzz", "ADMIN", ok, 404},
		{"template 200", "GET", "/master-data/atms/template", "ADMIN", ok, 200},
		{"template 404", "GET", "/master-data/zzz/template", "ADMIN", ok, 404},
		{"template 401", "GET", "/master-data/atms/template", "", ok, 401},
		{"template 403", "GET", "/master-data/atms/template", "VENDOR-USER", ok, 403},
		{"list 200", "GET", "/jobs?limit=5", "ADMIN", ok, 200},
		{"list 400", "GET", "/jobs?limit=99", "ADMIN", ok, 400},
		{"list 500", "GET", "/jobs", "ADMIN", &contractSvc{err: errors.New("x")}, 500},
		{"get 200", "GET", "/jobs/9", "ADMIN", ok, 200},
		{"get 400", "GET", "/jobs/x", "ADMIN", ok, 400},
		{"get 404", "GET", "/jobs/9", "ADMIN", &contractSvc{err: exportjob.ErrNotFound}, 404},
		{"file 200", "GET", "/jobs/9/file", "ADMIN", okCSV, 200},
		{"file 400", "GET", "/jobs/x/file", "ADMIN", ok, 400},
		{"file 403", "GET", "/jobs/9/file", "ADMIN", &contractSvc{startErr: exportjob.ErrForbidden}, 403},
		{"file 404", "GET", "/jobs/9/file", "ADMIN", &contractSvc{startErr: exportjob.ErrNotFound}, 404},
		{"file 409", "GET", "/jobs/9/file", "ADMIN", &contractSvc{startErr: exportjob.ErrNotReady}, 409},
		{"file 410", "GET", "/jobs/9/file", "ADMIN", &contractSvc{startErr: exportjob.ErrGone}, 410},
		{"file 500", "GET", "/jobs/9/file", "ADMIN", &contractSvc{startErr: errors.New("x")}, 500},
	}
	covered := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, exportsBase+c.path, nil)
			req.RemoteAddr = "10.1.2.3:5555"
			rec := httptest.NewRecorder()
			contractRouter(c.svc, c.role).ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if route, _, err := router.FindRoute(req); err == nil {
				covered[fmt.Sprintf("%s %s", route.Method, route.Path)] = true
			}
			if err := validateContract(router, req, rec.Code, rec.Header(), rec.Body.Bytes()); err != nil {
				t.Errorf("response does not match openapi.yaml: %v\nbody: %s", err, rec.Body.String())
			}
		})
	}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			if !covered[method+" "+path] && !covered[method+" /api/v1"+path] {
				t.Errorf("no validated response for %s %s", method, path)
			}
		}
	}
}
