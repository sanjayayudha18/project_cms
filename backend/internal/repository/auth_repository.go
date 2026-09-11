package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/auth"
)

// timestamptzToPtr converts a pgx pgtype.Timestamptz into a *time.Time,
// returning nil when the column was SQL NULL. sqlc >= 1.29 generates
// pgtype.Timestamptz for nullable timestamptz columns instead of
// *time.Time, so callers that need the pointer form (e.g. auth.UserRecord)
// convert at the repository boundary.
func timestamptzToPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// AuthRepository implements auth.UserRepository using sqlc-generated queries.
type AuthRepository struct {
	queries *db.Queries
}

// NewAuthRepository creates a new AuthRepository wrapping the given database connection.
func NewAuthRepository(dbConn db.DBTX) *AuthRepository {
	return &AuthRepository{queries: db.New(dbConn)}
}

// FindByUsername retrieves a user by username where deleted_at IS NULL.
// Returns nil, nil if no matching user is found.
func (r *AuthRepository) FindByUsername(ctx context.Context, username string) (*auth.UserRecord, error) {
	row, err := r.queries.FindUserByUsername(ctx, username)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &auth.UserRecord{
		ID:            row.ID,
		Username:      row.Username,
		FullName:      row.FullName,
		Email:         row.Email,
		PasswordHash:  row.PasswordHash,
		AuthSource:    row.AuthSource,
		RoleID:        row.RoleID,
		Role:          row.Role,
		IsKaryawan:    row.IsKaryawan,
		VendorID:      row.VendorID,
		IsActive:      row.IsActive,
		DeletedAt:     timestamptzToPtr(row.DeletedAt),
		SupervisorID:  row.SupervisorID,
		ApprovalLevel: row.ApprovalLevel,
		PasswordChangedAt:   timestamptzToPtr(row.PasswordChangedAt),
		FailedLoginAttempts: row.FailedLoginAttempts,
		LockedUntil:         timestamptzToPtr(row.LockedUntil),
		MustChangePassword:  row.MustChangePassword,
	}, nil
}

// FindByEmail retrieves a user by email where deleted_at IS NULL. Used by
// the login flow now that users sign in with email instead of username.
// Returns nil, nil if no matching user is found.
func (r *AuthRepository) FindByEmail(ctx context.Context, email string) (*auth.UserRecord, error) {
	row, err := r.queries.FindUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &auth.UserRecord{
		ID:            row.ID,
		Username:      row.Username,
		FullName:      row.FullName,
		Email:         row.Email,
		PasswordHash:  row.PasswordHash,
		AuthSource:    row.AuthSource,
		RoleID:        row.RoleID,
		Role:          row.Role,
		IsKaryawan:    row.IsKaryawan,
		VendorID:      row.VendorID,
		IsActive:      row.IsActive,
		DeletedAt:     timestamptzToPtr(row.DeletedAt),
		SupervisorID:  row.SupervisorID,
		ApprovalLevel: row.ApprovalLevel,
		PasswordChangedAt:   timestamptzToPtr(row.PasswordChangedAt),
		FailedLoginAttempts: row.FailedLoginAttempts,
		LockedUntil:         timestamptzToPtr(row.LockedUntil),
		MustChangePassword:  row.MustChangePassword,
	}, nil
}

// FindByID retrieves a user by ID where deleted_at IS NULL, including the
// auth-related fields FindByUsername returns (password_hash, auth_source,
// lockout state) — used by self-service actions like change-password.
// Returns nil, nil if no matching user is found.
func (r *AuthRepository) FindByID(ctx context.Context, userID int64) (*auth.UserRecord, error) {
	row, err := r.queries.FindUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &auth.UserRecord{
		ID:                  row.ID,
		Username:            row.Username,
		FullName:            row.FullName,
		Email:               row.Email,
		PasswordHash:        row.PasswordHash,
		AuthSource:          row.AuthSource,
		RoleID:              row.RoleID,
		Role:                row.Role,
		IsKaryawan:          row.IsKaryawan,
		VendorID:            row.VendorID,
		IsActive:            row.IsActive,
		DeletedAt:           timestamptzToPtr(row.DeletedAt),
		SupervisorID:        row.SupervisorID,
		ApprovalLevel:       row.ApprovalLevel,
		PasswordChangedAt:   timestamptzToPtr(row.PasswordChangedAt),
		FailedLoginAttempts: row.FailedLoginAttempts,
		LockedUntil:         timestamptzToPtr(row.LockedUntil),
		MustChangePassword:  row.MustChangePassword,
	}, nil
}

