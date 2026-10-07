package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// --- fake repo ----------------------------------------------------------

type fakeUserAdminRepo struct {
	listFunc                        func(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error)
	countFunc                       func(ctx context.Context, arg db.CountUsersAdminParams) (int64, error)
	getByIDFunc                     func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error)
	createFunc                      func(ctx context.Context, arg db.CreateUserAdminParams) (db.CreateUserAdminRow, error)
	updateFunc                      func(ctx context.Context, arg db.UpdateUserAdminParams) (db.UpdateUserAdminRow, error)
	findByUsernameFunc              func(ctx context.Context, username string) (*int64, error)
	findByEmailFunc                 func(ctx context.Context, email string) (*int64, error)
	findByEmailExcludingIDFunc      func(ctx context.Context, email string, excludeID int64) (*int64, error)
	findByEmployeeIDFunc            func(ctx context.Context, employeeID string) (*int64, error)
	findByEmployeeIDExcludingIDFunc func(ctx context.Context, employeeID string, excludeID int64) (*int64, error)
	getRoleByNameFunc               func(ctx context.Context, role string) (*db.GetRoleByNameRow, error)
	vendorExistsFunc                func(ctx context.Context, vendorID int64) (bool, error)
	supervisorExistsFunc            func(ctx context.Context, userID int64) (bool, error)

	createCalled bool
	updateCalled bool
	createArg    db.CreateUserAdminParams
}

func (f *fakeUserAdminRepo) List(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx, arg)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) Count(ctx context.Context, arg db.CountUsersAdminParams) (int64, error) {
	if f.countFunc != nil {
		return f.countFunc(ctx, arg)
	}
	return 0, nil
}

