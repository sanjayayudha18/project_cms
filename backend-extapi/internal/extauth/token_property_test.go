package extauth

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/pkg/auth"
	"pgregory.net/rapid"
)

var knownScopes = []string{ScopeUsersRead, ScopeUsersWrite}

func genKey(t *rapid.T, label string) []byte {
	return rapid.SliceOfN(rapid.Byte(), 32, 64).Draw(t, label)
}

func genClient(t *rapid.T) Client {
	return Client{
		ClientID:          "crown_" + rapid.StringMatching(`[a-z0-9]{26}`).Draw(t, "client_id"),
		CredentialVersion: rapid.Int32Range(1, 1000).Draw(t, "ver"),
		Scopes:            rapid.SliceOfNDistinct(rapid.SampledFrom(knownScopes), 0, 2, rapid.ID[string]).Draw(t, "client_scopes"),
	}
}

// Property 1: a Client_Token is rejected by pkg/auth.ValidateAccessToken and a
// User_Token is rejected by extauth Validate, also when both share one key.
// Validates: Requirements 3.1, 3.2, 3.3, 3.6.
func TestProperty1_TokensNotInterchangeable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		extKey := genKey(t, "ext_key")
		userKey := extKey
		if !rapid.Bool().Draw(t, "same_key") {
			userKey = genKey(t, "user_key")
		}
		ext := NewTokenService(extKey, rapid.SampledFrom([]string{"crown", "crown-dev"}).Draw(t, "iss"), 15*time.Minute)
		usr := auth.NewTokenService(auth.TokenConfig{
			SecretKey:          userKey,
			AccessTokenExpiry:  15 * time.Minute,
			SessionMaxLifetime: time.Hour,
		}, nil)

		c := genClient(t)
		c.Scopes = rapid.SliceOfNDistinct(rapid.SampledFrom(knownScopes), 1, 2, rapid.ID[string]).Draw(t, "scopes")
		clientTok, _, err := ext.Issue(c, c.Scopes)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		vendorID := rapid.Int64Range(1, 1<<40).Draw(t, "vendor_id")
		id := &auth.AuthIdentity{
			UserID:     rapid.Int64Range(1, 1<<40).Draw(t, "user_id"),
			Username:   rapid.StringMatching(`[a-z][a-z0-9._]{2,30}`).Draw(t, "username"),
			Role:       rapid.SampledFrom([]string{"ADMIN", "APPACCESS", "ATM-USER", "VENDOR-USER", "EXT-CLIENT"}).Draw(t, "role"),
			IsKaryawan: rapid.Bool().Draw(t, "karyawan"),
		}
		if rapid.Bool().Draw(t, "has_vendor") {
			id.VendorID = &vendorID
		}
		userTok, _, err := usr.GenerateTokenPair(id)
		if err != nil {
			t.Fatalf("GenerateTokenPair: %v", err)
		}

		// Sanity: each service accepts its own token, so the rejections below
		// come from the token kind, not from a broken setup.
		if _, err := ext.Validate(clientTok); err != nil {
			t.Fatalf("extauth rejects its own token: %v", err)
		}
		if _, err := usr.ValidateAccessToken(userTok); err != nil {
			t.Fatalf("pkg/auth rejects its own token: %v", err)
		}

		if claims, err := usr.ValidateAccessToken(clientTok); err == nil {
			t.Fatalf("pkg/auth accepted a Client_Token: %+v", claims)
		}
		if claims, err := ext.Validate(userTok); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("extauth accepted a User_Token (err=%v): %+v", err, claims)
		}
	})
}

// Property 9: a token only ever carries scopes from the client's set; a
// request with any scope outside that set (or unknown) yields no token.
// Validates: Requirements 2.1, 2.2, 2.3.
func TestProperty9_GrantedScopeAlwaysSubset(t *testing.T) {
	ts := newTestTokenService()
	rapid.Check(t, func(t *rapid.T) {
		c := genClient(t)
		parts := rapid.SliceOfN(rapid.SampledFrom([]string{
			ScopeUsersRead, ScopeUsersWrite, "users:admin", "USERS:READ", "", "admin",
		}), 0, 5).Draw(t, "requested")
		raw := strings.Join(parts, rapid.SampledFrom([]string{" ", ",", ", "}).Draw(t, "sep"))

		requested, parseErr := ParseScopes(raw)
		allowed := parseErr == nil && len(requested) > 0
		for _, s := range requested {
			allowed = allowed && c.HasScope(s)
		}
		if parseErr != nil {
			requested = parts // hand the raw scopes to Issue: it must refuse too
		}

		tok, _, err := ts.Issue(c, requested)
		if !allowed {
			if err == nil || tok != "" {
				t.Fatalf("token issued for %q with client scopes %v", raw, c.Scopes)
			}
			return
		}
		if err != nil {
			t.Fatalf("Issue(%v) for client scopes %v: %v", requested, c.Scopes, err)
		}
		claims, err := ts.Validate(tok)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		got := claims.Scopes()
		for _, s := range got {
			if !c.HasScope(s) {
				t.Fatalf("token scope %q not in client scopes %v", s, c.Scopes)
			}
		}
		if !slices.Equal(got, requested) {
			t.Fatalf("token scopes %v, requested %v", got, requested)
		}
	})
}
