package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
)

// Shared fixtures for the handler tests (task 7.4). Everything is in-process:
// miniredis for the rate limits, fakes for the client reader and user service.

const (
	testIP     = "10.1.2.3"
	tokenPath  = "/api/ext/v1/oauth/token"
	usersPath  = "/api/ext/v1/users"
	jsonCT     = "application/json"
	formCT     = "application/x-www-form-urlencoded"
	grantForm  = "grant_type=client_credentials"
	serviceUID = int64(99)
)

type fakeClients struct {
	m   map[string]*extauth.Client
	err error
}

func (f *fakeClients) GetByClientID(_ context.Context, id string) (*extauth.Client, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.m[id], nil
}

// fakeUsers records the last call and returns the configured user or error.
type fakeUsers struct {
	user  extuser.User
	err   error
	calls []string
	actor extuser.Actor
	id    int64
	cr    extuser.CreateRequest
	ur    extuser.UpdateRequest
	name  string
}

func (f *fakeUsers) rec(call string, a extuser.Actor, id int64) {
	f.calls, f.actor, f.id = append(f.calls, call), a, id
}

func (f *fakeUsers) Create(_ context.Context, a extuser.Actor, r extuser.CreateRequest) (extuser.User, error) {
	f.rec("create", a, 0)
	f.cr = r
	return f.user, f.err
}

func (f *fakeUsers) Update(_ context.Context, a extuser.Actor, id int64, r extuser.UpdateRequest) (extuser.User, error) {
	f.rec("update", a, id)
	f.ur = r
	return f.user, f.err
}

func (f *fakeUsers) Disable(_ context.Context, a extuser.Actor, id int64) error {
	f.rec("disable", a, id)
	return f.err
}

func (f *fakeUsers) Enable(_ context.Context, a extuser.Actor, id int64) (extuser.User, error) {
	f.rec("enable", a, id)
	return f.user, f.err
}

func (f *fakeUsers) Get(_ context.Context, id int64) (extuser.User, error) {
	f.rec("get", extuser.Actor{}, id)
	return f.user, f.err
}

func (f *fakeUsers) GetByUsername(_ context.Context, n string) (extuser.User, error) {
	f.rec("byname", extuser.Actor{}, 0)
	f.name = n
	return f.user, f.err
}

func sampleUser() extuser.User {
	eid := "E100"
	return extuser.User{
		ID: 7, Username: "alice", FullName: "Alice A", Email: "alice@example.id", Role: "ATM-USER",
		EmployeeID: &eid, IsActive: true, AuthSource: "ldap",
		CreatedAt: time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC),
	}
}

type env struct {
	h       http.Handler
	mr      *miniredis.Miniredis
	logs    *bytes.Buffer
	tokens  *extauth.TokenService
	id      string
	secret  string
	hash    string
	client  *extauth.Client
	other   *extauth.Client // inactive
	otherID string
	clients *fakeClients
	users   *fakeUsers
	ready   map[string]func(context.Context) error
}

func newEnv(t *testing.T) *env { return newEnvWith(t, 1000, 1000) }

// newEnvWith builds the full router. Not parallel-safe: it swaps slog's default
// logger to capture every log line.
func newEnvWith(t *testing.T, failLimit, apiPerMin int) *env {
	t.Helper()
	e := &env{logs: &bytes.Buffer{}, users: &fakeUsers{user: sampleUser()}, ready: map[string]func(context.Context) error{}}

	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	var err error
	if e.id, err = extauth.NewClientID(); err != nil {
		t.Fatal(err)
	}
	if e.otherID, err = extauth.NewClientID(); err != nil {
		t.Fatal(err)
	}
	if e.secret, err = extauth.NewSecret(); err != nil {
		t.Fatal(err)
	}
	if e.hash, err = extauth.HashSecret(e.secret); err != nil {
		t.Fatal(err)
	}
	e.client = &extauth.Client{
		ClientID: e.id, SecretHash: e.hash, CredentialVersion: 1, IsActive: true,
		Scopes:          []string{extauth.ScopeUsersRead, extauth.ScopeUsersWrite},
		AssignableRoles: []string{"ATM-USER"}, ServiceUserID: serviceUID,
		AllowedCIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	}
	inactive := *e.client
	inactive.ClientID, inactive.IsActive = e.otherID, false
	e.other = &inactive
	e.clients = &fakeClients{m: map[string]*extauth.Client{e.id: e.client, e.otherID: e.other}}

	e.mr = miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: e.mr.Addr(), MaxRetries: -1, DialTimeout: 300 * time.Millisecond})
	t.Cleanup(func() { _ = rdb.Close() })

	e.tokens = extauth.NewTokenService([]byte(strings.Repeat("k", 32)), "crown-ext-test", 15*time.Minute)
	e.h = NewRouter(Deps{
		Tokens:      e.tokens,
		Clients:     e.clients,
		Failures:    extauth.NewTokenFailures(rdb, failLimit, time.Minute),
		Rate:        extauth.NewAPIRate(rdb, apiPerMin),
		Users:       e.users,
		ReadyChecks: e.ready,
	})
	return e
}

