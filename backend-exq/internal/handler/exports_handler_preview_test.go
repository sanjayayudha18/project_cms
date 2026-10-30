package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

type stubPreview struct {
	page      exportjob.Preview
	err       error
	calls     int
	gotParams exportjob.Params
	gotAfter  int64
	gotLimit  int
}

func (f *stubPreview) Preview(_ context.Context, _ exportjob.Actor, p exportjob.Params, after int64, limit int) (exportjob.Preview, error) {
	f.calls++
	f.gotParams, f.gotAfter, f.gotLimit = p, after, limit
	return f.page, f.err
}

func newPreviewRouter(svc ExportsService, pv PreviewService, role string) http.Handler {
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
	r.Mount("/", h.Routes())
	return r
}

func samplePreview() exportjob.Preview {
	next := int64(12)
	return exportjob.Preview{
		Columns:   []string{"id", "code"},
		Rows:      [][]string{{"11", "A"}, {"12", "B"}},
		Total:     30,
		NextAfter: &next,
		Limit:     2,
	}
}

// previewPaths is one valid path per master route (POST path; preview = "/preview" + path).
var previewPaths = []string{
	"/vendors/3/vaults",
	"/vendors/3/branches/5/pics",
	"/vendors/3/package-prices",
	"/master-data/atms",
}

// Property 4: the preview route accepts or rejects a master-data request exactly like its POST twin.
func TestProperty4_PreviewValidationMatchesExport(t *testing.T) {
	ids := rapid.SampledFrom([]string{"1", "3", "0", "-1", "-99", "abc", "1.5", "99999999999999999999", "007"})
	statuses := rapid.SampledFrom([]string{"", "active", "disabled", "all", "ACTIVE", "zzz"})
	levels := rapid.SampledFrom([]string{"", "vendor", "branch", "atm", "all", "zzz"})
	branchQ := rapid.SampledFrom([]string{"", "5", "0", "-2", "x"})
	vendorEnt := rapid.SampledFrom([]string{"vendor", "branches", "vaults", "pics", "dsr-location-maps", "nope", "atms"})
	branchEnt := rapid.SampledFrom([]string{"vaults", "pics", "dsr-location-maps", "branches", "nope"})
	globalEnt := rapid.SampledFrom([]string{"atm-assignments", "atms", "vendor-branches", "vendor-pics", "vendor-vaults", "vendors", "nope", "vendor"})

	rapid.Check(t, func(rt *rapid.T) {
		var path string
		q := url.Values{}
		switch rapid.IntRange(0, 3).Draw(rt, "route") {
		case 0:
			path = fmt.Sprintf("/vendors/%s/%s", ids.Draw(rt, "vid"), vendorEnt.Draw(rt, "ent"))
		case 1:
			path = fmt.Sprintf("/vendors/%s/branches/%s/%s", ids.Draw(rt, "vid"), ids.Draw(rt, "bid"), branchEnt.Draw(rt, "ent"))
		case 2:
			path = fmt.Sprintf("/vendors/%s/package-prices", ids.Draw(rt, "vid"))
			if v := branchQ.Draw(rt, "branch_id"); v != "" {
				q.Set("branch_id", v)
			}
			if v := levels.Draw(rt, "price_level"); v != "" {
				q.Set("price_level", v)
			}
		default:
			path = "/master-data/" + globalEnt.Draw(rt, "ent")
		}
		if v := statuses.Draw(rt, "status"); v != "" {
			q.Set("status", v)
		}
		target := path
		if len(q) > 0 {
			target += "?" + q.Encode()
		}

		svc := &fakeSvc{created: true}
		pv := &stubPreview{page: samplePreview()}
		router := newPreviewRouter(svc, pv, "ADMIN")
		post := do(router, "POST", target)
		get := do(router, "GET", "/preview"+target)

		switch post.Code {
		case http.StatusAccepted:
			if get.Code != http.StatusOK {
				rt.Fatalf("%s: POST accepted but preview %d: %s", target, get.Code, get.Body)
			}
			if svc.gotParams == nil || !reflect.DeepEqual(*svc.gotParams, pv.gotParams) {
				rt.Fatalf("%s: params differ: export %+v preview %+v", target, svc.gotParams, pv.gotParams)
			}
		case http.StatusBadRequest, http.StatusNotFound:
			if get.Code != post.Code || errCode(t, get) != errCode(t, post) {
				rt.Fatalf("%s: POST %d %s vs preview %d %s", target, post.Code, post.Body, get.Code, get.Body)
			}
			if pv.calls != 0 {
				rt.Fatalf("%s: rejected request reached the previewer", target)
			}
		default:
			rt.Fatalf("%s: unexpected POST status %d: %s", target, post.Code, post.Body)
		}
	})
}

