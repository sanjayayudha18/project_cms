package auth

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/pkg/auth"
)

// AuditWriter is the one method ChangePasswordService needs from
// audit.Writer, kept as an interface so tests can fake it without a real DB
// (mirrors internal/approval.AuditWriter).
type AuditWriter interface {
	Write(ctx context.Context, entry audit.Entry) error
}

// ChangePasswordRequest holds the data for a self-service password change.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

// ChangePasswordService lets an already-authenticated local-password user
// change their own password. This is a change, not a reset: the correct old
// password is required. LDAP/Entra accounts are rejected — their password is
// managed by the external directory, not CMS. See Task 6 for APPACCESS's
// set-initial-password (admin-side, no old password needed).
type ChangePasswordService struct {
	userRepo auth.UserRepository
	audit    AuditWriter
}

// NewChangePasswordService creates a ChangePasswordService with the given dependencies.
func NewChangePasswordService(userRepo auth.UserRepository, auditWriter AuditWriter) *ChangePasswordService {
	return &ChangePasswordService{userRepo: userRepo, audit: auditWriter}
}

// ChangePassword verifies the old password, validates the new one, and
// persists the change. actorIP is recorded on the audit_logs entry.
func (s *ChangePasswordService) ChangePassword(ctx context.Context, userID int64, req ChangePasswordRequest, actorIP string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return auth.ErrServiceUnavailable
	}
	if user == nil {
		return auth.ErrInvalidCredentials
	}

	if !isLocalAuthSource(user.AuthSource) {
		return auth.ErrChangeNotAllowed
	}

	if user.PasswordHash == nil || bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.OldPassword)) != nil {
		return auth.ErrInvalidCredentials
	}

	if req.NewPassword == req.OldPassword {
		return auth.ErrPasswordUnchanged
	}

	if err := auth.ValidatePasswordStrength(req.NewPassword); err != nil {
		return err
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), BcryptCost)
	if err != nil {
		return auth.ErrServiceUnavailable
	}

	if err := s.userRepo.SetPassword(ctx, userID, string(newHash)); err != nil {
		return auth.ErrServiceUnavailable
	}

	// Never include password/hash values in the audit trail.
	if err := s.audit.Write(ctx, audit.Entry{
		ActorID:    userID,
		Action:     "password_change",
		EntityType: "user",
		EntityID:   userID,
		IP:         actorIP,
	}); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}