func (f *fakeUserAdminRepo) GetByID(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
	if f.getByIDFunc != nil {
		return f.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) Create(ctx context.Context, arg db.CreateUserAdminParams) (db.CreateUserAdminRow, error) {
	f.createCalled = true
	f.createArg = arg
	if f.createFunc != nil {
		return f.createFunc(ctx, arg)
	}
	return db.CreateUserAdminRow{
		ID: 1, Username: arg.Username, FullName: arg.FullName, Email: arg.Email,
		RoleID: arg.RoleID, IsKaryawan: arg.IsKaryawan, AuthSource: arg.AuthSource,
		EmployeeID: arg.EmployeeID, VendorID: arg.VendorID, IsActive: true,
		SupervisorID: arg.SupervisorID, ApprovalLevel: arg.ApprovalLevel,
	}, nil
}

func (f *fakeUserAdminRepo) Update(ctx context.Context, arg db.UpdateUserAdminParams) (db.UpdateUserAdminRow, error) {
	f.updateCalled = true
	if f.updateFunc != nil {
		return f.updateFunc(ctx, arg)
	}
	return db.UpdateUserAdminRow{
		ID: arg.ID, FullName: arg.FullName, Email: arg.Email, RoleID: arg.RoleID,
		IsKaryawan: arg.IsKaryawan, EmployeeID: arg.EmployeeID, VendorID: arg.VendorID,
		IsActive: true, SupervisorID: arg.SupervisorID, ApprovalLevel: arg.ApprovalLevel,
	}, nil
}

func (f *fakeUserAdminRepo) FindByUsername(ctx context.Context, username string) (*int64, error) {
	if f.findByUsernameFunc != nil {
		return f.findByUsernameFunc(ctx, username)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) FindByEmail(ctx context.Context, email string) (*int64, error) {
	if f.findByEmailFunc != nil {
		return f.findByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) FindByEmailExcludingID(ctx context.Context, email string, excludeID int64) (*int64, error) {
	if f.findByEmailExcludingIDFunc != nil {
		return f.findByEmailExcludingIDFunc(ctx, email, excludeID)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) FindByEmployeeID(ctx context.Context, employeeID string) (*int64, error) {
	if f.findByEmployeeIDFunc != nil {
		return f.findByEmployeeIDFunc(ctx, employeeID)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) FindByEmployeeIDExcludingID(ctx context.Context, employeeID string, excludeID int64) (*int64, error) {
	if f.findByEmployeeIDExcludingIDFunc != nil {
		return f.findByEmployeeIDExcludingIDFunc(ctx, employeeID, excludeID)
	}
	return nil, nil
}

func (f *fakeUserAdminRepo) GetRoleByName(ctx context.Context, role string) (*db.GetRoleByNameRow, error) {
	if f.getRoleByNameFunc != nil {
		return f.getRoleByNameFunc(ctx, role)
	}
	return &db.GetRoleByNameRow{ID: 1, Role: role}, nil
}

func (f *fakeUserAdminRepo) VendorExists(ctx context.Context, vendorID int64) (bool, error) {
	if f.vendorExistsFunc != nil {
		return f.vendorExistsFunc(ctx, vendorID)
	}
	return true, nil
}

func (f *fakeUserAdminRepo) SupervisorExists(ctx context.Context, userID int64) (bool, error) {
	if f.supervisorExistsFunc != nil {
		return f.supervisorExistsFunc(ctx, userID)
	}
	return true, nil
}

func activeUserRow(id int64, username, authSource, email string) *db.GetUserAdminByIDRow {
	return &db.GetUserAdminByIDRow{ID: id, Username: username, AuthSource: authSource, Email: email, IsActive: true}
}

func validCreateReq() CreateUserRequest {
	return CreateUserRequest{
		Username: "new.user", FullName: "New User", Email: "new.user@example.com",
		Role: "APPACCESS", AuthSource: "ldap",
	}
}

func ptrInt64(v int64) *int64 { return &v }

// --- Create: validation ---------------------------------------------------

func TestUserAdminService_Create_Validation(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(r *CreateUserRequest)
		wantField string
	}{
		{"missing username", func(r *CreateUserRequest) { r.Username = "" }, "username"},
		{"missing full_name", func(r *CreateUserRequest) { r.FullName = "" }, "full_name"},
		{"missing email", func(r *CreateUserRequest) { r.Email = "" }, "email"},
		{"invalid email", func(r *CreateUserRequest) { r.Email = "not-an-email" }, "email"},
		{"missing role", func(r *CreateUserRequest) { r.Role = "" }, "role"},
		{"invalid auth_source", func(r *CreateUserRequest) { r.AuthSource = "sso" }, "auth_source"},
		{"local without vendor_id", func(r *CreateUserRequest) {
			r.AuthSource = "local"
			r.TemporaryPassword = "Passw0rd1"
		}, "vendor_id"},
		{"local without temporary_password", func(r *CreateUserRequest) {
			r.AuthSource = "local"
			r.VendorID = ptrInt64(1)
		}, "temporary_password"},
		{"local with weak temporary_password remaps to temporary_password field", func(r *CreateUserRequest) {
			r.AuthSource = "local"
			r.VendorID = ptrInt64(1)
			r.TemporaryPassword = "short"
		}, "temporary_password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateReq()
			tt.mutate(&req)
			repo := &fakeUserAdminRepo{}
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(repo, auditW)

			_, err := svc.Create(context.Background(), 1, req, "10.0.0.1")
			var valErr *pkgauth.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("Create err = %v, want *pkgauth.ValidationError", err)
			}
			if valErr.Field != tt.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, tt.wantField)
			}
			if repo.createCalled {
				t.Error("repo.Create was called despite validation failure")
			}
			if len(auditW.entries) != 0 {
				t.Error("audit.Write was called despite validation failure")
			}
		})
	}
}

func TestUserAdminService_Create_LdapForbiddenFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(r *CreateUserRequest)
	}{
		{"vendor_id present for ldap", func(r *CreateUserRequest) { r.VendorID = ptrInt64(1) }},
		{"temporary_password present for ldap", func(r *CreateUserRequest) { r.TemporaryPassword = "Passw0rd1" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateReq()
			tt.mutate(&req)
			repo := &fakeUserAdminRepo{}
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(repo, auditW)

			_, err := svc.Create(context.Background(), 1, req, "10.0.0.1")
			if !errors.Is(err, ErrLdapFieldsNotAllowed) {
				t.Fatalf("err = %v, want ErrLdapFieldsNotAllowed", err)
			}
			if repo.createCalled {
				t.Error("repo.Create was called despite forbidden ldap fields")
			}
		})
	}
}

