package handler

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// report-export-migration 8.3, Property 11 (cutover): on the four report endpoints format=csv is a 400
// on field "format" for ANY other parameters and any allowed role (service never reached, never CSV);
// without format the JSON answer is exactly what the service returned.

const cutoverBase = "/api/v1/reports"

// cutoverTarget is one report endpoint plus the roles allowed through its route group.
type cutoverTarget struct {
	name, path string
	roles      []string
	mount      func(t *testing.T) (http.Handler, func() bool) // router + "service was reached"
}

func mountCutoverDsr() (http.Handler, *fakeDsrLateReader) {
	fake := &fakeDsrLateReader{}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(testTokenService())).Mount(cutoverBase, NewDsrLateHandler(fake).Routes())
	return r, fake
}

func mountCutoverRepl() (http.Handler, *fakeReplenishmentReader) {
	fake := &fakeReplenishmentReader{}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(testTokenService())).Mount(cutoverBase, NewReplenishmentHandler(fake).Routes())
	return r, fake
}

func cutoverTargets() []cutoverTarget {
	dsr := func(t *testing.T) (http.Handler, func() bool) {
		r, f := mountCutoverDsr()
		return r, func() bool { return !reflect.DeepEqual(f.filter, service.DsrLateFilter{}) }
	}
	repl := func(t *testing.T) (http.Handler, func() bool) {
		r, f := mountCutoverRepl()
		return r, func() bool { return f.calls != 0 || !reflect.DeepEqual(f.filter, service.ReplenishmentFilter{}) }
	}
	return []cutoverTarget{
		{"dsr daily", "/daily", reports.DsrLateViewerRoles, dsr},
		{"dsr summary", "/summary", reports.DsrLateViewerRoles, dsr},
		{"replenishment amount-mismatch", "/reports/amount-mismatch", reports.ReplenishmentViewerRoles, repl},
		{"replenishment trips", "/reports/trips", reports.ReplenishmentViewerRoles, repl},
	}
}

var (
	cutoverKeys = []string{"month", "date_from", "date_to", "vendor_id", "vendor_branch_id", "status", "classification",
		"only_diff", "page", "page_size", "Format", "formats", "x", "", "a b", "é"}
	cutoverVals = []string{"", "2026-10", "2026-10-01", "2026-10-14", "bad", "0", "-1", "1", "9999999999999999999", "late", "true",
		"csv", "CSV", "json", "<script>", "a&b=c", "%", "日本", "2099-01", strings.Repeat("z", 300)}
)

// randomCutoverQuery builds a random query string that always carries format=csv exactly once.
func randomCutoverQuery(rng *rand.Rand) string {
	q := url.Values{}
	for i, n := 0, rng.Intn(9); i < n; i++ {
		q.Add(cutoverKeys[rng.Intn(len(cutoverKeys))], cutoverVals[rng.Intn(len(cutoverVals))])
	}
	q.Set("format", "csv")
	return q.Encode()
}

func TestCutover_FormatCSVAlways400(t *testing.T) {
	rng := rand.New(rand.NewSource(11)) // seeded: reproducible
	const iterations = 40
	for _, tg := range cutoverTargets() {
		for _, role := range tg.roles {
			t.Run(tg.name+"/"+role, func(t *testing.T) {
				router, reached := tg.mount(t)
				token := tokenForRole(t, testTokenService(), 1, role)
				for i := 0; i < iterations; i++ {
					qs := randomCutoverQuery(rng)
					rec := doRequest(router, http.MethodGet, cutoverBase+tg.path+"?"+qs, token, "")
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("query %q: code = %d, want 400: %s", qs, rec.Code, rec.Body.String())
					}
					if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
						t.Fatalf("query %q: Content-Type = %q, want JSON", qs, ct)
					}
					var body ErrorResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
						t.Fatalf("query %q: body not JSON: %v", qs, err)
					}
					if body.Error != "validation_error" || len(body.Details) != 1 || body.Details[0].Field != "format" {
						t.Fatalf("query %q: body = %+v, want validation_error on field format", qs, body)
					}
					if reached() {
						t.Fatalf("query %q: service was reached", qs)
					}
				}
			})
		}
	}
}

// jsonEqual compares two JSON documents structurally.
func jsonEqual(t *testing.T, got []byte, want any) bool {
	t.Helper()
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v: %s", err, got)
	}
	if err := json.Unmarshal(wantRaw, &w); err != nil {
		t.Fatalf("want is not JSON: %v", err)
	}
	return reflect.DeepEqual(g, w)
}

// Without format (or a non-csv one) the body is exactly the service result, shape unchanged.
func TestCutover_WithoutFormatJSONUnchanged(t *testing.T) {
	const dsrQ = "month=2026-10&vendor_id=2&vendor_branch_id=3&status=late&page=2&page_size=10"
	const replQ = "date_from=2026-10-01&date_to=2026-10-14&vendor_id=2"
	ctx := t.Context()
	actor := service.Actor{}

	dsrFake := &fakeDsrLateReader{}
	daily, _ := dsrFake.Daily(ctx, actor, service.DsrLateFilter{})
	summary, _ := dsrFake.Summary(ctx, actor, service.DsrLateFilter{})
	replFake := &fakeReplenishmentReader{}
	mismatch, _ := replFake.AmountMismatch(ctx, actor, service.ReplenishmentFilter{})
	trips, _ := replFake.Trips(ctx, actor, time.Time{}, time.Time{}, nil)

	cases := []struct {
		name, path, role string
		want             any
	}{
		{"dsr daily", cutoverBase + "/daily?" + dsrQ, "ATM-USER", daily},
		{"dsr summary", cutoverBase + "/summary?" + dsrQ, "ADMIN", summary},
		{"amount-mismatch", cutoverBase + "/reports/amount-mismatch?" + replQ, "ATM-SPV", map[string]any{"items": mismatch}},
		{"trips", cutoverBase + "/reports/trips?" + replQ, "BRANCH-ATM-SPV", map[string]any{"items": trips}},
	}
	for _, tc := range cases {
		for _, suffix := range []string{"", "&format=json", "&format="} {
			t.Run(tc.name+suffix, func(t *testing.T) {
				var router http.Handler
				if strings.Contains(tc.path, "/reports/amount-mismatch") || strings.Contains(tc.path, "/reports/trips") {
					router, _ = mountCutoverRepl()
				} else {
					router, _ = mountCutoverDsr()
				}
				rec := doRequest(router, http.MethodGet, tc.path+suffix, tokenForRole(t, testTokenService(), 1, tc.role), "")
				if rec.Code != http.StatusOK {
					t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
				}
				if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
					t.Errorf("Content-Type = %q", ct)
				}
				if !jsonEqual(t, rec.Body.Bytes(), tc.want) {
					t.Errorf("body changed: %s", rec.Body.String())
				}
			})
		}
	}
}
