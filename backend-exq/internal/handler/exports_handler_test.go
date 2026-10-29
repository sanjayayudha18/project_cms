package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeSvc struct {
	createJob     exportjob.Job
	created       bool
	err           error
	startJob      exportjob.Job
	startErr      error
	chunks        [][]byte
	chunkErrAt    int // -1 none; else yield error at this index
	completeErr   error
	gotParams     *exportjob.Params
	gotActor      exportjob.Actor
	gotLimit      int
	createCalls   int
	completeCalls int
	abortCalls    int
}

func (f *fakeSvc) Create(_ context.Context, a exportjob.Actor, p exportjob.Params) (exportjob.Job, bool, error) {
	f.createCalls++
	f.gotActor, f.gotParams = a, &p
	return f.createJob, f.created, f.err
}
func (f *fakeSvc) List(_ context.Context, a exportjob.Actor, limit int) ([]exportjob.Job, error) {
	f.gotActor, f.gotLimit = a, limit
	return []exportjob.Job{f.createJob}, f.err
}
func (f *fakeSvc) Get(_ context.Context, _ exportjob.Actor, _ int64) (exportjob.Job, error) {
	return f.createJob, f.err
}
func (f *fakeSvc) StartDownload(_ context.Context, _ exportjob.Actor, _ int64) (exportjob.Job, error) {
	return f.startJob, f.startErr
}
func (f *fakeSvc) OpenChunks(_ context.Context, _ int64, _ int64) exportjob.ChunkIterator {
	return func(yield func([]byte, error) bool) {
		for i, c := range f.chunks {
			if f.chunkErrAt == i {
				yield(nil, errors.New("db down"))
				return
			}
			if !yield(c, nil) {
				return
			}
		}
	}
}
func (f *fakeSvc) CompleteDownload(context.Context, int64) error {
	f.completeCalls++
	return f.completeErr
}
func (f *fakeSvc) AbortDownload(context.Context, int64) error { f.abortCalls++; return nil }

func newRouter(f *fakeSvc, role string) http.Handler {
	h := NewExportsHandler(f)
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
	r.Mount("/", h.Routes())
	r.Get("/jobs/{jobID}/file", h.Download)
	return r
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = "10.1.2.3:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var b struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body not json: %v: %s", err, rec.Body.String())
	}
	return b.Error.Code
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: x", exportjob.ErrValidation), 400, "bad_request"},
		{exportjob.ErrUnauthenticated, 401, "unauthorized"},
		{exportjob.ErrForbidden, 403, "forbidden"},
		{exportjob.ErrUnknownEntity, 404, "not_found"},
		{exportjob.ErrNotFound, 404, "not_found"},
		{exportjob.ErrNotReady, 409, "export_not_ready"},
		{exportjob.ErrGone, 410, "export_gone"},
		{exportjob.ErrTooManyActive, 429, "too_many_exports"},
		{fmt.Errorf("%w: secret sql", exportjob.ErrInternal), 500, "internal_error"},
		{errors.New("boom secret"), 500, "internal_error"},
	}
	for _, c := range cases {
		t.Run(c.code+"_"+c.err.Error(), func(t *testing.T) {
			f := &fakeSvc{err: c.err, startErr: c.err}
			for _, target := range []struct{ m, p string }{
				{"POST", "/master-data/vendors"}, {"GET", "/jobs"}, {"GET", "/jobs/3"}, {"GET", "/jobs/3/file"},
			} {
				rec := do(newRouter(f, "ADMIN"), target.m, target.p)
				if rec.Code != c.status || errCode(t, rec) != c.code {
					t.Fatalf("%s %s: got %d %s want %d %s", target.m, target.p, rec.Code, rec.Body, c.status, c.code)
				}
				if strings.Contains(rec.Body.String(), "secret") {
					t.Fatalf("cause leaked: %s", rec.Body)
				}
			}
		})
	}
	rec := do(newRouter(&fakeSvc{err: exportjob.ErrTooManyActive}, "ADMIN"), "POST", "/master-data/vendors")
	if !strings.Contains(rec.Body.String(), "Maksimal 3 ekspor berjalan bersamaan. Tunggu salah satu selesai.") {
		t.Fatalf("429 message: %s", rec.Body)
	}
}

