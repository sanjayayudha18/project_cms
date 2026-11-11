package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

// ext-user-api task 3.3 (Req 9.5, 9.6): the service role cannot be granted
// and a service account is not found for AdminUserAPI.

func TestUserAdminService_ServiceRoleRejected(t *testing.T) {
	assertServiceRoleErr := func(t *testing.T, repo *fakeUserAdminRepo, err error) {
		t.Helper()
		var refErr *ReferenceError
		if !errors.As(err, &refErr) || refErr.Field != "role" || refErr.Message != "tidak dapat diberikan" {
			t.Fatalf("err = %v, want ReferenceError{role, tidak dapat diberikan}", err)
		}
		if repo.createCalled || repo.updateCalled {
			t.Error("repo write called despite the service role")
		}
	}

	t.Run("create", func(t *testing.T) {
		repo := &fakeUserAdminRepo{}
		req := validCreateReq()
		req.Role = userrules.ServiceRole
		_, err := NewUserAdminService(repo, nil, &fakeAuditWriter{}).Create(context.Background(), 1, "APPACCESS", req, "10.0.0.1")
		assertServiceRoleErr(t, repo, err)
	})

	t.Run("update", func(t *testing.T) {
		repo := &fakeUserAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
			return activeUserRow(id, "u", "ldap", "u@example.com"), nil
		}}
		req := UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: userrules.ServiceRole}
		_, err := NewUserAdminService(repo, nil, &fakeAuditWriter{}).Update(context.Background(), 1, "APPACCESS", 7, req, "10.0.0.1")
		assertServiceRoleErr(t, repo, err)
	})
}

func TestUserAdminService_Update_ServiceAccountNotFound(t *testing.T) {
	repo := &fakeUserAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
		return activeUserRow(id, "svc-idm", userrules.AuthSourceService, "svc@example.com"), nil
	}}
	req := UpdateUserRequest{FullName: "X", Email: "svc@example.com", Role: "APPACCESS"}
	_, err := NewUserAdminService(repo, nil, &fakeAuditWriter{}).Update(context.Background(), 1, "APPACCESS", 7, req, "10.0.0.1")
	if !errors.Is(err, pkgauth.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
	if repo.updateCalled {
		t.Error("repo.Update called for a service account")
	}
}