func TestPreview_Envelope(t *testing.T) {
	pv := &stubPreview{page: samplePreview()}
	rec := do(newPreviewRouter(&fakeSvc{}, pv, "ADMIN"), "GET", "/preview/master-data/atms?after=5&limit=10")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var b struct {
		Success bool `json:"success"`
		Data    struct {
			Columns   []string   `json:"columns"`
			Rows      [][]string `json:"rows"`
			Total     int64      `json:"total"`
			NextAfter *int64     `json:"next_after"`
			Limit     int        `json:"limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if !b.Success || len(b.Data.Columns) != 2 || len(b.Data.Rows) != 2 || b.Data.Total != 30 ||
		b.Data.NextAfter == nil || *b.Data.NextAfter != 12 || b.Data.Limit != 2 {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
	if pv.gotAfter != 5 || pv.gotLimit != 10 || pv.gotParams.Entity != "atms" {
		t.Fatalf("forwarded after=%d limit=%d params=%+v", pv.gotAfter, pv.gotLimit, pv.gotParams)
	}
}

func TestPreview_LastPageNextAfterNull(t *testing.T) {
	pv := &stubPreview{page: exportjob.Preview{Columns: []string{"id"}, Rows: [][]string{}, Limit: 50}}
	rec := do(newPreviewRouter(&fakeSvc{}, pv, "ADMIN"), "GET", "/preview/master-data/atms")
	if !strings.Contains(rec.Body.String(), `"next_after":null`) || pv.gotLimit != exportjob.DefaultPreviewLimit {
		t.Fatalf("body %s limit %d", rec.Body, pv.gotLimit)
	}
}

func TestPreview_GuardAndAuth(t *testing.T) {
	for _, path := range previewPaths {
		for _, c := range []struct {
			role string
			want int
		}{{"ATM-USER", 403}, {"VENDOR-USER", 403}, {"", 401}} {
			pv := &stubPreview{page: samplePreview()}
			rec := do(newPreviewRouter(&fakeSvc{}, pv, c.role), "GET", "/preview"+path)
			if rec.Code != c.want || pv.calls != 0 {
				t.Errorf("GET /preview%s as %q: got %d (calls %d), want %d", path, c.role, rec.Code, pv.calls, c.want)
			}
		}
	}
}

func TestPreview_Unavailable(t *testing.T) {
	cases := map[string]error{
		"bare":    exportjob.ErrUnavailable,
		"wrapped": fmt.Errorf("%w: count: conflict", exportjob.ErrUnavailable),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			rec := do(newPreviewRouter(&fakeSvc{}, &stubPreview{err: err}, "ADMIN"), "GET", "/preview/master-data/atms")
			if rec.Code != 503 || errCode(t, rec) != "preview_unavailable" {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
	rec := do(newPreviewRouter(&fakeSvc{}, &stubPreview{err: errors.New("boom secret")}, "ADMIN"), "GET", "/preview/master-data/atms")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestPreview_BadPaging(t *testing.T) {
	for _, qs := range []string{"after=-1", "after=x", "limit=0", "limit=101", "limit=x", "limit=-5", "after=1&limit=1000"} {
		pv := &stubPreview{page: samplePreview()}
		rec := do(newPreviewRouter(&fakeSvc{}, pv, "ADMIN"), "GET", "/preview/master-data/atms?"+qs)
		if rec.Code != 400 || errCode(t, rec) != "bad_request" || pv.calls != 0 {
			t.Errorf("?%s: got %d %s (calls %d)", qs, rec.Code, rec.Body, pv.calls)
		}
	}
}
