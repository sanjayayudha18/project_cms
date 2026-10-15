package rolemgmt

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeRepo fails the named method with errBoom; everything else succeeds with
// "role exists, catalog valid, no current permissions". Unit-level only: the
// real tx/audit behaviour is covered by service_integration_test.go.
type fakeRepo struct{ fail string }

func (f fakeRepo) err(method string) error {
	if f.fail == method {
		return errBoom
	}
	return nil
}

func (f fakeRepo) ListCatalog(context.Context) ([]db.MenuFeature, error) {
	return nil, f.err("ListCatalog")
}
func (f fakeRepo) CatalogEntriesExist(context.Context, []int64) (bool, error) {
	return true, f.err("CatalogEntriesExist")
}
func (f fakeRepo) ListRolesWithPermissions(context.Context) ([]db.ListRolesWithPermissionsRow, error) {
	return nil, f.err("ListRolesWithPermissions")
}
func (f fakeRepo) FindRoleByName(context.Context, string) (*int64, error) {
	return nil, f.err("FindRoleByName")
}
func (f fakeRepo) GetRole(_ context.Context, id int64) (*db.Role, error) {
	return &db.Role{ID: id}, f.err("GetRole")
}
func (f fakeRepo) ListRolePermissionIDs(context.Context, int64) ([]int64, error) {
	return nil, f.err("ListRolePermissionIDs")
}
func (f fakeRepo) ReplaceRolePermissions(context.Context, pgx.Tx, int64, []int64, int64) error {
	return f.err("ReplaceRolePermissions")
}

// failingPool counts Begin calls and always fails them.
type failingPool struct{ begins int }

func (p *failingPool) Begin(context.Context) (pgx.Tx, error) {
	p.begins++
	return nil, errBoom
}

func TestPermissionService_DependencyErrorsArePropagated(t *testing.T) {
	ctx := context.Background()
	create := func(s *PermissionService) error {
		_, err := s.CreateRole(ctx, 1, "ADMIN", CreateRoleRequest{Role: "NEW-ROLE"}, "")
		return err
	}
	update := func(s *PermissionService) error {
		_, err := s.UpdateRolePermissions(ctx, 1, "ADMIN", 7, []int64{1}, "")
		return err
	}
	tests := []struct {
		name       string
		fail       string
		call       func(*PermissionService) error
		wantBegins int // 0 = failed before any transaction was opened
	}{
		{"create: name lookup fails", "FindRoleByName", create, 0},
		{"create: begin tx fails", "", create, 1},
		{"update: role lookup fails", "GetRole", update, 0},
		{"update: catalog check fails", "CatalogEntriesExist", update, 0},
		{"update: current permissions fail", "ListRolePermissionIDs", update, 0},
		{"update: begin tx fails", "", update, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &failingPool{}
			err := tt.call(NewPermissionService(fakeRepo{fail: tt.fail}, pool))
			if !errors.Is(err, errBoom) {
				t.Fatalf("error = %v, want wrapped errBoom", err)
			}
			if pool.begins != tt.wantBegins {
				t.Fatalf("Begin called %d times, want %d", pool.begins, tt.wantBegins)
			}
		})
	}
}

func TestPermissionService_UnauthorizedActorNeverTouchesRepoOrTx(t *testing.T) {
	ctx := context.Background()
	pool := &failingPool{}
	// Every repo method fails: reaching any of them would surface errBoom instead of ErrNotAuthorized.
	svc := NewPermissionService(failAll{}, pool)
	for _, role := range []string{"", "ATM-USER", "VENDOR-USER", "ADMINX"} {
		if _, err := svc.CreateRole(ctx, 1, role, CreateRoleRequest{Role: "X"}, ""); !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("CreateRole as %q: error = %v, want ErrNotAuthorized", role, err)
		}
		if _, err := svc.UpdateRolePermissions(ctx, 1, role, 7, nil, ""); !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("UpdateRolePermissions as %q: error = %v, want ErrNotAuthorized", role, err)
		}
	}
	if pool.begins != 0 {
		t.Fatalf("Begin called %d times for unauthorized actors, want 0", pool.begins)
	}
}

// failAll fails every Repo method.
type failAll struct{}

func (failAll) ListCatalog(context.Context) ([]db.MenuFeature, error) { return nil, errBoom }
func (failAll) CatalogEntriesExist(context.Context, []int64) (bool, error) {
	return false, errBoom
}
func (failAll) ListRolesWithPermissions(context.Context) ([]db.ListRolesWithPermissionsRow, error) {
	return nil, errBoom
}
func (failAll) FindRoleByName(context.Context, string) (*int64, error)        { return nil, errBoom }
func (failAll) GetRole(context.Context, int64) (*db.Role, error)              { return nil, errBoom }
func (failAll) ListRolePermissionIDs(context.Context, int64) ([]int64, error) { return nil, errBoom }
func (failAll) ReplaceRolePermissions(context.Context, pgx.Tx, int64, []int64, int64) error {
	return errBoom
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unique violation", &pgconn.PgError{Code: "23505"}, true},
		{"wrapped unique violation", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"}), true},
		{"other pg error", &pgconn.PgError{Code: "23503"}, false},
		{"plain error", errBoom, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUniqueViolation(tt.err); got != tt.want {
				t.Fatalf("isUniqueViolation(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
