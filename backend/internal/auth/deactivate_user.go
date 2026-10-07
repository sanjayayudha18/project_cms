package auth

import (
	"context"
	"fmt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/pkg/auth"
)

// DeactivateUserService soft-deletes/reactivates user accounts. There is no
// hard-delete path anywhere in this codebase (Task 7): a user who has ever
// acted (audit_logs.actor_id -> users.id) must stay linked forever, and the
// normal flow is soft-delete for every user regardless of audit history —
// one consistent code path, not a conditional hard-delete for "clean" users.
type DeactivateUserService struct {
	userRepo auth.UserRepository
	audit    AuditWriter
}

// NewDeactivateUserService creates a DeactivateUserService with the given dependencies.
func NewDeactivateUserService(userRepo auth.UserRepository, auditWriter AuditWriter) *DeactivateUserService {
	return &DeactivateUserService{userRepo: userRepo, audit: auditWriter}
}

// Deactivate soft-deletes targetUserID: is_active=false, deleted_at=now().
// FindUserByUsername already filters deleted_at IS NULL, so this closes the
// user's login immediately without touching the users row itself — audit
// trail referencing them stays intact. actorID/actorIP are recorded on the
// audit_logs entry.
func (s *DeactivateUserService) Deactivate(ctx context.Context, actorID, targetUserID int64, actorIP string) error {
	target, err := s.userRepo.FindByID(ctx, targetUserID)
	if err != nil {
		return auth.ErrServiceUnavailable
	}
	if target == nil {
		return auth.ErrUserNotFound
	}

	if err := s.userRepo.Deactivate(ctx, targetUserID); err != nil {
		return auth.ErrServiceUnavailable
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "user_deactivated",
		EntityType: "user",
		EntityID:   targetUserID,
		IP:         actorIP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}

// Reactivate reverses Deactivate: is_active=true, deleted_at=NULL.
//
// ponytail: no pre-check that targetUserID currently exists/is deactivated —
// FindByID filters deleted_at IS NULL so it can't see a soft-deleted row to
// verify against, and this is an optional admin action per the spec. A
// bogus id just no-ops (0 rows updated, no error); add a
// FindByIDIncludingDeleted lookup if a real 404 distinction is ever needed.
func (s *DeactivateUserService) Reactivate(ctx context.Context, actorID, targetUserID int64, actorIP string) error {
	if err := s.userRepo.Reactivate(ctx, targetUserID); err != nil {
		return auth.ErrServiceUnavailable
	}

	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "user_reactivated",
		EntityType: "user",
		EntityID:   targetUserID,
		IP:         actorIP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}
