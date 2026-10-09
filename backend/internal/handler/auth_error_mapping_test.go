package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// Login failures must map to distinct, non-leaky responses: wrong credentials
// are a plain 401, account states are 403 with a machine-readable code, a
// rate limit carries Retry-After, and unknown errors never echo internals.
func TestAuthHandler_HandleAuthError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantBody string
	}{
		{"validation", &pkgauth.ValidationError{Field: "email", Message: "wajib diisi"}, http.StatusUnprocessableEntity, "email"},
		{"rate limited", &pkgauth.RateLimitError{RetryAfter: 60}, http.StatusTooManyRequests, "rate_limited"},
		{"invalid credentials", pkgauth.ErrInvalidCredentials, http.StatusUnauthorized, ""},
		{"wrapped invalid credentials", fmt.Errorf("login: %w", pkgauth.ErrInvalidCredentials), http.StatusUnauthorized, ""},
		{"inactive", pkgauth.ErrAccountInactive, http.StatusForbidden, "account_inactive"},
		{"portal mismatch", pkgauth.ErrPortalMismatch, http.StatusForbidden, "portal_mismatch"},
		{"password expired", pkgauth.ErrPasswordExpired, http.StatusForbidden, "password_expired"},
		{"locked", pkgauth.ErrAccountLocked, http.StatusForbidden, "account_locked"},
		{"ldap not configured", pkgauth.ErrLDAPNotConfigured, http.StatusServiceUnavailable, ""},
		{"service unavailable", pkgauth.ErrServiceUnavailable, http.StatusServiceUnavailable, ""},
		{"unknown", errors.New("pq: connection refused at 10.0.0.5"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&AuthHandler{}).handleAuthError(rec, tc.err)
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("= %d %s, want %d containing %q", rec.Code, rec.Body.String(), tc.wantCode, tc.wantBody)
			}
			if strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Errorf("internal error detail leaked: %s", rec.Body.String())
			}
		})
	}
	rec := httptest.NewRecorder()
	(&AuthHandler{}).handleAuthError(rec, &pkgauth.RateLimitError{RetryAfter: 60})
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
	}
}
