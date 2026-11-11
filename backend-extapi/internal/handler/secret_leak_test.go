package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
)

// Property 6: no client secret, secret hash, access token or Authorization
// header ever reaches slog output, a response body (the access token only in
// the token response itself) or an audit snapshot.
func TestNoSecretMaterialInLogsOrResponses(t *testing.T) {
	e := newEnvWith(t, 100000, 100000)

	type resp struct {
		name string
		w    *httptest.ResponseRecorder
	}
	var all []resp
	run := func(name string, r *http.Request) *httptest.ResponseRecorder {
		w := e.serve(r)
		all = append(all, resp{name, w})
		return w
	}

	// token endpoint: success and every failure shape
	okTok := run("token ok", req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, e.secret)))
	var tr tokenResponse
	mustJSON(t, okTok.Body.Bytes(), &tr)
	tok := tr.AccessToken
	basic := base64.StdEncoding.EncodeToString([]byte(e.id + ":" + e.secret))

	run("bad secret", req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, e.secret+"x")))
	run("swapped fields", req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.secret, e.id)))
	run("unknown client", req(http.MethodPost, tokenPath, grantForm, formCT, withBasic("crown_aaaaaaaaaaaaaaaaaaaaaaaaaa", e.secret)))
	run("inactive", req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.otherID, e.secret)))
	run("ip", req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, e.secret), withIP("192.168.1.1:1")))
	run("secret in body", req(http.MethodPost, tokenPath, grantForm+"&client_secret="+e.secret, formCT, withBasic(e.id, e.secret)))
	run("secret in query", req(http.MethodPost, tokenPath+"?client_secret="+e.secret, grantForm, formCT, withBasic(e.id, e.secret)))
	run("bad scope", req(http.MethodPost, tokenPath, grantForm+"&scope=admin", formCT, withBasic(e.id, e.secret)))
	run("bad grant", req(http.MethodPost, tokenPath, "grant_type=password", formCT, withBasic(e.id, e.secret)))

	// user API: success, auth failures, validation, 4xx and 5xx service errors
	run("create", req(http.MethodPost, usersPath, `{"username":"alice"}`, jsonCT, withBearer(tok)))
	run("get", req(http.MethodGet, usersPath+"/7", "", "", withBearer(tok)))
	run("garbage bearer", req(http.MethodGet, usersPath+"/7", "", "", withBearer("leaky-garbage-token-123")))
	run("basic on api", req(http.MethodGet, usersPath+"/7", "", "", withBasic(e.id, e.secret)))
	run("bad id", req(http.MethodGet, usersPath+"/zzz", "", "", withBearer(tok)))
	run("bad body", req(http.MethodPost, usersPath, `{"password":"`+e.secret+`"}`, jsonCT, withBearer(tok)))
	e.users.err = errors.New("internal boom")
	run("500", req(http.MethodGet, usersPath+"/7", "", "", withBearer(tok)))
	e.users.err = extuser.ErrRoleNotAllowed
	run("403", req(http.MethodPost, usersPath, `{"username":"alice"}`, jsonCT, withBearer(tok)))
	run("not found route", req(http.MethodGet, "/nope", "", "", withBearer(tok)))

	logs := e.logs.String()
	if logs == "" {
		t.Fatal("no log output captured; the check would be vacuous")
	}

	// 1. secret material: nowhere at all (the token response may hold the token only).
	forbidden := map[string]string{
		"client secret":      e.secret,
		"secret hash":        e.hash,
		"hash prefix":        "$2a$",
		"basic credentials":  basic,
		"garbage token":      "leaky-garbage-token-123",
		"Authorization name": "authorization",
	}
	check := func(where, text string) {
		lower := strings.ToLower(text)
		for what, needle := range forbidden {
			if strings.Contains(lower, strings.ToLower(needle)) {
				t.Errorf("%s leaks %s", where, what)
			}
		}
	}
	check("slog output", logs)
	for _, r := range all {
		check("response body of "+r.name, r.w.Body.String())
		for k, v := range r.w.Header() {
			check("header "+k+" of "+r.name, strings.Join(v, ","))
		}
	}

	// 2. the access token: only in the successful token response body.
	if tok == "" {
		t.Fatal("no token issued")
	}
	if strings.Contains(logs, tok) {
		t.Error("slog output leaks the access token")
	}
	if strings.Contains(logs, "Bearer ") {
		t.Error("slog output leaks a Bearer header")
	}
	for _, r := range all {
		if r.name != "token ok" && strings.Contains(r.w.Body.String(), tok) {
			t.Errorf("response body of %s echoes the access token", r.name)
		}
	}

	// 3. the issued-token log line identifies the token by jti only.
	if !strings.Contains(logs, `"msg":"ext_token_issued"`) || !strings.Contains(logs, `"jti"`) {
		t.Error("expected an ext_token_issued log line with jti")
	}
}

// Audit snapshots (extuser.User) and the wire user carry no credential field.
func TestUserSnapshotsHaveNoCredentialFields(t *testing.T) {
	banned := regexp.MustCompile(`(?i)secret|hash|password|token`)
	for name, v := range map[string]any{"audit snapshot extuser.User": sampleUser(), "wire ExtUser": toExtUser(sampleUser())} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for k := range m {
			if banned.MatchString(k) {
				t.Errorf("%s has credential-like field %q", name, k)
			}
		}
	}
}