// --- Create: local password handling ---------------------------------

func TestUserAdminService_Create_Local_HashesPasswordAndForcesChange(t *testing.T) {
	repo := &fakeUserAdminRepo{}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	req := validCreateReq()
	req.AuthSource = "local"
	req.VendorID = ptrInt64(1)
	req.TemporaryPassword = "Passw0rd1"

	_, err := svc.Create(context.Background(), 1, req, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !repo.createCalled {
		t.Fatal("repo.Create was not called")
	}
	if repo.createArg.PasswordHash == nil {
		t.Fatal("CreateUserAdminParams.PasswordHash is nil, want a bcrypt hash")
	}
	if *repo.createArg.PasswordHash == req.TemporaryPassword {
		t.Error("PasswordHash equals the plaintext temporary password -- it must be hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*repo.createArg.PasswordHash), []byte(req.TemporaryPassword)); err != nil {
		t.Errorf("stored hash does not verify against the temporary password: %v", err)
	}
	if !repo.createArg.MustChangePassword {
		t.Error("MustChangePassword = false, want true for a local create (Resolved Decision 3)")
	}
}

func TestUserAdminService_Create_Ldap_NoPasswordHash(t *testing.T) {
	repo := &fakeUserAdminRepo{}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Create(context.Background(), 1, validCreateReq(), "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.createArg.PasswordHash != nil {
		t.Error("PasswordHash is set for an ldap user, want nil")
	}
	if repo.createArg.MustChangePassword {
		t.Error("MustChangePassword = true for an ldap user, want false")
	}
}

// --- Create: conflicts ---------------------------------------------------

func TestUserAdminService_Create_Conflicts(t *testing.T) {
	existingID := int64(42)
	tests := []struct {
		name      string
		repo      *fakeUserAdminRepo
		req       func() CreateUserRequest
		wantField string
	}{
		{
			"username conflict", &fakeUserAdminRepo{
				findByUsernameFunc: func(ctx context.Context, username string) (*int64, error) { return &existingID, nil },
			}, validCreateReq, "username",
		},
		{
			"email conflict", &fakeUserAdminRepo{
				findByEmailFunc: func(ctx context.Context, email string) (*int64, error) { return &existingID, nil },
			}, validCreateReq, "email",
		},
		{
			"employee_id conflict", &fakeUserAdminRepo{
				findByEmployeeIDFunc: func(ctx context.Context, employeeID string) (*int64, error) { return &existingID, nil },
			}, func() CreateUserRequest { r := validCreateReq(); r.EmployeeID = "EMP1"; return r },
			"employee_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(tt.repo, auditW)

			_, err := svc.Create(context.Background(), 1, tt.req(), "10.0.0.1")
			var conflictErr *ConflictError
			if !errors.As(err, &conflictErr) {
				t.Fatalf("err = %v, want *ConflictError", err)
			}
			if conflictErr.Field != tt.wantField {
				t.Errorf("ConflictError.Field = %q, want %q", conflictErr.Field, tt.wantField)
			}
			if tt.repo.createCalled {
				t.Error("repo.Create was called despite a pre-check conflict")
			}
			if len(auditW.entries) != 0 {
				t.Error("audit.Write was called despite a conflict")
			}
		})
	}
}

func TestUserAdminService_Create_DBUniqueViolationFallback(t *testing.T) {
	repo := &fakeUserAdminRepo{
		createFunc: func(ctx context.Context, arg db.CreateUserAdminParams) (db.CreateUserAdminRow, error) {
			return db.CreateUserAdminRow{}, &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: "users_email_key"}
		},
	}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Create(context.Background(), 1, validCreateReq(), "10.0.0.1")
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) || conflictErr.Field != "email" {
		t.Fatalf("err = %v, want *ConflictError{Field: email}", err)
	}
}

