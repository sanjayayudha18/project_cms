package handler

// Property 17 (Validasi parameter) at the HTTP layer + template-without-service
// checks (task 10.4). The handler calls the REAL exportjob.ParseParams; only the
// service is faked (fakeSvc from exports_handler_test.go).

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

var (
	vendorEntities = []string{"vendor", "branches", "vaults", "pics", "dsr-location-maps"}
	cabangEntities = []string{"vaults", "pics", "dsr-location-maps"}
)

type propRoute struct {
	name     string
	kind     exportjob.Kind
	entities []string // valid entities for this route
	hasVID   bool
	hasBID   bool
	target   func(vid, bid, entity string) string
}

var propRoutes = []propRoute{
	{"vendor", exportjob.KindVendor, vendorEntities, true, false,
		func(v, _, e string) string { return "/vendors/" + v + "/" + e }},
	{"cabang", exportjob.KindCabang, cabangEntities, true, true,
		func(v, b, e string) string { return "/vendors/" + v + "/branches/" + b + "/" + e }},
	{"prices", exportjob.KindHargaPaket, []string{"package-prices"}, true, false,
		func(v, _, _ string) string { return "/vendors/" + v + "/package-prices" }},
	{"global", exportjob.KindMasterDataGlobal, csvexport.ImportableNames(), false, false,
		func(_, _, e string) string { return "/master-data/" + e }},
}

func positiveID(s string) bool { v, err := strconv.ParseInt(s, 10, 64); return err == nil && v > 0 }

// checkCase sends one request and asserts the Property 17 oracle. branchQ is the
// optional ?branch_id= of the prices route.
func checkCase(t *testing.T, tag string, rt propRoute, vid, bid, entity, status, level, branchQ string) {
	t.Helper()
	if rt.kind == exportjob.KindHargaPaket {
		entity = "package-prices" // fixed by the route, not a path segment
	}
	f := &fakeSvc{created: true, chunkErrAt: -1}
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if level != "" {
		q.Set("price_level", level)
	}
	if branchQ != "" {
		q.Set("branch_id", branchQ)
	}
	target := rt.target(url.PathEscape(vid), url.PathEscape(bid), url.PathEscape(entity))
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", target, nil)
	newRouter(f, "ADMIN").ServeHTTP(rec, req)

	// Expected outcome, in the order the handler/ParseParams check things.
	want := 202
	switch {
	case rt.hasVID && !positiveID(vid):
		want = 400
	case rt.hasBID && !positiveID(bid):
		want = 400
	case rt.kind == exportjob.KindHargaPaket && branchQ != "" && !positiveID(branchQ):
		want = 400
	case !slices.Contains(rt.entities, entity):
		want = 404
	case status != "" && !slices.Contains([]string{"active", "disabled", "all"}, status):
		want = 400
	case rt.kind == exportjob.KindHargaPaket && level != "" && !slices.Contains([]string{"vendor", "branch", "atm", "all"}, level):
		want = 400
	}
	// price_level is ignored (not rejected) on non-prices routes.
	if rec.Code != want || rec.Code == 500 {
		t.Fatalf("%s %s %s: got %d want %d body=%s", tag, rt.name, target, rec.Code, want, rec.Body)
	}
	if want != 202 {
		if f.createCalls != 0 {
			t.Fatalf("%s %s %s: Create called for rejected input", tag, rt.name, target)
		}
		return
	}
	p := f.gotParams
	wantStatus := status
	if wantStatus == "" {
		wantStatus = "active"
	}
	if f.createCalls != 1 || p.Kind != rt.kind || p.Entity != entity || p.StatusFilter != wantStatus {
		t.Fatalf("%s %s %s: params %+v calls=%d", tag, rt.name, target, p, f.createCalls)
	}
	if rt.kind == exportjob.KindHargaPaket {
		wantLevel := level
		if wantLevel == "" {
			wantLevel = "all"
		}
		if p.PriceLevel == nil || *p.PriceLevel != wantLevel {
			t.Fatalf("%s %s: price level %v want %s", tag, target, p.PriceLevel, wantLevel)
		}
	} else if p.PriceLevel != nil {
		t.Fatalf("%s %s: price level leaked %v", tag, target, *p.PriceLevel)
	}
}