type reqOpt func(*http.Request)

func withHeader(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }
func withBearer(tok string) reqOpt  { return withHeader("Authorization", "Bearer "+tok) }
func withBasic(u, p string) reqOpt  { return func(r *http.Request) { r.SetBasicAuth(u, p) } }
func withIP(addr string) reqOpt     { return func(r *http.Request) { r.RemoteAddr = addr } }

// req builds a request from testIP; ct "" sends no Content-Type.
func req(method, path, body, ct string, opts ...reqOpt) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if ct != "" {
		r.Header.Set("Content-Type", ct)
	}
	r.RemoteAddr = testIP + ":4321"
	for _, o := range opts {
		o(r)
	}
	return r
}

func (e *env) serve(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

// tokenFor obtains a real token through the router.
func (e *env) tokenFor(t *testing.T, scope string) string {
	t.Helper()
	form := grantForm
	if scope != "" {
		form += "&scope=" + url.QueryEscape(scope)
	}
	w := e.serve(req(http.MethodPost, tokenPath, form, formCT, withBasic(e.id, e.secret)))
	if w.Code != http.StatusOK {
		t.Fatalf("token request: %d %s", w.Code, w.Body)
	}
	var resp tokenResponse
	mustJSON(t, w.Body.Bytes(), &resp)
	return resp.AccessToken
}

// api sends a JSON request with a bearer token.
func (e *env) api(method, path, body, tok string) *httptest.ResponseRecorder {
	ct := ""
	if body != "" {
		ct = jsonCT
	}
	return e.serve(req(method, path, body, ct, withBearer(tok)))
}

func mustJSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("bad JSON %q: %v", b, err)
	}
}

// ---- Property 13 (backend-extapi side) ------------------------------------

func TestRouteTableOnlyAllowedPaths(t *testing.T) {
	e := newEnv(t)
	var got []string
	err := chi.Walk(e.h.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got = append(got, method+" "+route)
		if route != "/health" && route != "/ready" && !strings.HasPrefix(route, "/api/ext/v1/") {
			t.Errorf("route outside the allowlist: %s %s", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /health", "GET /ready", "POST /api/ext/v1/oauth/token",
		"POST /api/ext/v1/users", "GET /api/ext/v1/users", "GET /api/ext/v1/users/{id}",
		"PUT /api/ext/v1/users/{id}", "DELETE /api/ext/v1/users/{id}", "POST /api/ext/v1/users/{id}/enable",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("route table\n got %v\nwant %v", got, want)
	}
}

func TestUnknownRoutesAnswerJSON(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/", "/api/v1/users", "/api/ext/v1/nope", "/api/auth/login", "/metrics"} {
		w := e.serve(req(http.MethodGet, p, "", ""))
		if w.Code != http.StatusNotFound || w.Header().Get("Content-Type") != jsonCT || !strings.Contains(w.Body.String(), `"not_found"`) {
			t.Errorf("%s: %d %q", p, w.Code, w.Body)
		}
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, tokenPath}, {http.MethodPatch, usersPath}, {http.MethodPost, "/health"},
	} {
		w := e.serve(req(c.method, c.path, "", ""))
		if w.Code != http.StatusMethodNotAllowed || !strings.Contains(w.Body.String(), `"method_not_allowed"`) {
			t.Errorf("%s %s: %d %q", c.method, c.path, w.Code, w.Body)
		}
	}
}

