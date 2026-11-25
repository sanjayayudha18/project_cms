package rolemgmt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/go-chi/chi/v5"

	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// Task 2.5, spec permission-catalog-coverage. Properties 4, 5 and 6 plus the
// 422 assignable_roles rule, exhaustive over small domains (no DB needed).

// grantReader answers HasAnyPermission from a fixed grant set and counts calls.
type grantReader struct {
	granted []string
	err     error
	calls   int
}

func (g *grantReader) HasAnyPermission(_ context.Context, _ string, keys []string) (bool, error) {
	g.calls++
	if g.err != nil {
		return false, g.err
	}
	for _, k := range keys {
		if slices.Contains(g.granted, k) {
			return true, nil
		}
	}
	return false, nil
}

// serve mounts RequirePermission behind RequireAuth and reports whether the
// protected handler ran.
func serve(t *testing.T, reader ReplicaReader, role string, keys ...string) (code int, handlerCalled bool) {
	t.Helper()
	_, tokenSvc := mountRequirePermissionKeys(reader, keys...) // only for the token service
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc), RequirePermission(NewPermissionEvaluator(reader), keys...)).
		Get("/p", func(w http.ResponseWriter, _ *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.Header.Set("Authorization", "Bearer "+tokenForTestRole(t, tokenSvc, role))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code, handlerCalled
}

// Property 4: allowed <=> role internal AND (keys ∩ (grant ∪ Always_On) ≠ ∅).
func TestProperty4_DecisionEqualsEffectivePermissions(t *testing.T) {
	roles := []struct {
		name     string
		internal bool
	}{
		{"ADMIN", true}, {"ATM-USER", true}, {"CC-CWM", true},
		{"VENDOR-USER", false}, {"VENDOR-VAULT-USER", false}, {"EXT-CLIENT", false},
	}
	keyLists := [][]string{
		{"monitoring.audit-logs"},
		{"settings.vendors", "settings.pks"},
		{"approvals"},
		{"settings.pks", "dashboard"},
	}
	grants := [][]string{
		nil,
		{"monitoring.audit-logs"},
		{"settings.pks"},
		{"settings.vendors", "settings.pks"},
		{"settings.atms"},
	}
	for _, role := range roles {
		for _, keys := range keyLists {
			for _, granted := range grants {
				want := false
				if role.internal {
					for _, k := range keys {
						if IsAlwaysOn(k) || slices.Contains(granted, k) {
							want = true
						}
					}
				}
				reader := &grantReader{granted: granted}
				code, called := serve(t, reader, role.name, keys...)
				wantCode := http.StatusForbidden
				if want {
					wantCode = http.StatusOK
				}
				if code != wantCode || called != want {
					t.Errorf("role=%s keys=%v grant=%v: code=%d called=%v, want code=%d called=%v",
						role.name, keys, granted, code, called, wantCode, want)
				}
			}
		}
	}
}

// Property 5: any repository error -> 500 and the handler never runs.
func TestProperty5_FailClosed(t *testing.T) {
	for _, role := range []string{"ADMIN", "ATM-USER", "CC-CWM"} {
		for _, keys := range [][]string{
			{"monitoring.audit-logs"},
			{"settings.vendors", "settings.pks"},
		} {
			reader := &grantReader{err: errBoom, granted: keys} // granted ignored on error
			code, called := serve(t, reader, role, keys...)
			if code != http.StatusInternalServerError || called {
				t.Errorf("role=%s keys=%v: code=%d called=%v, want 500 and handler not called", role, keys, code, called)
			}
			if reader.calls != 1 {
				t.Errorf("role=%s keys=%v: repo calls=%d, want 1", role, keys, reader.calls)
			}
		}
	}
}

// Property 6: for every payload, the stored grants on Always_On/Locked keys
// are the same before and after, including a legacy settings.roles grant.
func TestProperty6_ProtectedKeysImmuneToEditor(t *testing.T) {
	var catalog []db.MenuFeature
	protected := map[int64]bool{}
	id := int64(0)
	add := func(key string) int64 {
		id++
		catalog = append(catalog, db.MenuFeature{ID: id, Key: key, Label: key, Portal: PortalMain})
		return id
	}
	for _, k := range AlwaysOnKeys {
		protected[add(k)] = true
	}
	for _, k := range LockedKeys {
		protected[add(k)] = true
	}
	freeA, freeB := add("monitoring.audit-logs"), add("settings.pks")
	n := len(catalog)

	rolesID := int64(0)
	for _, f := range catalog {
		if f.Key == "settings.roles" {
			rolesID = f.ID
		}
	}

	befores := [][]int64{
		nil,
		{rolesID},        // legacy ADMIN grant on settings.roles
		{rolesID, freeA}, // legacy grant plus a normal one
		{freeB},
	}
	protectedOf := func(ids []int64) []int64 {
		var out []int64
		for _, i := range ids {
			if protected[i] {
				out = append(out, i)
			}
		}
		slices.Sort(out)
		return out
	}

	for _, before := range befores {
		for mask := 0; mask < 1<<n; mask++ {
			var payload []int64
			for i := 0; i < n; i++ {
				if mask&(1<<i) != 0 {
					payload = append(payload, catalog[i].ID)
				}
			}
			got, err := resolvePermissionSet(db.Role{Role: "ADMIN", Portal: PortalMain}, payload, before, catalog)
			if err != nil {
				t.Fatalf("before=%v payload=%v: %v", before, payload, err)
			}
			if !slices.Equal(protectedOf(got), protectedOf(before)) {
				t.Fatalf("before=%v payload=%v: protected grants %v, want %v",
					before, payload, protectedOf(got), protectedOf(before))
			}
			// Free keys follow the payload exactly.
			for _, f := range []int64{freeA, freeB} {
				if slices.Contains(got, f) != slices.Contains(payload, f) {
					t.Fatalf("before=%v payload=%v: free key %d not following payload", before, payload, f)
				}
			}
		}
	}
}

// 422 assignable_roles: every role outside the list is rejected for a new
// grant; every listed role (any case) is accepted.
func TestAssignableRoles_Rejection(t *testing.T) {
	catalog := []db.MenuFeature{{ID: 1, Key: "settings.atms", Label: "ATM", AssignableRoles: []string{"ADMIN", "ADMIN_PARAM"}, Portal: PortalMain}}
	for _, tt := range []struct {
		role string
		ok   bool
	}{
		{"ADMIN", true}, {"admin_param", true}, {"ATM-USER", false}, {"CC-CWM", false}, {"ATM-SPV", false},
	} {
		_, err := resolvePermissionSet(db.Role{Role: tt.role, Portal: PortalMain}, []int64{1}, nil, catalog)
		if (err == nil) != tt.ok {
			t.Errorf("role %s: err=%v, want ok=%v", tt.role, err, tt.ok)
		}
		if err != nil {
			if _, isNAE := err.(*RoleNotAssignableError); !isNAE {
				t.Errorf("role %s: error %T, want *RoleNotAssignableError", tt.role, err)
			}
		}
	}
}