func randJunk(r *rand.Rand) string {
	const alphabet = "abcXYZ019-_ .%+/é\x00'\";<>"
	rs := []rune(alphabet)
	for {
		n := 1 + r.IntN(8)
		var b []rune
		for range n {
			b = append(b, rs[r.IntN(len(rs))])
		}
		if s := string(b); s != "." && s != ".." { // path-cleaning is router behaviour, not validation
			return s
		}
	}
}

func pick(r *rand.Rand, pool []string, junkPct int) string {
	if r.IntN(100) < junkPct {
		return randJunk(r)
	}
	return pool[r.IntN(len(pool))]
}

func TestProperty17HTTPParamValidation(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	r := rand.New(rand.NewPCG(seed, 17))
	ids := []string{"1", "42", "9223372036854775807", "0", "-1", "abc", "1.5", "9223372036854775808", "007"}
	statuses := []string{"", "active", "disabled", "all", "ACTIVE", " active", "deleted"}
	levels := []string{"", "vendor", "branch", "atm", "all", "ATM", "x"}
	branchQs := []string{"", "", "3", "0", "-4", "z"}

	for i := range 600 {
		rt := propRoutes[r.IntN(len(propRoutes))]
		entity := pick(r, rt.entities, 35)
		if rt.kind == exportjob.KindVendor && entity == "package-prices" {
			continue // routed to the prices route by design
		}
		checkCase(t, fmt.Sprintf("seed=%d i=%d", seed, i), rt,
			pick(r, ids, 15), pick(r, ids, 15), entity,
			pick(r, statuses, 25), pick(r, levels, 25), branchQs[r.IntN(len(branchQs))])
	}
}

func TestProperty17Defaults(t *testing.T) {
	for _, rt := range propRoutes {
		checkCase(t, "defaults", rt, "5", "6", rt.entities[0], "", "", "")
	}
}

func FuzzProperty17(f *testing.F) {
	f.Add("", "", "vendor", "1", uint8(0))
	f.Add("all", "atm", "package-prices", "7", uint8(2))
	f.Add("bogus", "bogus", "nope", "-1", uint8(3))
	f.Fuzz(func(t *testing.T, status, level, entity, id string, route uint8) {
		if entity == "" || entity == "." || entity == ".." || id == "" || id == "." || id == ".." {
			t.Skip()
		}
		rt := propRoutes[int(route)%len(propRoutes)]
		if rt.kind == exportjob.KindVendor && entity == "package-prices" {
			t.Skip()
		}
		checkCase(t, "fuzz", rt, id, "1", entity, status, level, "")
	})
}

func TestTemplateEveryImportableEntityNeverTouchesService(t *testing.T) {
	for _, name := range csvexport.ImportableNames() {
		t.Run(name, func(t *testing.T) {
			f := &fakeSvc{chunkErrAt: -1}
			rec := do(newRouter(f, "ADMIN"), "GET", "/master-data/"+name+"/template")
			body := rec.Body.Bytes()
			if rec.Code != 200 || !bytes.HasPrefix(body, []byte("\xef\xbb\xbf")) || bytes.Count(body, []byte("\n")) != 1 {
				t.Fatalf("got %d %q", rec.Code, body)
			}
			e, _ := csvexport.Lookup(name)
			if !bytes.Contains(body, []byte(e.Header[0])) {
				t.Fatalf("header %q missing from %q", e.Header[0], body)
			}
			if f.createCalls != 0 || f.gotParams != nil || f.gotActor != (exportjob.Actor{}) || f.gotLimit != 0 {
				t.Fatalf("service touched: %+v", f)
			}
		})
	}
	for _, name := range append(csvexport.ExportOnlyNames(), "nope") {
		if rec := do(newRouter(&fakeSvc{}, "ADMIN"), "GET", "/master-data/"+name+"/template"); rec.Code != 404 {
			t.Fatalf("%s: got %d want 404", name, rec.Code)
		}
	}
}
