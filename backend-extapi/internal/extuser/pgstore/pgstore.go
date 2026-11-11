// Package pgstore is the PostgreSQL implementation of extuser.Tx, extuser.Store
// and extuser.Reader on top of the sqlc queries (primary pool only). It lives
// outside internal/repository because audit imports repository and extuser
// imports audit, so repository cannot import extuser.
package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-extapi/internal/audit"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
)

// TxStore implements extuser.Tx: one primary transaction per call.
type TxStore struct{ runner *repository.TxRunner }

func NewTxStore(primary *pgxpool.Pool) *TxStore {
	return &TxStore{runner: repository.NewTxRunner(primary)}
}

// InTx commits when fn returns nil, rolls back otherwise.
func (t *TxStore) InTx(ctx context.Context, fn func(s extuser.Store) error) error {
	return t.runner.Run(ctx, func(tx repository.DBTX) error {
		return fn(&store{db: tx, q: repository.New(tx), audit: audit.NewWriter()})
	})
}

// Reader implements extuser.Reader on the PRIMARY pool (replica may lag).
type Reader struct{ q *repository.Queries }

func NewReader(primary repository.DBTX) *Reader { return &Reader{q: repository.New(primary)} }

func (r *Reader) GetUser(ctx context.Context, id int64) (*extuser.User, error) {
	return getUser(ctx, r.q, id)
}

func (r *Reader) GetUserByUsername(ctx context.Context, username string) (*extuser.User, error) {
	row, err := r.q.GetExtUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return toUser(repository.GetExtUserRow(row)), nil
}

type store struct {
	db    repository.DBTX
	q     *repository.Queries
	audit *audit.Writer
}

var (
	_ extuser.Store  = (*store)(nil)
	_ extuser.Tx     = (*TxStore)(nil)
	_ extuser.Reader = (*Reader)(nil)
)

func (s *store) LockUser(ctx context.Context, id int64) (*extuser.LockedUser, error) {
	row, err := s.q.LockUserForExt(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock user: %w", err)
	}
	return &extuser.LockedUser{
		ID: row.ID, AuthSource: row.AuthSource, Role: row.Role,
		IsActive: row.IsActive, Deleted: row.DeletedAt.Valid,
	}, nil
}

func (s *store) GetUser(ctx context.Context, id int64) (*extuser.User, error) {
	return getUser(ctx, s.q, id)
}

func (s *store) FindIDByUsername(ctx context.Context, username string) (*int64, error) {
	return optionalID(s.q.FindUserIDByUsername(ctx, username))
}

func (s *store) FindIDByEmail(ctx context.Context, email string, excludeID *int64) (*int64, error) {
	if excludeID == nil {
		return optionalID(s.q.FindUserIDByEmail(ctx, email))
	}
	return optionalID(s.q.FindUserIDByEmailExcludingID(ctx,
		repository.FindUserIDByEmailExcludingIDParams{Email: email, ID: *excludeID}))
}

func (s *store) FindIDByEmployeeID(ctx context.Context, employeeID string, excludeID *int64) (*int64, error) {
	if excludeID == nil {
		return optionalID(s.q.FindUserIDByEmployeeID(ctx, &employeeID))
	}
	return optionalID(s.q.FindUserIDByEmployeeIDExcludingID(ctx,
		repository.FindUserIDByEmployeeIDExcludingIDParams{EmployeeID: &employeeID, ID: *excludeID}))
}

func (s *store) RoleIDByName(ctx context.Context, role string) (*int64, error) {
	return optionalID(s.q.GetRoleIDByName(ctx, role))
}

func (s *store) Create(ctx context.Context, p extuser.CreateParams) (int64, error) {
	return s.q.CreateExtUser(ctx, repository.CreateExtUserParams{
		RoleID: p.RoleID, EmployeeID: p.EmployeeID, Username: p.Username, FullName: p.FullName,
		Email: p.Email, SupervisorID: p.SupervisorID, ApprovalLevel: p.ApprovalLevel,
	})
}

// Update returns extuser.ErrUserNotFound when no live row matched.
func (s *store) Update(ctx context.Context, p extuser.UpdateParams) error {
	return oneRow(s.q.UpdateExtUser(ctx, repository.UpdateExtUserParams{
		ID: p.ID, RoleID: p.RoleID, FullName: p.FullName, Email: p.Email,
		EmployeeID: p.EmployeeID, SupervisorID: p.SupervisorID, ApprovalLevel: p.ApprovalLevel,
	}))
}

func (s *store) Deactivate(ctx context.Context, id int64) error {
	return oneRow(s.q.DeactivateExtUser(ctx, id))
}

func (s *store) Reactivate(ctx context.Context, id int64) error {
	return oneRow(s.q.ReactivateExtUser(ctx, id))
}

func (s *store) WriteAudit(ctx context.Context, e audit.Entry) error {
	return s.audit.Write(ctx, s.db, e)
}

func getUser(ctx context.Context, q *repository.Queries, id int64) (*extuser.User, error) {
	row, err := q.GetExtUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return toUser(row), nil
}

func toUser(r repository.GetExtUserRow) *extuser.User {
	return &extuser.User{
		ID: r.ID, Username: r.Username, FullName: r.FullName, Email: r.Email, Role: r.Role,
		EmployeeID: r.EmployeeID, SupervisorID: r.SupervisorID, ApprovalLevel: r.ApprovalLevel,
		IsActive: r.IsActive, AuthSource: r.AuthSource, DeletedAt: timePtr(r.DeletedAt),
		CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC(),
	}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// optionalID maps "no row" to nil, nil.
func optionalID(id int64, err error) (*int64, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// oneRow: the target is locked FOR UPDATE, so zero rows means it is gone or
// already in the target state; reported as not found.
func oneRow(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return extuser.ErrUserNotFound
	}
	return nil
}
