package auth

import (
	"context"
	"errors"
	"testing"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

func TestSetInitialPasswordService_HappyPath(t *testing.T) {
	target := &pkgauth.UserRecord{ID: 7, Username: "new.user", AuthSource: "local"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewSetInitialPasswordService(repo, auditW)

	const appaccessActorID = 1
	err := svc.SetInitialPassword(context.Background(), appaccessActorID, target.ID, SetInitialPasswordRequest{
		NewPassword: "InitialPass1",
	}, "127.0.0.1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.setInitialPasswordCalled {
		t.Error("expected SetInitialPassword to be called")
	}
	if repo.setInitialPasswordUserID != target.ID {
		t.Errorf("expected SetInitialPassword called with userID=%d, got %d", target.ID, repo.setInitialPasswordUserID)
	}
	if len(auditW.entries) != 1 {
		t.Fatalf("expected exactly 1 audit entry, got %d", len(auditW.entries))
	}
	entry := auditW.entries[0]
	if entry.Action != "initial_password_set" {
		t.Errorf("expected action=initial_password_set, got %s", entry.Action)
	}
	if entry.ActorID != appaccessActorID {
		t.Errorf("expected ActorID=%d (APPACCESS actor, not target), got %d", appaccessActorID, entry.ActorID)
	}
	if entry.EntityType != "user" || entry.EntityID != target.ID {
		t.Errorf("expected entity=user/%d, got %s/%d", target.ID, entry.EntityType, entry.EntityID)
	}
	if entry.Before != nil || entry.After != nil {
		t.Errorf("expected no Before/After payload (no secrets), got Before=%v After=%v", entry.Before, entry.After)
	}
}

func TestSetInitialPasswordService_TargetLDAP_Rejected(t *testing.T) {
	target := &pkgauth.UserRecord{ID: 7, Username: "ldap.user", AuthSource: "ldap"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewSetInitialPasswordService(repo, auditW)

	err := svc.SetInitialPassword(context.Background(), 1, target.ID, SetInitialPasswordRequest{
		NewPassword: "InitialPass1",
	}, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrChangeNotAllowed) {
		t.Errorf("expected pkgauth.ErrChangeNotAllowed, got: %v", err)
	}
	if repo.setInitialPasswordCalled {
		t.Error("expected SetInitialPassword NOT to be called for ldap target")
	}
	if len(auditW.entries) != 0 {
		t.Error("expected no audit entry written on failure")
	}
}

func TestSetInitialPasswordService_TargetNotFound(t *testing.T) {
	repo := &stubUserRepo{findByIDResult: nil}
	auditW := &fakeAuditWriter{}

	svc := NewSetInitialPasswordService(repo, auditW)

	err := svc.SetInitialPassword(context.Background(), 1, 999, SetInitialPasswordRequest{
		NewPassword: "InitialPass1",
	}, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrUserNotFound) {
		t.Errorf("expected pkgauth.ErrUserNotFound, got: %v", err)
	}
}

func TestSetInitialPasswordService_WeakPassword_Rejected(t *testing.T) {
	target := &pkgauth.UserRecord{ID: 7, Username: "new.user", AuthSource: "local"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewSetInitialPasswordService(repo, auditW)

	err := svc.SetInitialPassword(context.Background(), 1, target.ID, SetInitialPasswordRequest{
		NewPassword: "short",
	}, "127.0.0.1")

	var validationErr *pkgauth.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *pkgauth.ValidationError, got: %T (%v)", err, err)
	}
	if repo.setInitialPasswordCalled {
		t.Error("expected SetInitialPassword NOT to be called")
	}
}

func TestSetInitialPasswordService_AuditWriteFails_PropagatesError(t *testing.T) {
	target := &pkgauth.UserRecord{ID: 7, Username: "new.user", AuthSource: "local"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{forceErr: errors.New("audit sink down")}

	svc := NewSetInitialPasswordService(repo, auditW)

	err := svc.SetInitialPassword(context.Background(), 1, target.ID, SetInitialPasswordRequest{
		NewPassword: "InitialPass1",
	}, "127.0.0.1")

	if err == nil {
		t.Fatal("expected an error when the audit write fails")
	}
}

func TestSetInitialPasswordService_NoOldPasswordRequired(t *testing.T) {
	// Unlike ChangePasswordService, SetInitialPassword must succeed with no
	// old-password check at all — the request type has no such field.
	target := &pkgauth.UserRecord{ID: 7, Username: "new.user", AuthSource: "local_dev", PasswordHash: nil}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewSetInitialPasswordService(repo, auditW)

	err := svc.SetInitialPassword(context.Background(), 1, target.ID, SetInitialPasswordRequest{
		NewPassword: "InitialPass1",
	}, "127.0.0.1")

	if err != nil {
		t.Fatalf("expected no error even with nil PasswordHash on target, got: %v", err)
	}
}
