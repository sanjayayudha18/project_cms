package auth

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/pkg/auth"
)

// SetInitialPasswordRequest holds the data for an APPACCESS admin action
// that sets a target user's initial (or reset) password.
type SetInitialPasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required"`
}

// SetInitialPasswordService lets APPACCESS set a target user's password
// without knowing their old one, forcing a change on the target's next
// login. Distinct from ChangePasswordService (self-service, requires the
// old password) — see Task 5 vs Task 6 in Auth-Local-Lifecycle/task.md.
type SetInitialPasswordService struct {
	userRepo auth.UserRepository
	audit    AuditWriter
}

// NewSetInitialPasswordService creates a SetInitialPasswordService with the given dependencies.
func NewSetInitialPasswordService(userRepo auth.UserRepository, auditWriter AuditWriter) *SetInitialPasswordService {
	return &SetInitialPasswordService{userRepo: userRepo, audit: auditWriter}
}

// SetInitialPassword sets targetUserID's password. actorID is the APPACCESS
// user performing the action (recorded as the audit actor, distinct from
// the target user); actorIP is recorded on the audit_logs entry.
func (s *SetInitialPasswordService) SetInitialPassword(ctx context.Context, actorID, targetUserID int64, req SetInitialPasswordRequest, actorIP string) error {
	target, err := s.userRepo.FindByID(ctx, targetUserID)
	if err != nil {
		return auth.ErrServiceUnavailable
	}
	if target == nil {
		return auth.ErrUserNotFound
	}

	if !isLocalAuthSource(target.AuthSource) {
		return auth.ErrChangeNotAllowed
	}

	if err := auth.ValidatePasswordStrength(req.NewPassword); err != nil {
		return err
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), BcryptCost)
	if err != nil {
		return auth.ErrServiceUnavailable
	}

	if err := s.userRepo.SetInitialPassword(ctx, targetUserID, string(newHash)); err != nil {
		return auth.ErrServiceUnavailable
	}

	// Never include password/hash values in the audit trail.
	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    actorID,
		Action:     "initial_password_set",
		EntityType: "user",
		EntityID:   targetUserID,
		IP:         actorIP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}
