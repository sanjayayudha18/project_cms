//go:build integration

package repository

import (
	"context"
	"net/netip"
	"os"
	"testing"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Every case runs in a transaction that is always rolled back: nothing persists.
func newTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return ctx, tx
}

func baseParams(ctx context.Context, t *testing.T, tx pgx.Tx) CreateApiClientParams {
	t.Helper()
	q := New(tx)
	svc, err := q.CreateServiceUser(ctx, CreateServiceUserParams{Username: "it-svc-1", FullName: "it svc 1", Email: "it-svc-1@example.test"})
	if err != nil {
		t.Fatalf("service user (migration 039 applied?): %v", err)
	}
	creator, err := q.CreateServiceUser(ctx, CreateServiceUserParams{Username: "it-svc-2", FullName: "it svc 2", Email: "it-svc-2@example.test"})
	if err != nil {
		t.Fatalf("creator user: %v", err)
	}
	id, err := extauth.NewClientID()
	if err != nil {
		t.Fatal(err)
	}
	return CreateApiClientParams{
		ClientID: id, Name: "it-client", SecretHash: "x", Scopes: []string{"users:read"},
		AssignableRoles: []string{}, AllowedCidrs: []netip.Prefix{}, ServiceUserID: svc, CreatedBy: creator,
	}
}

func TestApiClients_CheckAssignableRolesRejectsPrivileged(t *testing.T) {
	for _, role := range []string{"ADMIN", "APPACCESS", "ADMIN_PARAM", "EXT-CLIENT"} {
		t.Run(role, func(t *testing.T) {
			ctx, tx := newTx(t)
			p := baseParams(ctx, t, tx)
			p.AssignableRoles = []string{"ATM-USER", role}
			if _, err := New(tx).CreateApiClient(ctx, p); err == nil {
				t.Fatalf("expected CHECK violation for %s", role)
			}
		})
	}
	t.Run("non-privileged accepted", func(t *testing.T) {
		ctx, tx := newTx(t)
		p := baseParams(ctx, t, tx)
		p.AssignableRoles = []string{"ATM-USER"}
		if _, err := New(tx).CreateApiClient(ctx, p); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})
}

func TestApiClients_CheckClientID(t *testing.T) {
	for _, bad := range []string{"", "crown_short", "CROWN_abcdefghijklmnopqrstuvwxyz", "crown_abcdefghijklmnopqrstuvwxy1"} {
		ctx, tx := newTx(t)
		p := baseParams(ctx, t, tx)
		p.ClientID = bad
		if _, err := New(tx).CreateApiClient(ctx, p); err == nil {
			t.Errorf("client_id %q should violate CHECK", bad)
		}
	}
}

func TestApiClientRepository_CIDRRoundTripAndMissing(t *testing.T) {
	ctx, tx := newTx(t)
	p := baseParams(ctx, t, tx)
	p.AllowedCidrs = []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16"), netip.MustParsePrefix("192.168.5.7/32")}
	if _, err := New(tx).CreateApiClient(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	repo := NewApiClientRepository(tx)
	c, err := repo.GetByClientID(ctx, p.ClientID)
	if err != nil || c == nil {
		t.Fatalf("get: %v %v", c, err)
	}
	if len(c.AllowedCIDRs) != 2 || c.AllowedCIDRs[0] != p.AllowedCidrs[0] || c.AllowedCIDRs[1] != p.AllowedCidrs[1] {
		t.Errorf("cidrs = %v, want %v", c.AllowedCIDRs, p.AllowedCidrs)
	}
	if !c.AllowsIP(netip.MustParseAddr("10.1.2.3")) || c.AllowsIP(netip.MustParseAddr("10.2.0.1")) {
		t.Error("AllowsIP mismatch after round trip")
	}
	if !c.IsActive || c.CredentialVersion != 1 {
		t.Errorf("defaults: active=%v ver=%d", c.IsActive, c.CredentialVersion)
	}
	missing, err := repo.GetByClientID(ctx, "crown_aaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil || missing != nil {
		t.Errorf("missing client: got %v, %v; want nil, nil", missing, err)
	}
}
