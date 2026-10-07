package auth

import (
	"context"
	"errors"
	"testing"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

func TestDeactivateUserService_Deactivate_UserWithAuditHistory_IsSoftDeleted(t *testing.T) {
	// "User ber-audit" — in practice this means audit_logs rows already
	// reference this user's id as actor_id. The service never queries that
	// (see deactivate_user.go doc comment: one code path for everyone), so
	// this test just documents/locks the spec's first case: soft-delete,
	// never a hard DELETE.
	target := &pkgauth.UserRecord{ID: 7, Username: "audited.user"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewDeactivateUserService(repo, auditW)

	err := svc.Deactivate(context.Background(), 1, target.ID, "127.0.0.1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.deactivateCalled {
		t.Error("expected Deactivate to be called (soft-delete), never a hard delete")
	}
	if repo.deactivateUserID != target.ID {
		t.Errorf("expected Deactivate called with userID=%d, got %d", target.ID, repo.deactivateUserID)
	}
}

func TestDeactivateUserService_Deactivate_UserWithoutAuditHistory_StillSoftDeleted(t *testing.T) {
	// Spec's second case: a user with NO audit history still goes through
	// the exact same soft-delete path — consistency, not a hard-delete
	// shortcut for "clean" accounts.
	target := &pkgauth.UserRecord{ID: 8, Username: "no.audit.user"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewDeactivateUserService(repo, auditW)

	err := svc.Deactivate(context.Background(), 1, target.ID, "127.0.0.1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.deactivateCalled {
		t.Error("expected Deactivate to be called (soft-delete)")
	}
}

func TestDeactivateUserService_Deactivate_WritesAuditWithoutSecrets(t *testing.T) {
	target := &pkgauth.UserRecord{ID: 7, Username: "audited.user"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{}

	svc := NewDeactivateUserService(repo, auditW)

	const actorID = 99
	if err := svc.Deactivate(context.Background(), actorID, target.ID, "127.0.0.1"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(auditW.entries) != 1 {
		t.Fatalf("expected exactly 1 audit entry, got %d", len(auditW.entries))
	}
	entry := auditW.entries[0]
	if entry.Action != "user_deactivated" {
		t.Errorf("expected action=user_deactivated, got %s", entry.Action)
	}
	if entry.ActorID != actorID {
		t.Errorf("expected ActorID=%d, got %d", actorID, entry.ActorID)
	}
	if entry.EntityType != "user" || entry.EntityID != target.ID {
		t.Errorf("expected entity=user/%d, got %s/%d", target.ID, entry.EntityType, entry.EntityID)
	}
	if entry.Before != nil || entry.After != nil {
		t.Errorf("expected no Before/After payload, got Before=%v After=%v", entry.Before, entry.After)
	}
}

func TestDeactivateUserService_Deactivate_TargetNotFound(t *testing.T) {
	repo := &stubUserRepo{findByIDResult: nil}
	auditW := &fakeAuditWriter{}

	svc := NewDeactivateUserService(repo, auditW)

	err := svc.Deactivate(context.Background(), 1, 999, "127.0.0.1")

	if !errors.Is(err, pkgauth.ErrUserNotFound) {
		t.Errorf("expected pkgauth.ErrUserNotFound, got: %v", err)
	}
	if repo.deactivateCalled {
		t.Error("expected Deactivate NOT to be called for a nonexistent target")
	}
}

func TestDeactivateUserService_Deactivate_AuditWriteFails_PropagatesError(t *testing.T) {
	target := &pkgauth.UserRecord{ID: 7, Username: "audited.user"}
	repo := &stubUserRepo{findByIDResult: target}
	auditW := &fakeAuditWriter{forceErr: errors.New("audit sink down")}

	svc := NewDeactivateUserService(repo, auditW)

	err := svc.Deactivate(context.Background(), 1, target.ID, "127.0.0.1")

	if err == nil {
		t.Fatal("expected an error when the audit write fails")
	}
}

func TestDeactivateUserService_Reactivate_AuditWriteFails_PropagatesError(t *testing.T) {
	repo := &stubUserRepo{}
	auditW := &fakeAuditWriter{forceErr: errors.New("audit sink down")}

	svc := NewDeactivateUserService(repo, auditW)

	err := svc.Reactivate(context.Background(), 1, 7, "127.0.0.1")

	if err == nil {
		t.Fatal("expected an error when the audit write fails")
	}
}

func TestDeactivateUserService_Reactivate_HappyPath(t *testing.T) {
	repo := &stubUserRepo{}
	auditW := &fakeAuditWriter{}

	svc := NewDeactivateUserService(repo, auditW)

	const actorID = 99
	targetID := int64(7)
	if err := svc.Reactivate(context.Background(), actorID, targetID, "127.0.0.1"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !repo.reactivateCalled {
		t.Error("expected Reactivate to be called")
	}
	if repo.reactivateUserID != targetID {
		t.Errorf("expected Reactivate called with userID=%d, got %d", targetID, repo.reactivateUserID)
	}
	if len(auditW.entries) != 1 || auditW.entries[0].Action != "user_reactivated" {
		t.Fatalf("expected 1 audit entry with action=user_reactivated, got %+v", auditW.entries)
	}
}
