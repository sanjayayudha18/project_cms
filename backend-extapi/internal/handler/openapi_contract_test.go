package handler

// ExtContract test (spec ext-user-api, Req 11.2): backend-extapi/api/openapi.yaml
// is valid, lists exactly the routes NewRouter mounts under /api/ext/v1, and
// every response the real router writes validates against it. The router, auth
// chain and handlers are real; only the user service and client store are fakes.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend-extapi/api"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

const extBase = "/api/ext/v1"

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

// validateContract checks one recorded response against the operation the
// request maps to; an undocumented status is an error too.
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

// TestOpenAPIContract_RouteCoverage: routes and spec operations are the same set.
func TestOpenAPIContract_RouteCoverage(t *testing.T) {
	doc, _ := loadContract(t)
	e := newEnv(t)
	var code, spec []string
	err := chi.Walk(e.h.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, extBase+"/") {
			code = append(code, method+" "+strings.TrimPrefix(route, extBase))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			spec = append(spec, method+" "+path)
		}
	}
	sort.Strings(code)
	sort.Strings(spec)
	if strings.Join(code, "\n") != strings.Join(spec, "\n") {
		t.Errorf("routes and spec differ\nroutes:\n%s\n\nspec:\n%s", strings.Join(code, "\n"), strings.Join(spec, "\n"))
	}
}

type contractCase struct {
	name       string
	method     string
	path       string // relative to extBase
	body       string
	ct         string
	auth       string // "" none | "full" | "read" | "basic" | "badbasic"
	svcErr     error
	clientErr  error
	ip         string
	failLimit  int  // 0 = 1000
	apiPerMin  int  // 0 = 1000
	preFail    bool // send one bad-secret token request first
	redisDown  bool
	wantStatus int
}

const (
	ctCreateBody = `{"username":"alice","full_name":"Alice A","email":"alice@example.id","role":"ATM-USER","employee_id":"E100","supervisor_id":3,"approval_level":1}`
	ctUpdateBody = `{"full_name":"Alice A","email":"alice@example.id","role":"ATM-USER"}`
)

