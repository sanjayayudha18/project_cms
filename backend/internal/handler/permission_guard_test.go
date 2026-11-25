package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// permission-catalog-coverage 4.3: viewGuard replaces the role guard with the
// permission check, actionGuard stacks both, and nil evaluator = role guard only.

type fakePermReader struct {
	allowed bool
	gotKeys []string
}

func (f *fakePermReader) HasAnyPermission(_ context.Context, _ string, keys []string) (bool, error) {
	f.gotKeys = keys
	return f.allowed, nil
}

func TestPermissionGuards(t *testing.T) {
	keys := []string{"replenish.vendor-requests", "replenish.forecast-browser"}
	roles := []string{"ATM-USER"}
	cases := []struct {
		name     string
		action   bool
		inject   bool
		allowed  bool
		role     string
		wantCode int
	}{
		{"view nil evaluator: role in", false, false, false, "ATM-USER", http.StatusOK},
		{"view nil evaluator: role out", false, false, true, "ACM-USER", http.StatusForbidden},
		{"view: key granted, role out of old list", false, true, true, "ACM-USER", http.StatusOK},
		{"view: no key, role in old list", false, true, false, "ATM-USER", http.StatusForbidden},
		{"action nil evaluator: role in", true, false, false, "ATM-USER", http.StatusOK},
		{"action: key + role", true, true, true, "ATM-USER", http.StatusOK},
		{"action: key, role out", true, true, true, "ACM-USER", http.StatusForbidden},
		{"action: role, no key", true, true, false, "ATM-USER", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testTokenService()
			reader := &fakePermReader{allowed: tc.allowed}
			var e *rolemgmt.PermissionEvaluator
			if tc.inject {
				e = rolemgmt.NewPermissionEvaluator(reader)
			}
			guard := viewGuard
			if tc.action {
				guard = actionGuard
			}
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).With(guard(e, keys, roles)...).Get("/x", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			rec := doRequest(r, http.MethodGet, "/x", tokenForRole(t, ts, 1, tc.role), "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.inject && len(reader.gotKeys) != len(keys) {
				t.Fatalf("permission query keys = %v, want %v", reader.gotKeys, keys)
			}
		})
	}
}

// Holidays keep ADMIN behind forecasting.holidays once the evaluator is injected.
func TestAdminHolidayHandler_WithPermissions(t *testing.T) {
	cases := []struct {
		role     string
		allowed  bool
		wantCode int
	}{
		{"ADMIN", true, http.StatusOK},
		{"ADMIN", false, http.StatusForbidden},
		{"ADMIN_PARAM", true, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			ts := testTokenService()
			reader := &fakePermReader{allowed: tc.allowed}
			h := NewAdminHolidayHandler(&fakeHolidays{}).WithPermissions(rolemgmt.NewPermissionEvaluator(reader))
			r := chi.NewRouter()
			r.With(custommw.RequireAuth(ts)).Mount("/h", h.Routes())
			rec := doRequest(r, http.MethodGet, "/h", tokenForRole(t, ts, 1, tc.role), "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if len(reader.gotKeys) != 1 || reader.gotKeys[0] != "forecasting.holidays" {
				t.Fatalf("keys = %v", reader.gotKeys)
			}
		})
	}
}
