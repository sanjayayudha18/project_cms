package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// UserAdminRepository is the primary-pool repository backing the admin user
// CRUD endpoints (Req 2-5): list/get/create/update, plus the reference/
// uniqueness pre-checks the service layer needs before writing. Disable/
// enable reuse the existing AuthRepository.Deactivate/Reactivate via
// DeactivateUserService — not duplicated here.
//
// ponytail: uses dbPool for both reads and writes; swap List/Count to the
// dbRead pool when DATABASE_REPLICA_URL wiring lands (same TODO convention
// as AuditLogRepository / RbacReadRepository).
type UserAdminRepository struct {
	queries *db.Queries
}

// NewUserAdminRepository creates a UserAdminRepository wrapping the given database connection.
func NewUserAdminRepository(dbConn db.DBTX) *UserAdminRepository {
	return &UserAdminRepository{queries: db.New(dbConn)}
}

// List returns a page of users matching the given filters (Req 2).
func (r *UserAdminRepository) List(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error) {
	return r.queries.ListUsersAdmin(ctx, arg)
}

// Count returns the total number of users matching the given filters (same
// filter fields as List, without pagination) (Req 2.8).
func (r *UserAdminRepository) Count(ctx context.Context, arg db.CountUsersAdminParams) (int64, error) {
	return r.queries.CountUsersAdmin(ctx, arg)
}

// GetByID returns a user by id, including soft-deleted rows. Returns nil,
// nil if no matching user is found.
func (r *UserAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error) {
	row, err := r.queries.GetUserAdminByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// Create inserts a new user (Req 3). See CreateUserAdmin's doc comment for
// the single-INSERT password handling (Resolved Decision 5).
func (r *UserAdminRepository) Create(ctx context.Context, arg db.CreateUserAdminParams) (db.CreateUserAdminRow, error) {
	return r.queries.CreateUserAdmin(ctx, arg)
}

// Update overwrites a user's editable fields (Req 4). Returns pgx.ErrNoRows
// if the target id does not exist or is soft-disabled (the query filters
// deleted_at IS NULL) -- the caller maps that to 404 (Req 4.4).
func (r *UserAdminRepository) Update(ctx context.Context, arg db.UpdateUserAdminParams) (db.UpdateUserAdminRow, error) {
	return r.queries.UpdateUserAdmin(ctx, arg)
}

// FindByUsername returns the id of the user with the given username,
// including soft-deleted rows (Req 3.5 uniqueness pre-check). Returns nil,
// nil if no matching user is found.
func (r *UserAdminRepository) FindByUsername(ctx context.Context, username string) (*int64, error) {
	id, err := r.queries.FindUserAdminByUsername(ctx, username)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// FindByEmail returns the id of the user with the given email, including
// soft-deleted rows (Req 3.5 uniqueness pre-check). Returns nil, nil if no
// matching user is found.
func (r *UserAdminRepository) FindByEmail(ctx context.Context, email string) (*int64, error) {
	id, err := r.queries.FindUserAdminByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// FindByEmailExcludingID is FindByEmail scoped to Req 4.5's update
// uniqueness pre-check: a collision with a DIFFERENT record than excludeID.
func (r *UserAdminRepository) FindByEmailExcludingID(ctx context.Context, email string, excludeID int64) (*int64, error) {
	id, err := r.queries.FindUserAdminByEmailExcludingID(ctx, db.FindUserAdminByEmailExcludingIDParams{
		Email: email,
		ID:    excludeID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// FindByEmployeeID returns the id of the user with the given employee_id,
// including soft-deleted rows (Req 3.5 uniqueness pre-check). Returns nil,
// nil if no matching user is found. Callers only invoke this when
// employeeID is non-empty (employee_id is nullable).
func (r *UserAdminRepository) FindByEmployeeID(ctx context.Context, employeeID string) (*int64, error) {
	id, err := r.queries.FindUserAdminByEmployeeID(ctx, &employeeID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// FindByEmployeeIDExcludingID is FindByEmployeeID scoped to Req 4.5's update
// uniqueness pre-check.
func (r *UserAdminRepository) FindByEmployeeIDExcludingID(ctx context.Context, employeeID string, excludeID int64) (*int64, error) {
	id, err := r.queries.FindUserAdminByEmployeeIDExcludingID(ctx, db.FindUserAdminByEmployeeIDExcludingIDParams{
		EmployeeID: &employeeID,
		ID:         excludeID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// GetRoleByName resolves a role text (e.g. "APPACCESS") to its role_id (Req
// 3.6, 4.7). Returns nil, nil if the role does not exist.
func (r *UserAdminRepository) GetRoleByName(ctx context.Context, role string) (*db.GetRoleByNameRow, error) {
	row, err := r.queries.GetRoleByName(ctx, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// ListRoles returns every role, for the Role select in User_Form_Dialog (Req 11.2).
func (r *UserAdminRepository) ListRoles(ctx context.Context) ([]db.ListRolesRow, error) {
	return r.queries.ListRoles(ctx)
}

// VendorExists reports whether an active (non-disabled) vendor exists with
// the given id -- the vendor_id reference check on user create/update (Req
// 3.7, 4.7).
func (r *UserAdminRepository) VendorExists(ctx context.Context, vendorID int64) (bool, error) {
	return r.queries.VendorExistsAdmin(ctx, vendorID)
}

// SupervisorExists reports whether an active (non-disabled) user exists with
// the given id -- the supervisor_id reference check on user create/update
// (Req 3.7, 4.7).
func (r *UserAdminRepository) SupervisorExists(ctx context.Context, userID int64) (bool, error) {
	return r.queries.UserAdminExists(ctx, userID)
}
