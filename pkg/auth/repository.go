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
}

// RateLimiter abstracts login rate limiting for testability.
// The concrete implementation lives in pkg/middleware.
type RateLimiter interface {
	Check(ctx context.Context, username, ip string) error
	IncrementFailed(ctx context.Context, username, ip string) error
	ResetUsername(ctx context.Context, username string) error
}
