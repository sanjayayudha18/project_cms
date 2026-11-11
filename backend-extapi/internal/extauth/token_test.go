package extauth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	testKey   = []byte("0123456789abcdef0123456789abcdef")
	testNow   = time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	testIss   = "crown"
	testTTL   = 15 * time.Minute
	testCliID = "crown_aaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func newTestTokenService() *TokenService {
	ts := NewTokenService(testKey, testIss, testTTL)
	ts.now = func() time.Time { return testNow }
	return ts
}

func testClient() Client {
	return Client{ClientID: testCliID, CredentialVersion: 3, Scopes: []string{ScopeUsersRead, ScopeUsersWrite}}
}

// validClaims is a map that Validate accepts; cases delete or override keys.
func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":   testIss,
		"aud":   []string{Audience},
		"sub":   testCliID,
		"scope": "users:read users:write",
		"ver":   3,
		"jti":   "6f1c2d3e-0000-4000-8000-000000000001",
		"iat":   testNow.Unix(),
		"exp":   testNow.Add(testTTL).Unix(),
	}
}

func signHS(t *testing.T, method jwt.SigningMethod, key []byte, typ any, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	if typ != nil {
		tok.Header["typ"] = typ
	} else {
		delete(tok.Header, "typ")
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIssueThenValidate(t *testing.T) {
	ts := newTestTokenService()
	tok, expiresIn, err := ts.Issue(testClient(), []string{ScopeUsersWrite, ScopeUsersRead})
	if err != nil {
		t.Fatal(err)
	}
	if expiresIn != 900 {
		t.Errorf("expiresIn = %d, want 900", expiresIn)
	}

	parsed, _, err := jwt.NewParser().ParseUnverified(tok, jwt.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header["alg"] != "HS256" || parsed.Header["typ"] != TokenType {
		t.Errorf("header = %v", parsed.Header)
	}
	for _, k := range []string{"iss", "aud", "sub", "scope", "ver", "jti", "iat", "exp"} {
		if _, ok := parsed.Claims.(jwt.MapClaims)[k]; !ok {
			t.Errorf("claim %q missing", k)
		}
	}
	for _, k := range []string{"id", "username", "role"} { // pkg/auth user claims
		if _, ok := parsed.Claims.(jwt.MapClaims)[k]; ok {
			t.Errorf("unexpected user claim %q", k)
		}
	}

	c, err := ts.Validate(tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if c.Subject != testCliID || c.Ver != 3 || c.Issuer != testIss || c.ID == "" {
		t.Errorf("claims = %+v", c)
	}
	if got := strings.Join(c.Scopes(), " "); got != "users:write users:read" {
		t.Errorf("scopes = %q", got)
	}
	if !c.ExpiresAt.Equal(testNow.Add(testTTL)) || !c.IssuedAt.Equal(testNow) {
		t.Errorf("iat/exp = %v/%v", c.IssuedAt, c.ExpiresAt)
	}
	if len(c.Audience) != 1 || c.Audience[0] != Audience {
		t.Errorf("aud = %v", c.Audience)
	}
}

func TestIssueUniqueJTI(t *testing.T) {
	ts := newTestTokenService()
	a, _, _ := ts.Issue(testClient(), []string{ScopeUsersRead})
	b, _, _ := ts.Issue(testClient(), []string{ScopeUsersRead})
	ca, _ := ts.Validate(a)
	cb, _ := ts.Validate(b)
	if ca == nil || cb == nil || ca.ID == cb.ID {
		t.Fatal("jti must be unique per token")
	}
}

func TestIssueRejects(t *testing.T) {
	ts := newTestTokenService()
	readOnly := testClient()
	readOnly.Scopes = []string{ScopeUsersRead}
	noID := testClient()
	noID.ClientID = ""
	noVer := testClient()
	noVer.CredentialVersion = 0

	tests := []struct {
		name   string
		client Client
		scopes []string
	}{
		{"no scope", testClient(), nil},
		{"unknown scope", testClient(), []string{"admin"}},
		{"scope not allowed for client", readOnly, []string{ScopeUsersWrite}},
		{"no client id", noID, []string{ScopeUsersRead}},
		{"credential version 0", noVer, []string{ScopeUsersRead}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tok, _, err := ts.Issue(tt.client, tt.scopes); err == nil {
				t.Fatalf("Issue succeeded: %q", tok)
			}
		})
	}
}

func TestValidateRejects(t *testing.T) {
	ts := newTestTokenService()
	without := func(k string) jwt.MapClaims { c := validClaims(); delete(c, k); return c }
	with := func(k string, v any) jwt.MapClaims { c := validClaims(); c[k] = v; return c }

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rs256, err := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims()).SignedString(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	noneTok := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims())
	noneTok.Header["typ"] = TokenType
	none, err := noneTok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{"alg none", none},
		{"RS256", rs256},
		{"HS512", signHS(t, jwt.SigningMethodHS512, testKey, TokenType, validClaims())},
		{"wrong key (e.g. JWT_SECRET)", signHS(t, jwt.SigningMethodHS256, []byte("another-secret-of-32-bytes-xxxxx"), TokenType, validClaims())},
		{"typ missing", signHS(t, jwt.SigningMethodHS256, testKey, nil, validClaims())},
		{"typ JWT", signHS(t, jwt.SigningMethodHS256, testKey, "JWT", validClaims())},
		{"wrong aud", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("aud", []string{"crown"}))},
		{"no aud", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("aud"))},
		{"wrong iss", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("iss", "evil"))},
		{"no iss", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("iss"))},
		{"expired", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("exp", testNow.Add(-time.Second).Unix()))},
		{"iat in future", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("iat", testNow.Add(time.Minute).Unix()))},
		{"no exp", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("exp"))},
		{"no iat", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("iat"))},
		{"no sub", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("sub"))},
		{"no jti", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("jti"))},
		{"no ver", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("ver"))},
		{"ver 0", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("ver", 0))},
		{"no scope", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, without("scope"))},
		{"blank scope", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("scope", "  "))},
		{"unknown scope", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("scope", "users:read admin"))},
		{"comma scope", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, with("scope", "users:read,users:write"))},
		{"user token shape", signHS(t, jwt.SigningMethodHS256, testKey, TokenType, jwt.MapClaims{
			"id": 1, "username": "u", "role": "ADMIN", "iat": testNow.Unix(), "exp": testNow.Add(time.Minute).Unix(),
		})},
		{"garbage", "not.a.jwt"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ts.Validate(tt.token)
			if err == nil {
				t.Fatalf("Validate accepted token, claims = %+v", c)
			}
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestValidateAcceptsHandBuiltToken(t *testing.T) {
	// Guards the reject table: its base claims really are valid.
	ts := newTestTokenService()
	if _, err := ts.Validate(signHS(t, jwt.SigningMethodHS256, testKey, TokenType, validClaims())); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateExpiresWithClock(t *testing.T) {
	ts := newTestTokenService()
	tok, _, err := ts.Issue(testClient(), []string{ScopeUsersRead})
	if err != nil {
		t.Fatal(err)
	}
	ts.now = func() time.Time { return testNow.Add(testTTL - time.Second) }
	if _, err := ts.Validate(tok); err != nil {
		t.Fatalf("just before exp: %v", err)
	}
	ts.now = func() time.Time { return testNow.Add(testTTL + time.Second) }
	if _, err := ts.Validate(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("after exp: err = %v", err)
	}
}

func TestValidateRejectsOtherIssuerService(t *testing.T) {
	tok, _, err := newTestTokenService().Issue(testClient(), []string{ScopeUsersRead})
	if err != nil {
		t.Fatal(err)
	}
	other := NewTokenService(testKey, "crown-staging", testTTL)
	other.now = func() time.Time { return testNow }
	if _, err := other.Validate(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}
