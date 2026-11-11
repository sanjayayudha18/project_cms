package extauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"pgregory.net/rapid"

	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

const (
	mwIPInside  = "10.0.0.7"
	mwIPOutside = "192.168.1.1"
)

type fakeReader struct {
	client *Client
	err    error
	calls  int
}

func (f *fakeReader) GetByClientID(_ context.Context, id string) (*Client, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.client == nil || f.client.ClientID != id {
		return nil, nil
	}
	c := *f.client
	return &c, nil
}

func mwClient() Client {
	c := testClient()
	c.IsActive = true
	c.ServiceUserID = 42
	c.AllowedCIDRs = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
	return c
}

func mustIssue(t testing.TB, c Client, scopes ...string) string {
	t.Helper()
	tok, _, err := newTestTokenService().Issue(c, scopes)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// seen records what reached the protected handler.
type seen struct {
	called  bool
	cc      *ClientContext
	hasAuth bool
}

func chain(tokens *TokenService, reader ClientReader, rate *APIRate, scope string, s *seen) http.Handler {
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.called = true
		s.cc, _ = GetClientContext(r.Context())
		_, s.hasAuth = custommw.GetAuthContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	return RequireClient(tokens, reader, rate)(RequireScope(scope)(final))
}

func do(h http.Handler, auth, remote string, lf *custommw.LogFields) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/ext/v1/users/1", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.RemoteAddr = remote
	if lf != nil {
		req = req.WithContext(custommw.WithLogFields(req.Context(), lf))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t tHelper, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	if body["message"] == "" {
		t.Errorf("error body without message: %q", rec.Body.String())
	}
	return body["error"]
}

func TestRequireClient_Errors(t *testing.T) {
	good := mwClient()
	inactive := good
	inactive.IsActive = false
	rotated := good
	rotated.CredentialVersion = good.CredentialVersion + 1
	noCIDR := good
	noCIDR.AllowedCIDRs = nil
	readOnly := good
	readOnly.Scopes = []string{ScopeUsersRead}

	tok := mustIssue(t, good, ScopeUsersRead, ScopeUsersWrite)
	readTok := mustIssue(t, good, ScopeUsersRead)
	userTok := signHS(t, jwt.SigningMethodHS256, testKey, nil, jwt.MapClaims{"id": 1, "username": "u", "role": "ADMIN", "exp": testNow.Unix() + 60})

	tests := []struct {
		name       string
		client     *Client
		readerErr  error
		auth       string
		remote     string
		scope      string
		wantStatus int
		wantCode   string
		wantWWW    string
		wantLogID  bool
	}{
		{name: "no header", client: &good, remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "basic scheme", client: &good, auth: "Basic " + tok, remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "empty bearer", client: &good, auth: "Bearer ", remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "garbage token", client: &good, auth: "Bearer abc.def.ghi", remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "user token", client: &good, auth: "Bearer " + userTok, remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "client missing", auth: "Bearer " + tok, remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "client disabled", client: &inactive, auth: "Bearer " + tok, remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "secret rotated", client: &rotated, auth: "Bearer " + tok, remote: mwIPInside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "disabled wins over bad IP", client: &inactive, auth: "Bearer " + tok, remote: mwIPOutside, wantStatus: 401, wantCode: "invalid_token", wantWWW: bearerInvalidToken},
		{name: "lookup error", client: &good, readerErr: errors.New("db down"), auth: "Bearer " + tok, remote: mwIPInside, wantStatus: 500, wantCode: "internal_error"},
		{name: "ip outside", client: &good, auth: "Bearer " + tok, remote: mwIPOutside, wantStatus: 403, wantCode: "ip_not_allowed", wantLogID: true},
		{name: "empty cidr list", client: &noCIDR, auth: "Bearer " + tok, remote: mwIPInside, wantStatus: 403, wantCode: "ip_not_allowed", wantLogID: true},
		{name: "unparsable remote", client: &good, auth: "Bearer " + tok, remote: "not-an-ip", wantStatus: 403, wantCode: "ip_not_allowed", wantLogID: true},
		{name: "scope missing in token", client: &good, auth: "Bearer " + readTok, remote: mwIPInside, scope: ScopeUsersWrite, wantStatus: 403, wantCode: "insufficient_scope", wantWWW: bearerInsufficientScope, wantLogID: true},
		{name: "scope removed from client", client: &readOnly, auth: "Bearer " + tok, remote: mwIPInside, scope: ScopeUsersWrite, wantStatus: 403, wantCode: "insufficient_scope", wantWWW: bearerInsufficientScope, wantLogID: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, rdb := newMiniRedis(t)
			scope := tt.scope
			if scope == "" {
				scope = ScopeUsersRead
			}
			s := &seen{}
			reader := &fakeReader{client: tt.client, err: tt.readerErr}
			lf := &custommw.LogFields{}
			rec := do(chain(newTestTokenService(), reader, NewAPIRate(rdb, 100), scope, s), tt.auth, tt.remote, lf)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := errorCode(t, rec); got != tt.wantCode {
				t.Errorf("error = %q, want %q", got, tt.wantCode)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != tt.wantWWW {
				t.Errorf("WWW-Authenticate = %q, want %q", got, tt.wantWWW)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			if s.called {
				t.Error("protected handler reached")
			}
			if (lf.ClientID != "") != tt.wantLogID {
				t.Errorf("LogFields.ClientID = %q, wantLogID %v", lf.ClientID, tt.wantLogID)
			}
			// The rate counter is touched only after client and IP pass.
			wantRateKeys := tt.wantCode == "insufficient_scope"
			if got := len(mr.Keys()) > 0; got != wantRateKeys {
				t.Errorf("rate keys = %v, want present %v", mr.Keys(), wantRateKeys)
			}
		})
	}
}

func TestRequireClient_Success(t *testing.T) {
	_, rdb := newMiniRedis(t)
	c := mwClient()
	tok := mustIssue(t, c, ScopeUsersWrite)

	for _, tc := range []struct{ auth, remote string }{
		{"Bearer " + tok, mwIPInside},
		{"bearer " + tok, mwIPInside + ":51234"},
		{"Bearer " + tok, "[::ffff:" + mwIPInside + "]:443"},
	} {
		s := &seen{}
		lf := &custommw.LogFields{}
		rec := do(chain(newTestTokenService(), &fakeReader{client: &c}, NewAPIRate(rdb, 100), ScopeUsersWrite, s), tc.auth, tc.remote, lf)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%q from %q: status %d body %s", tc.auth[:7], tc.remote, rec.Code, rec.Body.String())
		}
		if s.cc == nil || s.cc.Client.ClientID != c.ClientID || s.cc.Client.ServiceUserID != 42 {
			t.Fatalf("ClientContext = %+v", s.cc)
		}
		if s.cc.IP != mwIPInside {
			t.Errorf("IP = %q", s.cc.IP)
		}
		if !slices.Equal(s.cc.GrantedScopes, []string{ScopeUsersWrite}) {
			t.Errorf("GrantedScopes = %v", s.cc.GrantedScopes)
		}
		if s.hasAuth {
			t.Error("AuthContext present (Req 3.6)")
		}
		if lf.ClientID != c.ClientID {
			t.Errorf("LogFields.ClientID = %q", lf.ClientID)
		}
	}
}

func TestRequireClient_NoLogFieldsHolder(t *testing.T) {
	_, rdb := newMiniRedis(t)
	c := mwClient()
	rec := do(chain(newTestTokenService(), &fakeReader{client: &c}, NewAPIRate(rdb, 100), ScopeUsersRead, &seen{}),
		"Bearer "+mustIssue(t, c, ScopeUsersRead), mwIPInside, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRequireClient_RateLimit(t *testing.T) {
	_, rdb := newMiniRedis(t)
	c := mwClient()
	h := chain(newTestTokenService(), &fakeReader{client: &c}, NewAPIRate(rdb, 2), ScopeUsersRead, &seen{})
	auth := "Bearer " + mustIssue(t, c, ScopeUsersRead)

	for i := range 2 {
		if rec := do(h, auth, mwIPInside, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("request %d: status %d", i+1, rec.Code)
		}
	}
	rec := do(h, auth, mwIPInside, nil)
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "rate_limited" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || ra < 1 || ra > 60 {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
}

func TestRequireClient_RateStoreDown(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	mr.Close()
	c := mwClient()
	s := &seen{}
	rec := do(chain(newTestTokenService(), &fakeReader{client: &c}, NewAPIRate(rdb, 100), ScopeUsersRead, s),
		"Bearer "+mustIssue(t, c, ScopeUsersRead), mwIPInside, nil)
	if rec.Code != http.StatusServiceUnavailable || errorCode(t, rec) != "service_unavailable" || s.called {
		t.Fatalf("status %d body %s called %v", rec.Code, rec.Body.String(), s.called)
	}
}

func TestRequireScope_WithoutRequireClient(t *testing.T) {
	called := false
	h := RequireScope(ScopeUsersRead)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized || called || rec.Header().Get("WWW-Authenticate") != bearerInvalidToken {
		t.Fatalf("status %d called %v", rec.Code, called)
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	for d, want := range map[int64]string{0: "1", 1: "1", 1_500_000_000: "2", 60_000_000_000: "60"} {
		if got := retryAfterSeconds(time.Duration(d)); got != want {
			t.Errorf("retryAfterSeconds(%d ns) = %s, want %s", d, got, want)
		}
	}
}

// Property 7: a cryptographically valid Client_Token is rejected with 401 when
// the client is disabled or its ver differs from credential_version, with 403
// ip_not_allowed when the source IP is outside allowed_cidrs (empty list
// included), and with 403 insufficient_scope when the route scope is missing.
// Validates: Requirements 1.4, 1.5, 1.7, 4.2, 4.3, 4.4.
func TestProperty7_RevokedRotatedMisplacedRejected(t *testing.T) {
	_, rdb := newMiniRedis(t)
	rate := NewAPIRate(rdb, 1<<30)
	tokens := newTestTokenService()

	rapid.Check(t, func(rt *rapid.T) {
		issued := mwClient()
		issued.Scopes = rapid.SliceOfNDistinct(rapid.SampledFrom(knownScopes), 1, 2, rapid.ID[string]).Draw(rt, "client_scopes")
		tokScopes := rapid.SliceOfNDistinct(rapid.SampledFrom(issued.Scopes), 1, len(issued.Scopes), rapid.ID[string]).Draw(rt, "token_scopes")
		tok, _, err := tokens.Issue(issued, tokScopes)
		if err != nil {
			rt.Fatal(err)
		}

		// State of the client row at request time.
		now := issued
		now.IsActive = rapid.Bool().Draw(rt, "active")
		rotated := rapid.Bool().Draw(rt, "rotated")
		if rotated {
			now.CredentialVersion += rapid.Int32Range(1, 5).Draw(rt, "ver_bump")
		}
		emptyCIDRs := rapid.Bool().Draw(rt, "empty_cidrs")
		if emptyCIDRs {
			now.AllowedCIDRs = nil
		}
		inside := rapid.Bool().Draw(rt, "ip_inside")
		remote := mwIPOutside
		if inside {
			remote = netip.AddrFrom4([4]byte{10, 0, 0, rapid.ByteRange(1, 254).Draw(rt, "host")}).String()
		}
		routeScope := rapid.SampledFrom(knownScopes).Draw(rt, "route_scope")

		s := &seen{}
		lf := &custommw.LogFields{}
		rec := do(chain(tokens, &fakeReader{client: &now}, rate, routeScope, s), "Bearer "+tok, remote, lf)

		wantStatus, wantCode := http.StatusNoContent, ""
		switch {
		case !now.IsActive || rotated:
			wantStatus, wantCode = http.StatusUnauthorized, "invalid_token"
		case emptyCIDRs || !inside:
			wantStatus, wantCode = http.StatusForbidden, "ip_not_allowed"
		case !slices.Contains(tokScopes, routeScope):
			wantStatus, wantCode = http.StatusForbidden, "insufficient_scope"
		}
		if rec.Code != wantStatus {
			rt.Fatalf("status = %d, want %d (body %s)", rec.Code, wantStatus, rec.Body.String())
		}
		if wantCode != "" {
			if got := errorCode(rt, rec); got != wantCode {
				rt.Fatalf("error = %q, want %q", got, wantCode)
			}
		}
		if s.called != (wantStatus == http.StatusNoContent) {
			rt.Fatalf("handler called = %v", s.called)
		}
		if s.hasAuth {
			rt.Fatal("AuthContext present")
		}
		if wantStatus == http.StatusUnauthorized && lf.ClientID != "" {
			rt.Fatalf("client_id logged for rejected token: %q", lf.ClientID)
		}
	})
}

// tHelper is the part of *testing.T and *rapid.T that errorCode needs.
type tHelper interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}