// --- Create: references ---------------------------------------------------

func TestUserAdminService_Create_ReferenceErrors(t *testing.T) {
	tests := []struct {
		name      string
		repo      *fakeUserAdminRepo
		req       func() CreateUserRequest
		wantField string
	}{
		{
			"unknown role", &fakeUserAdminRepo{
				getRoleByNameFunc: func(ctx context.Context, role string) (*db.GetRoleByNameRow, error) { return nil, nil },
			}, validCreateReq, "role",
		},
		{
			"unknown vendor_id", &fakeUserAdminRepo{
				vendorExistsFunc: func(ctx context.Context, vendorID int64) (bool, error) { return false, nil },
			}, func() CreateUserRequest {
				r := validCreateReq()
				r.AuthSource = "local"
				r.VendorID = ptrInt64(99)
				r.TemporaryPassword = "Passw0rd1"
				return r
			}, "vendor_id",
		},
		{
			"unknown supervisor_id", &fakeUserAdminRepo{
				supervisorExistsFunc: func(ctx context.Context, userID int64) (bool, error) { return false, nil },
			}, func() CreateUserRequest { r := validCreateReq(); r.SupervisorID = ptrInt64(99); return r }, "supervisor_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(tt.repo, auditW)

			_, err := svc.Create(context.Background(), 1, tt.req(), "10.0.0.1")
			var refErr *ReferenceError
			if !errors.As(err, &refErr) {
				t.Fatalf("err = %v, want *ReferenceError", err)
			}
			if refErr.Field != tt.wantField {
				t.Errorf("ReferenceError.Field = %q, want %q", refErr.Field, tt.wantField)
			}
		})
	}
}

// --- Create: audit --------------------------------------------------------

func TestUserAdminService_Create_Success_AuditsOnce_NoPasswordInPayload(t *testing.T) {
	repo := &fakeUserAdminRepo{}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	req := validCreateReq()
	req.AuthSource = "local"
	req.VendorID = ptrInt64(1)
	req.TemporaryPassword = "Passw0rd1"

	got, err := svc.Create(context.Background(), 7, req, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Username != req.Username {
		t.Errorf("got.Username = %q, want %q", got.Username, req.Username)
	}
	if len(auditW.entries) != 1 {
		t.Fatalf("audit entries = %d, want exactly 1", len(auditW.entries))
	}
	entry := auditW.entries[0]
	if entry.Action != "user_created" || entry.EntityType != "user" || entry.ActorID != 7 || entry.Before != nil {
		t.Errorf("audit entry = %+v, want action=user_created entity_type=user actor_id=7 before=nil", entry)
	}
	// Assert the marshalled audit payload never contains the plaintext
	// temporary password or a "password_hash" key (Req 9.3).
	payload, err := json.Marshal(entry.After)
	if err != nil {
		t.Fatalf("marshal audit After: %v", err)
	}
	if strings.Contains(string(payload), req.TemporaryPassword) {
		t.Error("audit payload contains the plaintext temporary password")
	}
	if strings.Contains(string(payload), "password_hash") || strings.Contains(string(payload), "password") {
		t.Errorf("audit payload contains a password-related key: %s", payload)
	}
}

func TestUserAdminService_Create_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeUserAdminRepo{}
	auditW := &fakeAuditWriter{forceErr: errors.New("audit db down")}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Create(context.Background(), 1, validCreateReq(), "10.0.0.1")
	if err == nil {
		t.Fatal("Create: want an error when the audit write fails, got nil")
	}
	if !repo.createCalled {
		t.Error("repo.Create was not called -- the user row itself should still have been inserted")
	}
}

// --- Update -----------------------------------------------------------