func TestHealthAndReady(t *testing.T) {
	e := newEnv(t)
	if w := e.serve(req(http.MethodGet, "/health", "", "")); w.Code != 200 || !strings.Contains(w.Body.String(), `"ok"`) {
		t.Fatalf("health: %d %s", w.Code, w.Body)
	}
	e.ready["db"] = func(context.Context) error { return nil }
	if w := e.serve(req(http.MethodGet, "/ready", "", "")); w.Code != 200 {
		t.Fatalf("ready ok: %d %s", w.Code, w.Body)
	}
	e.ready["redis"] = func(context.Context) error { return errors.New("dial tcp 10.9.9.9:6379 secret-dsn") }
	w := e.serve(req(http.MethodGet, "/ready", "", ""))
	if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "secret-dsn") || !strings.Contains(w.Body.String(), "fail") {
		t.Fatalf("ready fail: %d %s", w.Code, w.Body)
	}
}

// ---- authentication chain on /users ----------------------------------------

func TestUsersRequireClientToken(t *testing.T) {
	e := newEnv(t)
	good := e.tokenFor(t, "")
	readOnly := e.tokenFor(t, "users:read")

	rotated := e.tokenFor(t, "")
	e.client.CredentialVersion = 2 // secret rotated after the token was issued
	if w := e.api(http.MethodGet, usersPath+"/7", "", rotated); w.Code != 401 {
		t.Errorf("rotated credential: %d", w.Code)
	}
	e.client.CredentialVersion = 1

	for name, r := range map[string]*http.Request{
		"no header":    req(http.MethodGet, usersPath+"/7", "", ""),
		"garbage":      req(http.MethodGet, usersPath+"/7", "", "", withBearer("not-a-jwt")),
		"basic scheme": req(http.MethodGet, usersPath+"/7", "", "", withBasic(e.id, e.secret)),
		"empty bearer": req(http.MethodGet, usersPath+"/7", "", "", withHeader("Authorization", "Bearer ")),
	} {
		w := e.serve(r)
		if w.Code != 401 || !strings.Contains(w.Header().Get("WWW-Authenticate"), "invalid_token") || !strings.Contains(w.Body.String(), "invalid_token") {
			t.Errorf("%s: %d %v %s", name, w.Code, w.Header(), w.Body)
		}
	}
	// the scheme name is case-insensitive (RFC 6750)
	if w := e.serve(req(http.MethodGet, usersPath+"/7", "", "", withHeader("Authorization", "bearer "+good))); w.Code != 200 {
		t.Errorf("lowercase scheme: %d", w.Code)
	}

	// a users:read token cannot write
	w := e.api(http.MethodPost, usersPath, `{}`, readOnly)
	if w.Code != 403 || !strings.Contains(w.Header().Get("WWW-Authenticate"), "insufficient_scope") {
		t.Errorf("scope: %d %v", w.Code, w.Header())
	}
	if !slices.Equal(e.users.calls, []string{"get"}) { // only the lowercase-bearer GET got through
		t.Errorf("service reached by a rejected request: %v", e.users.calls)
	}

	// deactivated client
	e.client.IsActive = false
	if w := e.api(http.MethodGet, usersPath+"/7", "", good); w.Code != 401 {
		t.Errorf("inactive client: %d", w.Code)
	}
	e.client.IsActive = true

	// source IP outside allowed_cidrs
	w = e.serve(req(http.MethodGet, usersPath+"/7", "", "", withBearer(good), withIP("192.168.1.1:99")))
	if w.Code != 403 || !strings.Contains(w.Body.String(), "ip_not_allowed") {
		t.Errorf("ip: %d %s", w.Code, w.Body)
	}
}

func TestAPIRateLimitAndRedisDown(t *testing.T) {
	e := newEnvWith(t, 1000, 2)
	tok := e.tokenFor(t, "")
	for i := 0; i < 2; i++ {
		if w := e.api(http.MethodGet, usersPath+"/7", "", tok); w.Code != 200 {
			t.Fatalf("req %d: %d", i, w.Code)
		}
	}
	w := e.api(http.MethodGet, usersPath+"/7", "", tok)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "rate_limited") {
		t.Fatalf("rate: %d %v %s", w.Code, w.Header(), w.Body)
	}

	e.mr.Close() // fail closed
	if w := e.api(http.MethodGet, usersPath+"/7", "", tok); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "service_unavailable") {
		t.Fatalf("redis down: %d %s", w.Code, w.Body)
	}
}

func TestClientLookupErrorOnAPIIs500WithoutDetail(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(t, "")
	e.clients.err = errors.New("pq: connection to 10.7.7.7 refused")
	w := e.api(http.MethodGet, usersPath+"/7", "", tok)
	if w.Code != 500 || strings.Contains(w.Body.String(), "10.7.7.7") {
		t.Fatalf("lookup error: %d %s", w.Code, w.Body)
	}
}
