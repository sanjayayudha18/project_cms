package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

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
		ID:           row.ID,
		Username:     row.Username,
		FullName:     row.FullName,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		AuthSource:   row.AuthSource,
		RoleID:       row.RoleID,
		Role:         row.Role,
		IsKaryawan:   row.IsKaryawan,
		VendorID:     row.VendorID,
		IsActive:     row.IsActive,
		DeletedAt:    row.DeletedAt,
	}, nil
}

// UpdateLastLogin sets last_login_at to the current time for the given user.
func (r *AuthRepository) UpdateLastLogin(ctx context.Context, userID int64) error {
	return r.queries.UpdateLastLogin(ctx, userID)
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
		ID:         row.ID,
		Username:   row.Username,
		FullName:   row.FullName,
		Email:      row.Email,
		Role:       row.Role,
		IsKaryawan: row.IsKaryawan,
		VendorID:   row.VendorID,
	}, nil
}
