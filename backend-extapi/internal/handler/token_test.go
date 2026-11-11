package handler

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const invalidClientBody = `{"error":"invalid_client","error_description":"client authentication failed"}` + "\n"

func (e *env) token(form string, opts ...reqOpt) *httptest.ResponseRecorder {
	return e.serve(req(http.MethodPost, tokenPath, form, formCT, opts...))
}

func TestTokenSuccess(t *testing.T) {
	e := newEnv(t)
	w := e.token(grantForm, withBasic(e.id, e.secret))
	if w.Code != 200 || w.Header().Get("Content-Type") != jsonCT ||
		w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("success: %d %v", w.Code, w.Header())
	}
	var resp tokenResponse
	mustJSON(t, w.Body.Bytes(), &resp)
	if resp.TokenType != "Bearer" || resp.ExpiresIn != 900 || resp.Scope != "users:read users:write" || resp.AccessToken == "" {
		t.Fatalf("body: %+v", resp)
	}
	claims, err := e.tokens.Validate(resp.AccessToken)
	if err != nil || claims.Subject != e.id || claims.Scope != resp.Scope {
		t.Fatalf("claims: %+v %v", claims, err)
	}
}

func TestTokenScopes(t *testing.T) {
	for _, c := range []struct {
		name, scope, want string
		code              int
	}{
		{"default grants all held", "", "users:read users:write", 200},
		{"subset", "users:read", "users:read", 200},
		{"comma separated", "users:write,users:read", "users:write users:read", 200},
		{"duplicates dropped", "users:read users:read", "users:read", 200},
		{"unknown", "admin", "", 400},
		{"one unknown among valid", "users:read admin", "", 400},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			form := grantForm
			if c.scope != "" {
				form += "&scope=" + url.QueryEscape(c.scope)
			}
			w := e.token(form, withBasic(e.id, e.secret))
			if w.Code != c.code {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if c.code == 200 {
				var resp tokenResponse
				mustJSON(t, w.Body.Bytes(), &resp)
				if resp.Scope != c.want {
					t.Fatalf("scope %q, want %q", resp.Scope, c.want)
				}
			} else if !strings.Contains(w.Body.String(), `"invalid_scope"`) {
				t.Fatalf("want invalid_scope: %s", w.Body)
			}
		})
	}

	t.Run("scope the client does not hold", func(t *testing.T) {
		e := newEnv(t)
		e.client.Scopes = []string{"users:read"}
		w := e.token(grantForm+"&scope=users:write", withBasic(e.id, e.secret))
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"invalid_scope"`) {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
	t.Run("client with no usable scope", func(t *testing.T) {
		e := newEnv(t)
		e.client.Scopes = []string{"legacy:thing"}
		w := e.token(grantForm, withBasic(e.id, e.secret))
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"invalid_scope"`) {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
}

// Property 8: every cause of a failed client authentication answers the same
// status, body and challenge.
func TestInvalidClientIsIdenticalForEveryCause(t *testing.T) {
	e := newEnvWith(t, 100000, 1000) // keep the failed-request limit out of the way
	causes := map[string]*http.Request{
		"no authorization":     req(http.MethodPost, tokenPath, grantForm, formCT),
		"not basic":            req(http.MethodPost, tokenPath, grantForm, formCT, withHeader("Authorization", "Bearer abc")),
		"broken base64":        req(http.MethodPost, tokenPath, grantForm, formCT, withHeader("Authorization", "Basic !!!")),
		"empty client_id":      req(http.MethodPost, tokenPath, grantForm, formCT, withBasic("", e.secret)),
		"empty secret":         req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, "")),
		"bad percent escape":   req(http.MethodPost, tokenPath, grantForm, formCT, withBasic("%zz", e.secret)),
		"client_id bad format": req(http.MethodPost, tokenPath, grantForm, formCT, withBasic("nope", e.secret)),
		"unknown client":       req(http.MethodPost, tokenPath, grantForm, formCT, withBasic("crown_aaaaaaaaaaaaaaaaaaaaaaaaaa", e.secret)),
		"wrong secret":         req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, e.secret+"x")),
		"inactive client":      req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.otherID, e.secret)),
		"ip not allowed":       req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, e.secret), withIP("192.168.1.1:5")),
		"unparseable ip":       req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.id, e.secret), withIP("garbage")),
		"fields swapped":       req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(e.secret, e.id)),
	}
	// random credentials of any shape must look the same too
	for i := 0; i < 15; i++ {
		u, p := randWord(rand.IntN(40)+1), randWord(rand.IntN(40)+1)
		causes["random "+u] = req(http.MethodPost, tokenPath, grantForm, formCT, withBasic(u, p))
	}

	for name, r := range causes {
		w := e.serve(r)
		if w.Code != http.StatusUnauthorized || w.Body.String() != invalidClientBody ||
			w.Header().Get("WWW-Authenticate") != basicChallenge ||
			w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Pragma") != "no-cache" ||
			w.Header().Get("Content-Type") != jsonCT || w.Header().Get("Retry-After") != "" {
			t.Errorf("%s: %d %q %v", name, w.Code, w.Body, w.Header())
		}
	}
}

