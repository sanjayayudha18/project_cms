//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
)

// role-management-revamp 4.4 on real Postgres: a role made through roles.party_scope behaves like the
// seeded vendor roles (Property 6), ListVendorRolesForParty keeps the old vendorrole.ForParty mapping,
// and a scope-read failure never grants access (Property 7).

// scopeReaderFunc is a ScopeReader stub (error or fixed scope).
type scopeReaderFunc func(ctx context.Context, role string) ([]string, error)

func (f scopeReaderFunc) PartyScope(ctx context.Context, role string) ([]string, error) {
	return f(ctx, role)
}

// seedVendorRole inserts a vendor role with the given scope; removed after the users that hold it.
func seedVendorRole(t *testing.T, f *vendorOrderFixture, scope []string) string {
	t.Helper()
	name := "VENDOR-T44-" + uuid.NewString()[:8]
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO roles (role, description, portal, party_scope) VALUES ($1, 'test 4.4', 'vendor', $2)`, name, scope); err != nil {
		t.Fatalf("seed role %s: %v", name, err)
	}
	return name
}

// deleteRolesLater is registered before newVendorOrderFixture, so it runs after the users are deleted (LIFO).
func deleteRolesLater(t *testing.T, names *[]string) {
	t.Helper()
	t.Cleanup(func() {
		if len(*names) == 0 {
			return
		}
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			t.Errorf("cleanup roles: %v", err)
			return
		}
		defer func() { _ = conn.Close(ctx) }()
		if _, err := conn.Exec(ctx, `DELETE FROM roles WHERE role = ANY($1)`, *names); err != nil {
			t.Errorf("cleanup roles %v: %v", *names, err)
		}
	})
}

// scopeOutcome is what one actor sees: the list, the parties Get returns, and the realization list.
type scopeOutcome struct {
	listed  []int64
	visible []int64
	realize string // "ok" | "forbidden"
}

func (f *vendorOrderFixture) outcome(t *testing.T, va VendorActor, parties []int64) scopeOutcome {
	t.Helper()
	ctx := context.Background()
	var o scopeOutcome
	l, err := f.orders.List(ctx, va, VendorOrderFilter{})
	if err != nil {
		t.Fatalf("%s list: %v", va.Role, err)
	}
	for _, it := range l.Items {
		o.listed = append(o.listed, it.ID)
	}
	slices.Sort(o.listed)
	for _, p := range parties {
		switch _, err := f.orders.Get(ctx, va, p); {
		case err == nil:
			o.visible = append(o.visible, p)
		case !errors.Is(err, ErrVendorOrderNotFound):
			t.Fatalf("%s get %d: %v", va.Role, p, err)
		}
	}
	today := jakartaCalendarDate(0)
	switch _, err := f.orders.ListRealizations(ctx, va, ReplenishmentFilter{From: today.AddDate(0, 0, -7), To: today}); {
	case err == nil:
		o.realize = "ok"
	case errors.Is(err, ErrNotAuthorized):
		o.realize = "forbidden"
	default:
		t.Fatalf("%s realizations: %v", va.Role, err)
	}
	return o
}

func (f *vendorOrderFixture) orderOutcome(t *testing.T, role string, branch *int64, parties []int64) scopeOutcome {
	t.Helper()
	return f.outcome(t, f.vendorUserAs(t, role, f.vendorID, branch), parties)
}

func (f *vendorOrderFixture) allParties(t *testing.T) []int64 {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT id FROM vendor_request_vendor_parties WHERE vendor_request_id = $1 ORDER BY id`, f.reqID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil || len(ids) == 0 {
		t.Fatalf("parties of request %d: %v (%d)", f.reqID, err, len(ids))
	}
	return ids
}

func TestIntegration_VendorPartyScope_NewRoleMatchesSeededRole(t *testing.T) { // Property 6
	var roles []string
	deleteRolesLater(t, &roles)
	f := newVendorOrderFixture(t)
	parties := f.allParties(t)

	for _, tc := range []struct {
		name   string
		scope  []string
		legacy string
	}{
		{"replenish", []string{"replenish"}, vendorrole.ATMUser},
		{"vault", []string{"vault"}, vendorrole.VaultUser},
		{"replenish+vault", []string{"replenish", "vault"}, vendorrole.User},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newRole := seedVendorRole(t, f, tc.scope)
			roles = append(roles, newRole)
			pinned := &f.branchID
			if tc.name == "vault" {
				pinned = &f.cash1 // the fixture's vault party lives at cash1, not at the replenish branch
			}
			for _, branch := range []*int64{nil, pinned} {
				legacy := f.orderOutcome(t, tc.legacy, branch, parties)
				got := f.orderOutcome(t, newRole, branch, parties)
				if !slices.Equal(got.listed, legacy.listed) || !slices.Equal(got.visible, legacy.visible) || got.realize != legacy.realize {
					t.Errorf("branch=%v: %s = %+v, want same as %s = %+v", branch != nil, newRole, got, tc.legacy, legacy)
				}
				if len(legacy.listed) == 0 {
					t.Errorf("fixture gave %s an empty list: the comparison proves nothing", tc.legacy)
				}
			}
		})
	}
}