func TestUserAdminService_Update_NotFound(t *testing.T) {
	tests := []struct {
		name    string
		getByID func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error)
	}{
		{"missing id", func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) { return nil, nil }},
		{"soft-disabled target", func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
			row := activeUserRow(id, "u", "ldap", "u@example.com")
			row.DeletedAt.Valid = true
			return row, nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserAdminRepo{getByIDFunc: tt.getByID}
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(repo, auditW)

			_, err := svc.Update(context.Background(), 1, 99, UpdateUserRequest{FullName: "X", Email: "x@example.com", Role: "APPACCESS"}, "10.0.0.1")
			if !errors.Is(err, pkgauth.ErrUserNotFound) {
				t.Fatalf("err = %v, want pkgauth.ErrUserNotFound", err)
			}
			if repo.updateCalled {
				t.Error("repo.Update was called despite a not-found target")
			}
		})
	}
}

func TestUserAdminService_Update_ImmutableFields(t *testing.T) {
	otherUsername := "different.username"
	otherAuthSource := "local"
	tests := []struct {
		name string
		req  UpdateUserRequest
	}{
		{"username change attempt", UpdateUserRequest{Username: &otherUsername, FullName: "X", Email: "x@example.com", Role: "APPACCESS"}},
		{"auth_source change attempt", UpdateUserRequest{AuthSource: &otherAuthSource, FullName: "X", Email: "x@example.com", Role: "APPACCESS"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserAdminRepo{
				getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
					return activeUserRow(id, "original.username", "ldap", "orig@example.com"), nil
				},
			}
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(repo, auditW)

			_, err := svc.Update(context.Background(), 1, 1, tt.req, "10.0.0.1")
			if !errors.Is(err, ErrImmutableField) {
				t.Fatalf("err = %v, want ErrImmutableField", err)
			}
			if repo.updateCalled {
				t.Error("repo.Update was called despite an immutable-field change attempt")
			}
		})
	}
}

func TestUserAdminService_Update_SelfSupervision(t *testing.T) {
	repo := &fakeUserAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
			return activeUserRow(id, "u", "ldap", "u@example.com"), nil
		},
	}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{
		FullName: "X", Email: "x@example.com", Role: "APPACCESS", SupervisorID: ptrInt64(1),
	}, "10.0.0.1")
	if !errors.Is(err, ErrSelfSupervision) {
		t.Fatalf("err = %v, want ErrSelfSupervision", err)
	}
	if repo.updateCalled {
		t.Error("repo.Update was called despite self-supervision")
	}
}

func TestUserAdminService_Update_Validation(t *testing.T) {
	repo := &fakeUserAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
			return activeUserRow(id, "u", "ldap", "u@example.com"), nil
		},
	}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	tests := []struct {
		name      string
		req       UpdateUserRequest
		wantField string
	}{
		{"missing full_name", UpdateUserRequest{Email: "x@example.com", Role: "APPACCESS"}, "full_name"},
		{"missing email", UpdateUserRequest{FullName: "X", Role: "APPACCESS"}, "email"},
		{"invalid email", UpdateUserRequest{FullName: "X", Email: "bad", Role: "APPACCESS"}, "email"},
		{"missing role", UpdateUserRequest{FullName: "X", Email: "x@example.com"}, "role"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(context.Background(), 1, 1, tt.req, "10.0.0.1")
			var valErr *pkgauth.ValidationError
			if !errors.As(err, &valErr) || valErr.Field != tt.wantField {
				t.Fatalf("err = %v, want *pkgauth.ValidationError{Field: %s}", err, tt.wantField)
			}
		})
	}
}