func contractCases() []contractCase {
	j := jsonCT
	conflict := &userrules.ConflictError{Field: "email", Message: "sudah dipakai"}
	return []contractCase{
		// token
		{name: "token ok", method: "POST", path: "/oauth/token", body: grantForm, ct: formCT, auth: "basic", wantStatus: 200},
		{name: "token wrong content type", method: "POST", path: "/oauth/token", body: grantForm, ct: j, auth: "basic", wantStatus: 400},
		{name: "token unsupported grant", method: "POST", path: "/oauth/token", body: "grant_type=password", ct: formCT, auth: "basic", wantStatus: 400},
		{name: "token invalid scope", method: "POST", path: "/oauth/token", body: grantForm + "&scope=users:admin", ct: formCT, auth: "basic", wantStatus: 400},
		{name: "token bad secret", method: "POST", path: "/oauth/token", body: grantForm, ct: formCT, auth: "badbasic", wantStatus: 401},
		{name: "token no auth", method: "POST", path: "/oauth/token", body: grantForm, ct: formCT, wantStatus: 401},
		{name: "token rate limited", method: "POST", path: "/oauth/token", body: grantForm, ct: formCT, auth: "badbasic", failLimit: 1, preFail: true, wantStatus: 429},
		{name: "token redis down", method: "POST", path: "/oauth/token", body: grantForm, ct: formCT, auth: "basic", redisDown: true, wantStatus: 503},
		{name: "token client lookup error", method: "POST", path: "/oauth/token", body: grantForm, ct: formCT, auth: "basic", clientErr: errors.New("boom"), wantStatus: 500},

		// create
		{name: "create", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", wantStatus: 201},
		{name: "create bad json", method: "POST", path: "/users", body: "{", ct: j, auth: "full", wantStatus: 400},
		{name: "create unknown field", method: "POST", path: "/users", body: `{"x":1}`, ct: j, auth: "full", wantStatus: 400},
		{name: "create wrong content type", method: "POST", path: "/users", body: ctCreateBody, ct: "text/plain", auth: "full", wantStatus: 415},
		{name: "create no token", method: "POST", path: "/users", body: ctCreateBody, ct: j, wantStatus: 401},
		{name: "create read scope", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "read", wantStatus: 403},
		{name: "create ip not allowed", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", ip: "192.168.1.1:99", wantStatus: 403},
		{name: "create role not allowed", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", svcErr: extuser.ErrRoleNotAllowed, wantStatus: 403},
		{name: "create conflict", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", svcErr: conflict, wantStatus: 409},
		{name: "create validation", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full",
			svcErr: &auth.ValidationError{Field: "email", Message: "tidak valid"}, wantStatus: 422},
		{name: "create invalid reference", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full",
			svcErr: &userrules.ReferenceError{Field: "role", Message: "tidak ada"}, wantStatus: 400},
		{name: "create self supervision", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", svcErr: extuser.ErrSelfSupervision, wantStatus: 400},
		{name: "create internal", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", svcErr: errors.New("boom"), wantStatus: 500},
		{name: "create rate limited", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", apiPerMin: 1, wantStatus: 429},
		{name: "create redis down", method: "POST", path: "/users", body: ctCreateBody, ct: j, auth: "full", redisDown: true, wantStatus: 503},

		// find / get
		{name: "find", method: "GET", path: "/users?username=alice", auth: "read", wantStatus: 200},
		{name: "find missing username", method: "GET", path: "/users", auth: "read", wantStatus: 400},
		{name: "find not found", method: "GET", path: "/users?username=zed", auth: "read", svcErr: extuser.ErrUserNotFound, wantStatus: 404},
		{name: "find no token", method: "GET", path: "/users?username=alice", wantStatus: 401},
		{name: "get", method: "GET", path: "/users/7", auth: "read", wantStatus: 200},
		{name: "get bad id", method: "GET", path: "/users/abc", auth: "read", wantStatus: 400},
		{name: "get not found", method: "GET", path: "/users/7", auth: "read", svcErr: extuser.ErrUserNotFound, wantStatus: 404},
		{name: "get internal", method: "GET", path: "/users/7", auth: "read", svcErr: errors.New("boom"), wantStatus: 500},

		// update
		{name: "update", method: "PUT", path: "/users/7", body: ctUpdateBody, ct: j, auth: "full", wantStatus: 200},
		{name: "update bad id", method: "PUT", path: "/users/0", body: ctUpdateBody, ct: j, auth: "full", wantStatus: 400},
		{name: "update wrong content type", method: "PUT", path: "/users/7", body: ctUpdateBody, ct: "text/plain", auth: "full", wantStatus: 415},
		{name: "update not found", method: "PUT", path: "/users/7", body: ctUpdateBody, ct: j, auth: "full", svcErr: extuser.ErrUserNotFound, wantStatus: 404},
		{name: "update conflict", method: "PUT", path: "/users/7", body: ctUpdateBody, ct: j, auth: "full", svcErr: conflict, wantStatus: 409},
		{name: "update validation", method: "PUT", path: "/users/7", body: ctUpdateBody, ct: j, auth: "full",
			svcErr: &auth.ValidationError{Field: "full_name", Message: "wajib diisi"}, wantStatus: 422},

		// disable / enable
		{name: "disable", method: "DELETE", path: "/users/7", auth: "full", wantStatus: 204},
		{name: "disable read scope", method: "DELETE", path: "/users/7", auth: "read", wantStatus: 403},
		{name: "disable not found", method: "DELETE", path: "/users/7", auth: "full", svcErr: extuser.ErrUserNotFound, wantStatus: 404},
		{name: "enable", method: "POST", path: "/users/7/enable", auth: "full", wantStatus: 200},
		{name: "enable bad id", method: "POST", path: "/users/x/enable", auth: "full", wantStatus: 400},
		{name: "enable not found", method: "POST", path: "/users/7/enable", auth: "full", svcErr: extuser.ErrUserNotFound, wantStatus: 404},
		{name: "enable internal", method: "POST", path: "/users/7/enable", auth: "full", svcErr: errors.New("boom"), wantStatus: 500},
	}
}

func serveContractCase(t *testing.T, tc contractCase) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	fl, rate := tc.failLimit, tc.apiPerMin
	if fl == 0 {
		fl = 1000
	}
	if rate == 0 {
		rate = 1000
	}
	e := newEnvWith(t, fl, rate)
	var tok string
	switch tc.auth {
	case "full":
		tok = e.tokenFor(t, "")
	case "read":
		tok = e.tokenFor(t, "users:read")
	}
	if tc.apiPerMin > 0 { // spend the allowance so the next call is limited
		e.api(http.MethodGet, usersPath+"/7", "", tok)
	}
	if tc.preFail {
		e.serve(req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, "wrong")))
	}
	e.users.err, e.clients.err = tc.svcErr, tc.clientErr
	if tc.redisDown {
		e.mr.Close()
	}

	var opts []reqOpt
	switch tc.auth {
	case "full", "read":
		opts = append(opts, withBearer(tok))
	case "basic":
		opts = append(opts, withBasic(e.id, e.secret))
	case "badbasic":
		opts = append(opts, withBasic(e.id, "wrong-secret"))
	}
	if tc.ip != "" {
		opts = append(opts, withIP(tc.ip))
	}
	rec := e.serve(req(tc.method, extBase+tc.path, tc.body, tc.ct, opts...))
	// a fresh request for the validator: the served one has a consumed body
	return rec, httptest.NewRequest(tc.method, extBase+tc.path, nil)
}

func TestOpenAPIContract_Responses(t *testing.T) {
	doc, router := loadContract(t)
	covered := map[string]bool{}
	for _, tc := range contractCases() {
		t.Run(tc.name, func(t *testing.T) {
			rec, vreq := serveContractCase(t, tc)
			if route, _, err := router.FindRoute(vreq); err == nil {
				covered[route.Method+" "+strings.TrimPrefix(route.Path, extBase)] = true
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if err := validateContract(router, vreq, rec.Code, rec.Header(), rec.Body.Bytes()); err != nil {
				t.Errorf("response does not match openapi.yaml: %v\nbody: %s", err, rec.Body.String())
			}
		})
	}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			if !covered[method+" "+path] {
				t.Errorf("no response case for %s %s in contractCases", method, path)
			}
		}
	}
}

// TestOpenAPIContract_RejectsDrift: the schemas catch a
// wrong type, a dropped field, a bad format and an undocumented status.
func TestOpenAPIContract_RejectsDrift(t *testing.T) {
	_, router := loadContract(t)
	rec, vreq := serveContractCase(t, contractCase{method: "GET", path: "/users/7", auth: "read"})
	good := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("fixture: %d %s", rec.Code, good)
	}
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"id as string", 200, strings.Replace(good, `"id":7`, `"id":"7"`, 1)},
		{"missing field", 200, strings.Replace(good, `"username":"alice",`, "", 1)},
		{"bad timestamp", 200, strings.Replace(good, `"created_at":"2026-11-01T03:00:00Z"`, `"created_at":"yesterday"`, 1)},
		{"undocumented status", http.StatusTeapot, good},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status == 200 && tc.body == good {
				t.Fatalf("fixture did not change; replace target missing in %s", good)
			}
			if err := validateContract(router, vreq, tc.status, rec.Header(), []byte(tc.body)); err == nil {
				t.Errorf("validation passed, want failure")
			}
		})
	}
}