func TestIntegration_VendorPartyScope_ListVendorRolesForParty(t *testing.T) {
	var roles []string
	deleteRolesLater(t, &roles)
	f := newVendorOrderFixture(t)
	repo := rolemgmt.NewRepository(f.pool, f.pool)
	ctx := context.Background()

	// Fixed expectations = the deleted vendorrole.ForParty (git show HEAD:…/vendorrole.go):
	// replenish -> [VENDOR-USER, VENDOR-ATM-USER], vault -> [VENDOR-USER, VENDOR-VAULT-USER].
	legacy := []string{"VENDOR-ATM-USER", "VENDOR-USER", "VENDOR-VAULT-USER"}
	onlyLegacy := func(in []string) []string {
		var out []string
		for _, r := range in {
			if slices.Contains(legacy, r) {
				out = append(out, r)
			}
		}
		return out
	}
	extra := seedVendorRole(t, f, []string{"vault"})
	roles = append(roles, extra)

	for _, tc := range []struct {
		party     string
		want      []string // legacy roles only, sorted
		wantExtra bool
	}{
		{"replenish", []string{"VENDOR-ATM-USER", "VENDOR-USER"}, false},
		{"vault", []string{"VENDOR-USER", "VENDOR-VAULT-USER"}, true},
		{"cashcount", nil, false}, // never a party notice recipient (old ForParty -> nil)
		{"unknown", nil, false},
	} {
		got, err := repo.ListVendorRolesForParty(ctx, tc.party)
		if err != nil {
			t.Fatalf("%s: %v", tc.party, err)
		}
		if l := onlyLegacy(got); !slices.Equal(l, tc.want) {
			t.Errorf("%s: legacy roles = %v, want %v", tc.party, l, tc.want)
		}
		if has := slices.Contains(got, extra); has != tc.wantExtra {
			t.Errorf("%s: new role listed = %v, want %v", tc.party, has, tc.wantExtra)
		}
	}

	// Old PartyRoles values as fixed expectations, read through the scope reader.
	for role, want := range map[string][]string{
		"VENDOR-USER":       {"replenish", "vault"},
		"VENDOR-ATM-USER":   {"replenish"},
		"VENDOR-VAULT-USER": {"vault"},
		"ADMIN":             nil,
		"NO-SUCH-ROLE":      nil,
	} {
		got, err := repo.PartyScope(ctx, role)
		if err != nil {
			t.Fatalf("PartyScope(%s): %v", role, err)
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("PartyScope(%s) = %v, want %v", role, got, want)
		}
	}
}

func TestIntegration_VendorPartyScope_FailClosed(t *testing.T) { // Property 7
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	va := f.wideUser
	parties := f.allParties(t)
	boom := errors.New("replica down")
	rf := ReplenishmentFilter{From: jakartaCalendarDate(-7), To: jakartaCalendarDate(0)}

	for _, tc := range []struct {
		name    string
		reader  scopeReaderFunc
		wantErr func(error) bool
	}{
		{"read error", func(context.Context, string) ([]string, error) { return nil, boom }, func(e error) bool { return errors.Is(e, boom) }},
		{"empty scope", func(context.Context, string) ([]string, error) { return nil, nil }, func(e error) bool { return errors.Is(e, ErrNotAuthorized) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewVendorOrderService(f.pool, f.pool, tc.reader)
			calls := map[string]error{}
			_, calls["List"] = svc.List(ctx, va, VendorOrderFilter{})
			_, calls["Get"] = svc.Get(ctx, va, parties[0])
			_, calls["Accept"] = svc.Accept(ctx, va, parties[0])
			_, calls["Reject"] = svc.Reject(ctx, va, parties[0], "armada tidak tersedia")
			_, calls["ListRealizations"] = svc.ListRealizations(ctx, va, rf)
			for name, err := range calls {
				if err == nil || !tc.wantErr(err) {
					t.Errorf("%s: err = %v, want a refusal", name, err)
				}
			}
			if n := countRows(t, f.pool, `SELECT count(*) FROM vendor_request_vendor_parties WHERE vendor_request_id = $1 AND status <> 'pending'`, f.reqID); n != 0 {
				t.Fatalf("refused calls decided %d parties", n)
			}
		})
	}
}
