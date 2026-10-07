package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// fakeAuditWriter records every entry written (mirrors internal/approval's
// test double), for asserting audit_logs is written without secrets.
type fakeAuditWriter struct {
	entries  []audit.Entry
	forceErr error
}

func (f *fakeAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	if f.forceErr != nil {
		return f.forceErr
	}
	f.entries = append(f.entries, entry)
	return nil
}

func localUserWithPassword(t *testing.T, authSource, password string) *pkgauth.UserRecord {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	hashStr := string(hash)
	return &pkgauth.UserRecord{
		ID:           1,
		Username:     "john.admin",
		AuthSource:   authSource,
		PasswordHash: &hashStr,
	}
}

func TestChangePasswordService_HappyPath(t *testing.T) {
	user := localUserWithPassword(t, "local", "OldPassword1")
	repo := &stubUserRepo{findByIDResult: user}
	auditW := &fakeAuditWriter{}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), user.ID, ChangePasswordRequest{
		OldPassword: "OldPassword1",
		NewPassword: "NewPassword2",
	}, "127.0.0.1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.setPasswordCalled {
		t.Error("expected SetPassword to be called")
	}
	if len(auditW.entries) != 1 {
		t.Fatalf("expected exactly 1 audit entry, got %d", len(auditW.entries))
	}
	entry := auditW.entries[0]
	if entry.Action != "password_change" {
		t.Errorf("expected action=password_change, got %s", entry.Action)
	}
	if entry.ActorID != user.ID {
		t.Errorf("expected ActorID=%d, got %d", user.ID, entry.ActorID)
	}
	if entry.EntityType != "user" || entry.EntityID != user.ID {
		t.Errorf("expected entity=user/%d, got %s/%d", user.ID, entry.EntityType, entry.EntityID)
	}
	// Never leak the password/hash into the audit trail.
	if entry.Before != nil || entry.After != nil {
		t.Errorf("expected no Before/After payload (no secrets), got Before=%v After=%v", entry.Before, entry.After)
	}
}

func TestChangePasswordService_WrongOldPassword_Rejected(t *testing.T) {
	user := localUserWithPassword(t, "local", "OldPassword1")
	repo := &stubUserRepo{findByIDResult: user}
	auditW := &fakeAuditWriter{}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), user.ID, ChangePasswordRequest{
		OldPassword: "WrongPassword",
		NewPassword: "NewPassword2",
	}, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials, got: %v", err)
	}
	if repo.setPasswordCalled {
		t.Error("expected SetPassword NOT to be called")
	}
	if len(auditW.entries) != 0 {
		t.Error("expected no audit entry written on failure")
	}
}

func TestChangePasswordService_LDAPAccount_Rejected(t *testing.T) {
	user := localUserWithPassword(t, "ldap", "OldPassword1")
	repo := &stubUserRepo{findByIDResult: user}
	auditW := &fakeAuditWriter{}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), user.ID, ChangePasswordRequest{
		OldPassword: "OldPassword1",
		NewPassword: "NewPassword2",
	}, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrChangeNotAllowed) {
		t.Errorf("expected pkgauth.ErrChangeNotAllowed, got: %v", err)
	}
	if repo.setPasswordCalled {
		t.Error("expected SetPassword NOT to be called for ldap account")
	}
}

func TestChangePasswordService_NewEqualsOld_Rejected(t *testing.T) {
	user := localUserWithPassword(t, "local_dev", "SamePassword1")
	repo := &stubUserRepo{findByIDResult: user}
	auditW := &fakeAuditWriter{}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), user.ID, ChangePasswordRequest{
		OldPassword: "SamePassword1",
		NewPassword: "SamePassword1",
	}, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrPasswordUnchanged) {
		t.Errorf("expected pkgauth.ErrPasswordUnchanged, got: %v", err)
	}
	if repo.setPasswordCalled {
		t.Error("expected SetPassword NOT to be called")
	}
}

func TestChangePasswordService_WeakNewPassword_Rejected(t *testing.T) {
	user := localUserWithPassword(t, "local", "OldPassword1")
	repo := &stubUserRepo{findByIDResult: user}
	auditW := &fakeAuditWriter{}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), user.ID, ChangePasswordRequest{
		OldPassword: "OldPassword1",
		NewPassword: "short",
	}, "127.0.0.1")

	var validationErr *pkgauth.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *pkgauth.ValidationError, got: %T (%v)", err, err)
	}
	if repo.setPasswordCalled {
		t.Error("expected SetPassword NOT to be called")
	}
}

func TestChangePasswordService_AuditWriteFails_PropagatesError(t *testing.T) {
	user := localUserWithPassword(t, "local", "OldPassword1")
	repo := &stubUserRepo{findByIDResult: user}
	auditW := &fakeAuditWriter{forceErr: errors.New("audit sink down")}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), user.ID, ChangePasswordRequest{
		OldPassword: "OldPassword1",
		NewPassword: "NewPassword2",
	}, "127.0.0.1")

	if err == nil {
		t.Fatal("expected an error when the audit write fails")
	}
	if !repo.setPasswordCalled {
		t.Error("expected SetPassword to still have been called before the audit write failed")
	}
}

func TestChangePasswordService_UserNotFound_GenericError(t *testing.T) {
	repo := &stubUserRepo{findByIDResult: nil}
	auditW := &fakeAuditWriter{}

	svc := NewChangePasswordService(repo, auditW)

	err := svc.ChangePassword(context.Background(), 999, ChangePasswordRequest{
		OldPassword: "OldPassword1",
		NewPassword: "NewPassword2",
	}, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials, got: %v", err)
	}
}