func TestUserAdminService_Update_Conflicts(t *testing.T) {
	existingID := int64(42)
	base := func() *fakeUserAdminRepo {
		return &fakeUserAdminRepo{
			getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
				return activeUserRow(id, "u", "ldap", "orig@example.com"), nil
			},
		}
	}

	t.Run("email conflict excluding self", func(t *testing.T) {
		repo := base()
		repo.findByEmailExcludingIDFunc = func(ctx context.Context, email string, excludeID int64) (*int64, error) { return &existingID, nil }
		auditW := &fakeAuditWriter{}
		svc := NewUserAdminService(repo, auditW)

		_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "new@example.com", Role: "APPACCESS"}, "10.0.0.1")
		var conflictErr *ConflictError
		if !errors.As(err, &conflictErr) || conflictErr.Field != "email" {
			t.Fatalf("err = %v, want *ConflictError{Field: email}", err)
		}
	})

	t.Run("employee_id conflict excluding self", func(t *testing.T) {
		repo := base()
		repo.findByEmployeeIDExcludingIDFunc = func(ctx context.Context, employeeID string, excludeID int64) (*int64, error) { return &existingID, nil }
		auditW := &fakeAuditWriter{}
		svc := NewUserAdminService(repo, auditW)

		_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "orig@example.com", Role: "APPACCESS", EmployeeID: "EMP99"}, "10.0.0.1")
		var conflictErr *ConflictError
		if !errors.As(err, &conflictErr) || conflictErr.Field != "employee_id" {
			t.Fatalf("err = %v, want *ConflictError{Field: employee_id}", err)
		}
	})
}

func TestUserAdminService_Update_RepoNoRowsMapsToNotFound(t *testing.T) {
	repo := &fakeUserAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
			return activeUserRow(id, "u", "ldap", "u@example.com"), nil
		},
		updateFunc: func(ctx context.Context, arg db.UpdateUserAdminParams) (db.UpdateUserAdminRow, error) {
			return db.UpdateUserAdminRow{}, pgx.ErrNoRows
		},
	}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: "APPACCESS"}, "10.0.0.1")
	if !errors.Is(err, pkgauth.ErrUserNotFound) {
		t.Fatalf("err = %v, want pkgauth.ErrUserNotFound", err)
	}
	if len(auditW.entries) != 0 {
		t.Error("audit.Write was called despite the update affecting 0 rows")
	}
}

func TestUserAdminService_Update_Success_AuditsOnceWithBeforeAfter(t *testing.T) {
	before := activeUserRow(1, "u", "ldap", "orig@example.com")
	repo := &fakeUserAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) { return before, nil },
	}
	auditW := &fakeAuditWriter{}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 7, 1, UpdateUserRequest{FullName: "Renamed", Email: "orig@example.com", Role: "APPACCESS"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(auditW.entries) != 1 {
		t.Fatalf("audit entries = %d, want exactly 1", len(auditW.entries))
	}
	entry := auditW.entries[0]
	if entry.Action != "user_updated" || entry.Before == nil || entry.After == nil {
		t.Errorf("audit entry = %+v, want action=user_updated with before and after set", entry)
	}
}

func TestUserAdminService_Update_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeUserAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
			return activeUserRow(id, "u", "ldap", "u@example.com"), nil
		},
	}
	auditW := &fakeAuditWriter{forceErr: errors.New("audit db down")}
	svc := NewUserAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: "APPACCESS"}, "10.0.0.1")
	if err == nil {
		t.Fatal("Update: want an error when the audit write fails, got nil")
	}
	if !repo.updateCalled {
		t.Error("repo.Update was not called -- the row itself should still have been updated")
	}
}

func TestConflictError_And_ReferenceError_ErrorStrings(t *testing.T) {
	ce := &ConflictError{Field: "email", Message: "sudah digunakan"}
	if got, want := ce.Error(), "email: sudah digunakan"; got != want {
		t.Errorf("ConflictError.Error() = %q, want %q", got, want)
	}
	re := &ReferenceError{Field: "vendor_id", Message: "tidak ditemukan"}
	if got, want := re.Error(), "vendor_id: tidak ditemukan"; got != want {
		t.Errorf("ReferenceError.Error() = %q, want %q", got, want)
	}
}

func TestConflictFieldFromPgError_UnknownConstraint(t *testing.T) {
	field, ok := conflictFieldFromPgError(&pgconn.PgError{Code: pgUniqueViolation, ConstraintName: "some_other_key"})
	if !ok || field != "unknown" {
		t.Errorf("conflictFieldFromPgError(unknown constraint) = (%q, %v), want (unknown, true)", field, ok)
	}
	if _, ok := conflictFieldFromPgError(errors.New("not a pg error")); ok {
		t.Error("conflictFieldFromPgError(non-pg error) = ok=true, want false")
	}
}