func TestCreateStatusAndJobJSON(t *testing.T) {
	bs := int64(10)
	job := exportjob.Job{
		ID: 5, Params: exportjob.Params{Kind: exportjob.KindMasterDataGlobal, Entity: "vendor", StatusFilter: "active"},
		FileName: "f.csv", Status: exportjob.StatusQueued, ByteSize: &bs,
		CreatedAt: time.Date(2026, 10, 28, 7, 0, 0, 0, time.FixedZone("WIB", 7*3600)),
	}
	for _, created := range []bool{true, false} {
		f := &fakeSvc{createJob: job, created: created}
		rec := do(newRouter(f, "ADMIN_PARAM"), "POST", "/master-data/vendors")
		want := 200
		if created {
			want = 202
		}
		if rec.Code != want {
			t.Fatalf("created=%v got %d", created, rec.Code)
		}
		var b struct {
			Success bool
			Data    map[string]any
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil || !b.Success {
			t.Fatalf("envelope: %v %s", err, rec.Body)
		}
		if b.Data["created_at"] != "2026-10-28T00:00:00Z" || b.Data["vendor_id"] != nil || b.Data["started_at"] != nil || b.Data["byte_size"] != float64(10) {
			t.Fatalf("job json: %v", b.Data)
		}
		if f.gotActor.UserID != 7 || f.gotActor.Username != "adm" || f.gotActor.Role != "ADMIN_PARAM" || f.gotActor.IP != "10.1.2.3" {
			t.Fatalf("actor: %+v", f.gotActor)
		}
	}
}

func TestParamsPassThroughAndValidation(t *testing.T) {
	f := &fakeSvc{created: true}
	h := newRouter(f, "ADMIN")
	if rec := do(h, "POST", "/vendors/4/package-prices?status=all&branch_id=9&price_level=atm"); rec.Code != 202 {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	p := f.gotParams
	if p.Kind != exportjob.KindHargaPaket || *p.VendorID != 4 || *p.BranchID != 9 || p.StatusFilter != "all" || *p.PriceLevel != "atm" {
		t.Fatalf("params: %+v", p)
	}
	f.createCalls = 0
	for _, target := range []string{
		"/vendors/4/package-prices?price_level=bogus",
		"/vendors/4/package-prices?branch_id=0",
		"/vendors/4/package-prices?branch_id=x",
		"/vendors/4/vendor?status=bogus",
		"/vendors/0/vendor", "/vendors/-1/vendor", "/vendors/abc/vendor",
		"/vendors/4/branches/0/vaults", "/vendors/4/branches/x/vaults",
	} {
		rec := do(h, "POST", target)
		if rec.Code != 400 || errCode(t, rec) != "bad_request" {
			t.Fatalf("%s: got %d %s", target, rec.Code, rec.Body)
		}
	}
	if f.createCalls != 0 {
		t.Fatalf("Create called %d times for invalid input", f.createCalls)
	}
	for _, target := range []string{"/jobs/0", "/jobs/x", "/jobs/-2/file", "/jobs?limit=0", "/jobs?limit=51", "/jobs?limit=x"} {
		if rec := do(h, "GET", target); rec.Code != 400 {
			t.Fatalf("%s: got %d", target, rec.Code)
		}
	}
	if rec := do(h, "GET", "/jobs?limit=50"); rec.Code != 200 || f.gotLimit != 50 {
		t.Fatalf("limit: %d %d", rec.Code, f.gotLimit)
	}
	if do(h, "GET", "/jobs"); f.gotLimit != 20 {
		t.Fatalf("default limit %d", f.gotLimit)
	}
}

func TestRoutePrecedence(t *testing.T) {
	cases := []struct {
		target string
		kind   exportjob.Kind
		entity string
	}{
		{"/vendors/1/package-prices", exportjob.KindHargaPaket, "package-prices"},
		{"/vendors/1/branches/2/vaults", exportjob.KindCabang, "vaults"},
		{"/vendors/1/branches", exportjob.KindVendor, "branches"},
		{"/vendors/1/vault" + "s", exportjob.KindVendor, "vaults"},
		{"/master-data/atms", exportjob.KindMasterDataGlobal, "atms"},
	}
	for _, c := range cases {
		f := &fakeSvc{created: true}
		do(newRouter(f, "ADMIN"), "POST", c.target)
		if f.gotParams == nil || f.gotParams.Kind != c.kind || f.gotParams.Entity != c.entity {
			t.Fatalf("%s: params %+v", c.target, f.gotParams)
		}
	}
}

func TestTemplate(t *testing.T) {
	f := &fakeSvc{}
	rec := do(newRouter(f, "ADMIN"), "GET", "/master-data/vendors/template")
	if rec.Code != 200 {
		// entity names come from csvexport; find one dynamically below
		t.Logf("vendors not an entity: %d", rec.Code)
	}
	var entity string
	for _, e := range []string{"vendors", "vendor", "atms", "vendor-branches", "vaults"} {
		if r := do(newRouter(f, "ADMIN"), "GET", "/master-data/"+e+"/template"); r.Code == 200 {
			entity, rec = e, r
			break
		}
	}
	if entity == "" {
		t.Fatal("no importable entity found")
	}
	body := rec.Body.Bytes()
	if !bytes.HasPrefix(body, []byte("\xef\xbb\xbf")) || bytes.Count(body, []byte("\n")) != 1 {
		t.Fatalf("template body: %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("ct %s", ct)
	}
	if f.createCalls != 0 {
		t.Fatal("service called")
	}
	if rec := do(newRouter(f, "ADMIN"), "GET", "/master-data/nope/template"); rec.Code != 404 {
		t.Fatalf("unknown entity: %d", rec.Code)
	}
}

func TestRoleGuard(t *testing.T) {
	f := &fakeSvc{}
	h := newRouter(f, "ATM-USER")
	for _, c := range []struct{ m, p string }{
		{"POST", "/vendors/1/vendor"}, {"POST", "/vendors/1/branches/2/vaults"}, {"POST", "/vendors/1/package-prices"},
		{"POST", "/master-data/vendors"}, {"GET", "/master-data/vendors/template"},
	} {
		if rec := do(h, c.m, c.p); rec.Code != 403 {
			t.Fatalf("%s %s: got %d", c.m, c.p, rec.Code)
		}
	}
	if f.createCalls != 0 {
		t.Fatal("service reached")
	}
	for _, p := range []string{"/jobs", "/jobs/3"} {
		if rec := do(h, "GET", p); rec.Code != 200 {
			t.Fatalf("%s: got %d", p, rec.Code)
		}
	}
	if rec := do(newRouter(f, ""), "POST", "/master-data/vendors"); rec.Code != 401 {
		t.Fatalf("no auth: %d", rec.Code)
	}
}

func dlJob() exportjob.Job {
	bs, cc := int64(8), int64(2)
	return exportjob.Job{ID: 3, FileName: `a"b.csv`, ByteSize: &bs, ChunkCount: &cc}
}

func TestDownloadOK(t *testing.T) {
	f := &fakeSvc{startJob: dlJob(), chunks: [][]byte{[]byte("abcd"), []byte("efgh")}, chunkErrAt: -1}
	rec := do(newRouter(f, "ADMIN"), "GET", "/jobs/3/file")
	if rec.Code != 200 || rec.Body.String() != "abcdefgh" || f.completeCalls != 1 || f.abortCalls != 0 {
		t.Fatalf("got %d %q complete=%d abort=%d", rec.Code, rec.Body, f.completeCalls, f.abortCalls)
	}
	want := map[string]string{
		"Content-Type":           "text/csv; charset=utf-8",
		"Content-Disposition":    `attachment; filename="a\"b.csv"`,
		"Content-Length":         "8",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Fatalf("%s = %q want %q", k, got, v)
		}
	}
}

func TestDownloadCompleteFailureOnlyLogged(t *testing.T) {
	f := &fakeSvc{startJob: dlJob(), chunks: [][]byte{[]byte("abcd"), []byte("efgh")}, chunkErrAt: -1, completeErr: errors.New("x")}
	if rec := do(newRouter(f, "ADMIN"), "GET", "/jobs/3/file"); rec.Code != 200 || rec.Body.Len() != 8 {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestDownloadMidStreamAbort(t *testing.T) {
	f := &fakeSvc{startJob: dlJob(), chunks: [][]byte{[]byte("abcd"), []byte("efgh")}, chunkErrAt: 1}
	var rec any
	func() {
		defer func() { rec = recover() }()
		do(newRouter(f, "ADMIN"), "GET", "/jobs/3/file")
	}()
	if rec != http.ErrAbortHandler {
		t.Fatalf("recovered %v", rec)
	}
	if f.abortCalls != 1 || f.completeCalls != 0 {
		t.Fatalf("abort=%d complete=%d", f.abortCalls, f.completeCalls)
	}
}

func TestDownloadStartErrorIsJSON(t *testing.T) {
	f := &fakeSvc{startErr: exportjob.ErrGone, chunkErrAt: -1}
	rec := do(newRouter(f, "ADMIN"), "GET", "/jobs/3/file")
	if rec.Code != 410 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") || rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("got %d %v", rec.Code, rec.Header())
	}
}

func TestDownloadMissingMetadata(t *testing.T) {
	f := &fakeSvc{startJob: exportjob.Job{ID: 3}, chunkErrAt: -1}
	rec := do(newRouter(f, "ADMIN"), "GET", "/jobs/3/file")
	if rec.Code != 500 || f.abortCalls != 1 {
		t.Fatalf("got %d abort=%d", rec.Code, f.abortCalls)
	}
}
