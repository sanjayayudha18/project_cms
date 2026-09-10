package auth

import (
	"context"
	"time"
)

// UserRecord represents a user row from the database, including the joined role name.
type UserRecord struct {
	ID           int64
	Username     string
	FullName     string
	Email        string
	PasswordHash *string // nullable for LDAP users
	AuthSource   string  // "ldap", "local", "local_dev"
	RoleID       int64
	Role         string // joined from roles table
	IsKaryawan   bool
	VendorID     *int64 // nullable — only set for vendor users
	IsActive     bool
	DeletedAt    *time.Time // nullable (soft delete)
	SupervisorID *int64     // nullable — reporting line (RBAC-Setup)
	ApprovalLevel *int32    // nullable — maker-checker level, independent of Role
	PasswordChangedAt *time.Time // nullable — local-password policy only (Auth-Local-Lifecycle); nil = not yet evaluated
	FailedLoginAttempts int32    // local-password lockout policy only; 0 for LDAP users
	LockedUntil       *time.Time // nullable — local-password lockout policy only; nil = not locked
	MustChangePassword bool      // forces a password change on next login (Auth-Local-Lifecycle)
}

// UserRepository abstracts database access for user-related queries
// used by the auth service.
type UserRepository interface {
	// FindByUsername retrieves a user by username where deleted_at IS NULL.
	// Returns nil, nil if no matching user is found.
	FindByUsername(ctx context.Context, username string) (*UserRecord, error)

	// UpdateLastLogin sets last_login_at to the current time for the given user.
	UpdateLastLogin(ctx context.Context, userID int64) error

	// GetUserProfile retrieves a user's profile by ID for the /me endpoint.
	// Returns nil, nil if no matching user is found.
	GetUserProfile(ctx context.Context, userID int64) (*UserRecord, error)

	// MarkPasswordExpired sets must_change_password=true for the given user,
	// used when the local-password 90-day expiry policy rejects a login.
	MarkPasswordExpired(ctx context.Context, userID int64) error

	// IncrementFailedLogin increments failed_login_attempts by 1 and returns
	// the new count. The caller (service layer) decides whether the new
	// count reaches MaxFailedLogins and, if so, calls LockAccount.
	IncrementFailedLogin(ctx context.Context, userID int64) (int32, error)

	// LockAccount sets locked_until for the given user.
	LockAccount(ctx context.Context, userID int64, until time.Time) error

	// ResetLockout clears failed_login_attempts and locked_until, used after
	// a successful login.
	ResetLockout(ctx context.Context, userID int64) error

	// FindByID retrieves a user by ID where deleted_at IS NULL, including the
	// auth-related fields (password_hash, auth_source) that GetUserProfile
	// omits — used by self-service actions like change-password where the
	// caller is already authenticated via JWT (UserID from AuthContext).
	// Returns nil, nil if no matching user is found.
	FindByID(ctx context.Context, userID int64) (*UserRecord, error)

	// SetPassword sets a new bcrypt password hash, marks password_changed_at
	// as now, clears must_change_password, and resets the lockout counters —
	// used by self-service change-password (Task 5).
	SetPassword(ctx context.Context, userID int64, newPasswordHash string) error

	// SetInitialPassword sets a new bcrypt password hash for a target user
	// and forces a change on their next login (must_change_password=true) —
	// used by APPACCESS's admin set-initial-password (Task 6). No old
	// password check: this is a set, not a self-service change.
	SetInitialPassword(ctx context.Context, userID int64, newPasswordHash string) error

	// Deactivate soft-deletes a user: is_active=false, deleted_at=now().
	// There is no hard-delete counterpart anywhere in this codebase (Task
	// 7) — audit_logs rows referencing this user must stay linked forever.
	Deactivate(ctx context.Context, userID int64) error

	// Reactivate reverses Deactivate: is_active=true, deleted_at=NULL.
	Reactivate(ctx context.Context, userID int64) error
}

// RateLimiter abstracts login rate limiting for testability.
// The concrete implementation lives in pkg/middleware.
type RateLimiter interface {
	Check(ctx context.Context, username, ip string) error
	IncrementFailed(ctx context.Context, username, ip string) error
	ResetUsername(ctx context.Context, username string) error
}