func TestUserAdminService_Update_ReferenceErrors(t *testing.T) {
	base := func() *fakeUserAdminRepo {
		return &fakeUserAdminRepo{
			getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
				return activeUserRow(id, "u", "ldap", "u@example.com"), nil
			},
		}
	}

	t.Run("unknown role", func(t *testing.T) {
		repo := base()
		repo.getRoleByNameFunc = func(ctx context.Context, role string) (*db.GetRoleByNameRow, error) { return nil, nil }
		svc := NewUserAdminService(repo, &fakeAuditWriter{})

		_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: "MISSING"}, "10.0.0.1")
		var refErr *ReferenceError
		if !errors.As(err, &refErr) || refErr.Field != "role" {
			t.Fatalf("err = %v, want *ReferenceError{Field: role}", err)
		}
	})

	t.Run("unknown vendor_id", func(t *testing.T) {
		repo := base()
		repo.vendorExistsFunc = func(ctx context.Context, vendorID int64) (bool, error) { return false, nil }
		svc := NewUserAdminService(repo, &fakeAuditWriter{})

		_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: "APPACCESS", VendorID: ptrInt64(99)}, "10.0.0.1")
		var refErr *ReferenceError
		if !errors.As(err, &refErr) || refErr.Field != "vendor_id" {
			t.Fatalf("err = %v, want *ReferenceError{Field: vendor_id}", err)
		}
	})

	t.Run("unknown supervisor_id", func(t *testing.T) {
		repo := base()
		repo.supervisorExistsFunc = func(ctx context.Context, userID int64) (bool, error) { return false, nil }
		svc := NewUserAdminService(repo, &fakeAuditWriter{})

		_, err := svc.Update(context.Background(), 1, 1, UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: "APPACCESS", SupervisorID: ptrInt64(99)}, "10.0.0.1")
		var refErr *ReferenceError
		if !errors.As(err, &refErr) || refErr.Field != "supervisor_id" {
			t.Fatalf("err = %v, want *ReferenceError{Field: supervisor_id}", err)
		}
	})
}

// --- List / Count / Get pass-throughs (Req 2, 9.5 -- read-only, no audit) -

func TestUserAdminService_List_PassesThroughToRepo(t *testing.T) {
	want := []db.ListUsersAdminRow{{ID: 1, Username: "u1"}}
	repo := &fakeUserAdminRepo{listFunc: func(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error) {
		if arg.Status != "active" {
			t.Errorf("Status = %q, want active", arg.Status)
		}
		return want, nil
	}}
	svc := NewUserAdminService(repo, &fakeAuditWriter{})

	got, err := svc.List(context.Background(), db.ListUsersAdminParams{Status: "active"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("List = %+v, want %+v", got, want)
	}
}

func TestUserAdminService_Count_PassesThroughToRepo(t *testing.T) {
	repo := &fakeUserAdminRepo{countFunc: func(ctx context.Context, arg db.CountUsersAdminParams) (int64, error) { return 42, nil }}
	svc := NewUserAdminService(repo, &fakeAuditWriter{})

	got, err := svc.Count(context.Background(), db.CountUsersAdminParams{Status: "all"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 42 {
		t.Errorf("Count = %d, want 42", got)
	}
}

func TestUserAdminService_Get_PassesThroughToRepo(t *testing.T) {
	repo := &fakeUserAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
		return activeUserRow(id, "u", "ldap", "u@example.com"), nil
	}}
	svc := NewUserAdminService(repo, &fakeAuditWriter{})

	got, err := svc.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.ID != 7 {
		t.Errorf("Get = %+v, want id=7", got)
	}

	repo.getByIDFunc = func(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) { return nil, nil }
	got, err = svc.Get(context.Background(), 999)
	if err != nil {
		t.Fatalf("Get(missing): %v", err)
	}
	if got != nil {
		t.Errorf("Get(missing) = %+v, want nil", got)
	}
}
