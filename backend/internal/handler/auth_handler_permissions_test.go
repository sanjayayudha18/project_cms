package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// fakePermKeys is a mutable PermissionKeyReader; changing grants between two
// requests stands in for a PUT role-permissions between them (the PUT itself
// is covered in rolemgmt tests).
type fakePermKeys struct {
	grants map[string][]string
	err    error
}

func (f *fakePermKeys) ListRoleFeatureKeys(_ context.Context, role string) ([]string, error) {
	return f.grants[role], f.err
}

func mountMyPermissions(p PermissionKeyReader) (http.Handler, *pkgauth.TokenService) {
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	h := (&AuthHandler{tokenService: ts}).WithPermissions(p)
	r := chi.NewRouter()
	r.Mount("/api/v1/auth", h.Routes())
	return r, ts
}

func getMyPermissions(t *testing.T, router http.Handler, token string) (int, []string) {
	t.Helper()
	rec := doRequest(router, http.MethodGet, "/api/v1/auth/me/permissions", token, "")
	var body struct {
		Permissions []string `json:"permissions"`
	}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v (%s)", err, rec.Body.String())
		}
		if body.Permissions == nil {
			t.Fatalf("permissions must be [] not null: %s", rec.Body.String())
		}
	}
	return rec.Code, body.Permissions
}

// Property 10: /me/permissions = grants ∪ Always_On ∪ (Locked for APPACCESS/ADMIN);
// non-internal roles get [].
func TestAuthHandler_MyPermissions(t *testing.T) {
	grants := map[string][]string{
		"ADMIN":       {"settings.vendors"},
		"APPACCESS":   {},
		"ATM-USER":    {"replenish.vendor-requests"},
		"VENDOR-USER": {"settings.vendors"}, // stale grant must never leak
	}
	alwaysOn := []string{"dashboard", "approvals"}
	locked := []string{"settings.roles", "settings.roles.create", "settings.roles.edit", "settings.users"}
	cat := func(parts ...[]string) []string {
		out := []string{}
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	tests := []struct {
		role string
		want []string
	}{
		{"ADMIN", cat(grants["ADMIN"], alwaysOn, locked)},
		{"APPACCESS", cat(alwaysOn, locked)},
		{"ATM-USER", cat(grants["ATM-USER"], alwaysOn)},
		{"VENDOR-USER", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			router, ts := mountMyPermissions(&fakePermKeys{grants: grants})
			code, got := getMyPermissions(t, router, tokenForRole(t, ts, 7, tt.role))
			if code != http.StatusOK {
				t.Fatalf("status %d", code)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestAuthHandler_MyPermissions_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountMyPermissions(&fakePermKeys{})
	if code, _ := getMyPermissions(t, router, ""); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestAuthHandler_MyPermissions_ReflectsGrantChange(t *testing.T) {
	f := &fakePermKeys{grants: map[string][]string{"ATM-USER": {"replenish.vendor-requests"}}}
	router, ts := mountMyPermissions(f)
	token := tokenForRole(t, ts, 7, "ATM-USER")

	_, before := getMyPermissions(t, router, token)
	f.grants["ATM-USER"] = []string{"settings.vendors"} // after a PUT
	_, after := getMyPermissions(t, router, token)

	if reflect.DeepEqual(before, after) || after[0] != "settings.vendors" {
		t.Errorf("grant change not reflected: before %v, after %v", before, after)
	}
}

func TestAuthHandler_MyPermissions_RepoError_500(t *testing.T) {
	router, ts := mountMyPermissions(&fakePermKeys{err: errors.New("db down")})
	if code, _ := getMyPermissions(t, router, tokenForRole(t, ts, 7, "ATM-USER")); code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", code)
	}
}