func randWord(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(b)
}

func TestTokenInvalidRequest(t *testing.T) {
	e := newEnv(t)
	ok := withBasic(e.id, e.secret)
	big := grantForm + "&scope=" + strings.Repeat("a", maxTokenBody)
	for _, c := range []struct {
		name string
		r    *http.Request
		code string
	}{
		{"json content type", req(http.MethodPost, tokenPath, `{"grant_type":"client_credentials"}`, jsonCT, ok), "invalid_request"},
		{"no content type", req(http.MethodPost, tokenPath, grantForm, "", ok), "invalid_request"},
		{"broken content type", req(http.MethodPost, tokenPath, grantForm, ";;;", ok), "invalid_request"},
		{"duplicate grant_type", req(http.MethodPost, tokenPath, grantForm+"&grant_type=client_credentials", formCT, ok), "invalid_request"},
		{"duplicate scope", req(http.MethodPost, tokenPath, grantForm+"&scope=users:read&scope=users:write", formCT, ok), "invalid_request"},
		{"client_secret in body", req(http.MethodPost, tokenPath, grantForm+"&client_secret=x", formCT, ok), "invalid_request"},
		{"client_secret in query", req(http.MethodPost, tokenPath+"?client_secret=x", grantForm, formCT, ok), "invalid_request"},
		{"body too large", req(http.MethodPost, tokenPath, big, formCT, ok), "invalid_request"},
		{"missing grant_type", req(http.MethodPost, tokenPath, "", formCT, ok), "unsupported_grant_type"},
		{"wrong grant_type", req(http.MethodPost, tokenPath, "grant_type=password", formCT, ok), "unsupported_grant_type"},
		{"grant_type only in query", req(http.MethodPost, tokenPath+"?"+grantForm, "", formCT, ok), "unsupported_grant_type"},
	} {
		w := e.serve(c.r)
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"error":"`+c.code+`"`) ||
			w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("WWW-Authenticate") != "" {
			t.Errorf("%s: %d %s %v", c.name, w.Code, w.Body, w.Header())
		}
	}
}

func TestTokenBareRemoteAddrFromTrustedRealIP(t *testing.T) {
	e := newEnv(t)
	if w := e.token(grantForm, withBasic(e.id, e.secret), withIP(testIP)); w.Code != 200 { // no port
		t.Fatalf("bare IP: %d %s", w.Code, w.Body)
	}
	if w := e.token(grantForm, withBasic(e.id, e.secret), withIP("[::ffff:10.1.2.3]:80")); w.Code != 200 { // v4-in-v6
		t.Fatalf("mapped IP: %d %s", w.Code, w.Body)
	}
}

func TestTokenFailedRequestLimit(t *testing.T) {
	e := newEnvWith(t, 3, 1000)
	for i := 0; i < 3; i++ {
		if w := e.token(grantForm, withBasic(e.id, "wrong")); w.Code != 401 {
			t.Fatalf("fail %d: %d", i, w.Code)
		}
	}
	// blocked even with the right secret
	w := e.token(grantForm, withBasic(e.id, e.secret))
	if w.Code != http.StatusTooManyRequests || !strings.Contains(w.Body.String(), `"rate_limited"`) ||
		w.Header().Get("Retry-After") == "" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("limit: %d %v %s", w.Code, w.Header(), w.Body)
	}
}

func TestTokenSuccessResetsClientCounterOnly(t *testing.T) {
	e := newEnvWith(t, 100, 1000)
	e.token(grantForm, withBasic(e.id, "wrong"))
	clientKey, ipKey := "rate:ext:token:client:"+e.id, "rate:ext:token:ip:"+testIP
	if !e.mr.Exists(clientKey) || !e.mr.Exists(ipKey) {
		t.Fatalf("failure not counted: %v", e.mr.Keys())
	}
	if w := e.token(grantForm, withBasic(e.id, e.secret)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if e.mr.Exists(clientKey) || !e.mr.Exists(ipKey) {
		t.Fatalf("keys after success: %v", e.mr.Keys())
	}
}

func TestTokenFailsClosedWhenRedisDown(t *testing.T) {
	e := newEnv(t)
	e.mr.Close()
	w := e.token(grantForm, withBasic(e.id, e.secret))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"service_unavailable"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestTokenClientLookupErrorIs500WithoutDetail(t *testing.T) {
	e := newEnv(t)
	e.clients.err = errors.New("pq: password authentication failed for 10.7.7.7")
	w := e.token(grantForm, withBasic(e.id, e.secret))
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"server_error"`) || strings.Contains(w.Body.String(), "10.7.7.7") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestWriteTokenErrorUnknownCodeIsServerError(t *testing.T) {
	w := httptest.NewRecorder()
	writeTokenError(w, "made_up")
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"server_error"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