// SetPassword sets a new bcrypt password hash, marks password_changed_at as
// now, clears must_change_password, and resets the lockout counters.
func (r *AuthRepository) SetPassword(ctx context.Context, userID int64, newPasswordHash string) error {
	return r.queries.SetPassword(ctx, db.SetPasswordParams{
		ID:           userID,
		PasswordHash: &newPasswordHash,
	})
}

// SetInitialPassword sets a new bcrypt password hash for a target user and
// forces a change on their next login (must_change_password=true).
func (r *AuthRepository) SetInitialPassword(ctx context.Context, userID int64, newPasswordHash string) error {
	return r.queries.SetInitialPassword(ctx, db.SetInitialPasswordParams{
		ID:           userID,
		PasswordHash: &newPasswordHash,
	})
}

// Deactivate soft-deletes a user: is_active=false, deleted_at=now().
func (r *AuthRepository) Deactivate(ctx context.Context, userID int64) error {
	return r.queries.DeactivateUser(ctx, userID)
}

// Reactivate reverses Deactivate: is_active=true, deleted_at=NULL.
func (r *AuthRepository) Reactivate(ctx context.Context, userID int64) error {
	return r.queries.ReactivateUser(ctx, userID)
}

// UpdateLastLogin sets last_login_at to the current time for the given user.
func (r *AuthRepository) UpdateLastLogin(ctx context.Context, userID int64) error {
	return r.queries.UpdateLastLogin(ctx, userID)
}

// MarkPasswordExpired sets must_change_password=true for the given user,
// used when the local-password 90-day expiry policy rejects a login.
func (r *AuthRepository) MarkPasswordExpired(ctx context.Context, userID int64) error {
	return r.queries.MarkPasswordExpired(ctx, userID)
}

// IncrementFailedLogin increments failed_login_attempts by 1 and returns the
// new count. The caller (service layer) decides whether the new count
// reaches MaxFailedLogins and, if so, calls LockAccount.
func (r *AuthRepository) IncrementFailedLogin(ctx context.Context, userID int64) (int32, error) {
	return r.queries.IncrementFailedLogin(ctx, userID)
}

// LockAccount sets locked_until for the given user.
func (r *AuthRepository) LockAccount(ctx context.Context, userID int64, until time.Time) error {
	return r.queries.LockAccount(ctx, db.LockAccountParams{
		ID:          userID,
		LockedUntil: pgtype.Timestamptz{Time: until, Valid: true},
	})
}

// ResetLockout clears failed_login_attempts and locked_until, used after a
// successful login.
func (r *AuthRepository) ResetLockout(ctx context.Context, userID int64) error {
	return r.queries.ResetLockout(ctx, userID)
}

// GetUserProfile retrieves a user's profile by ID for the /me endpoint.
// Returns nil, nil if no matching user is found.
func (r *AuthRepository) GetUserProfile(ctx context.Context, userID int64) (*auth.UserRecord, error) {
	row, err := r.queries.GetUserProfile(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &auth.UserRecord{
		ID:            row.ID,
		Username:      row.Username,
		FullName:      row.FullName,
		Email:         row.Email,
		Role:          row.Role,
		IsKaryawan:    row.IsKaryawan,
		VendorID:      row.VendorID,
		SupervisorID:  row.SupervisorID,
		ApprovalLevel: row.ApprovalLevel,
	}, nil
}
