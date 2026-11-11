// Package extuser creates, updates, disables and enables users on behalf of
// an external client (ext-user-api Req 5–10). Every write runs in one primary
// transaction: lock the target, check the Managed_User rule, change users,
// write audit_logs as the client's service account — or roll everything back.
package extuser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend-extapi/internal/audit"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

var (
	ErrRoleNotAllowed  = errors.New("role tidak boleh diberikan atau diubah lewat API eksternal")
	ErrUserNotFound    = errors.New("pengguna tidak ditemukan")
	ErrSelfSupervision = errors.New("supervisor_id tidak boleh sama dengan id sendiri")
)

// Actor is the calling client, built by the handler from ClientContext.
type Actor struct {
	ServiceUserID   int64
	ClientID        string
	AssignableRoles []string
	IP              string
}

// CanAssign: never a privileged role, and only the client's allowlist.
func (a Actor) CanAssign(role string) bool {
	return extauth.Client{AssignableRoles: a.AssignableRoles}.CanAssign(role)
}

// User is the stored user without password data; also the audit snapshot.
type User struct {
	ID            int64      `json:"id"`
	Username      string     `json:"username"`
	FullName      string     `json:"full_name"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	EmployeeID    *string    `json:"employee_id"`
	SupervisorID  *int64     `json:"supervisor_id"`
	ApprovalLevel *int32     `json:"approval_level"`
	IsActive      bool       `json:"is_active"`
	AuthSource    string     `json:"auth_source"`
	DeletedAt     *time.Time `json:"deleted_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// LockedUser is the row state read under FOR UPDATE.
type LockedUser struct {
	ID         int64
	AuthSource string
	Role       string
	IsActive   bool
	Deleted    bool // deleted_at IS NOT NULL
}

func (u *LockedUser) active() bool { return u.IsActive && !u.Deleted }

// managedSource: only ldap users are Managed_Users (D13). Service accounts and
// vendor/local users never are.
func managedSource(authSource string) bool { return authSource == userrules.AuthSourceLDAP }

type CreateRequest struct {
	Username      string
	FullName      string
	Email         string
	Role          string
	EmployeeID    string
	SupervisorID  *int64
	ApprovalLevel *int32
}

// UpdateRequest replaces every editable field (PUT).
type UpdateRequest struct {
	FullName      string
	Email         string
	Role          string
	EmployeeID    string
	SupervisorID  *int64
	ApprovalLevel *int32
}

type CreateParams struct {
	RoleID        int64
	Username      string
	FullName      string
	Email         string
	EmployeeID    *string
	SupervisorID  *int64
	ApprovalLevel *int32
}

type UpdateParams struct {
	ID            int64
	RoleID        int64
	FullName      string
	Email         string
	EmployeeID    *string
	SupervisorID  *int64
	ApprovalLevel *int32
}

// Store is one instance per transaction. Lookups return nil, nil when absent.
type Store interface {
	LockUser(ctx context.Context, id int64) (*LockedUser, error)
	GetUser(ctx context.Context, id int64) (*User, error)
	FindIDByUsername(ctx context.Context, username string) (*int64, error)
	FindIDByEmail(ctx context.Context, email string, excludeID *int64) (*int64, error)
	FindIDByEmployeeID(ctx context.Context, employeeID string, excludeID *int64) (*int64, error)
	RoleIDByName(ctx context.Context, role string) (*int64, error)
	Create(ctx context.Context, p CreateParams) (int64, error)
	Update(ctx context.Context, p UpdateParams) error
	Deactivate(ctx context.Context, id int64) error
	Reactivate(ctx context.Context, id int64) error
	WriteAudit(ctx context.Context, e audit.Entry) error
}

// Tx commits when fn returns nil and rolls back otherwise.
type Tx interface {
	InTx(ctx context.Context, fn func(s Store) error) error
}

// Reader reads from the PRIMARY pool (IDM re-checks after a timeout; a
// replica may lag). Returns nil, nil when absent.
type Reader interface {
	GetUser(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
}

type Service struct {
	tx     Tx
	reader Reader
}

func NewService(tx Tx, reader Reader) *Service { return &Service{tx: tx, reader: reader} }

// Create makes an ldap user without a password (Req 5).
func (s *Service) Create(ctx context.Context, a Actor, req CreateRequest) (User, error) {
	username := strings.TrimSpace(req.Username)
	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(req.Email)
	role := strings.TrimSpace(req.Role)
	employeeID := strings.TrimSpace(req.EmployeeID)
	if username == "" {
		return User{}, &auth.ValidationError{Field: "username", Message: "wajib diisi"}
	}
	if err := userrules.ValidateProfile(fullName, email, role); err != nil {
		return User{}, err
	}
	// Req 5.4: before touching the DB.
	if !a.CanAssign(role) {
		return User{}, ErrRoleNotAllowed
	}

	var out User
	err := s.tx.InTx(ctx, func(st Store) error {
		id, err := st.FindIDByUsername(ctx, username)
		if err := userrules.Uniqueness("username", id, err); err != nil {
			return err
		}
		id, err = st.FindIDByEmail(ctx, email, nil)
		if err := userrules.Uniqueness("email", id, err); err != nil {
			return err
		}
		if employeeID != "" {
			id, err = st.FindIDByEmployeeID(ctx, employeeID, nil)
			if err := userrules.Uniqueness("employee_id", id, err); err != nil {
				return err
			}
		}
		roleID, err := resolveRefs(ctx, st, role, req.SupervisorID)
		if err != nil {
			return err
		}
		newID, err := st.Create(ctx, CreateParams{
			RoleID: roleID, Username: username, FullName: fullName, Email: email,
			EmployeeID: userrules.NilIfEmpty(employeeID), SupervisorID: req.SupervisorID,
			ApprovalLevel: req.ApprovalLevel,
		})
		if err != nil {
			return mapWriteError("creating user", err)
		}
		after, err := mustGet(ctx, st, newID)
		if err != nil {
			return err
		}
		out = *after
		return writeAudit(ctx, st, a, "user_created", newID, nil, after)
	})
	return out, err
}

// Update replaces the editable fields of an active Managed_User (Req 6).
func (s *Service) Update(ctx context.Context, a Actor, id int64, req UpdateRequest) (User, error) {
	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(req.Email)
	role := strings.TrimSpace(req.Role)
	employeeID := strings.TrimSpace(req.EmployeeID)
	if err := userrules.ValidateProfile(fullName, email, role); err != nil {
		return User{}, err
	}

	var out User
	err := s.tx.InTx(ctx, func(st Store) error {
		locked, err := st.LockUser(ctx, id)
		if err != nil {
			return fmt.Errorf("locking user: %w", err)
		}
		if locked == nil || !locked.active() || !managedSource(locked.AuthSource) {
			return ErrUserNotFound
		}
		if userrules.IsPrivileged(locked.Role) || !a.CanAssign(role) {
			return ErrRoleNotAllowed
		}
		if req.SupervisorID != nil && *req.SupervisorID == id {
			return ErrSelfSupervision
		}
		before, err := mustGet(ctx, st, id)
		if err != nil {
			return err
		}
		if err := uniqueOnUpdate(ctx, st, before, email, employeeID); err != nil {
			return err
		}
		roleID, err := resolveRefs(ctx, st, role, req.SupervisorID)
		if err != nil {
			return err
		}
		if err := st.Update(ctx, UpdateParams{
			ID: id, RoleID: roleID, FullName: fullName, Email: email,
			EmployeeID: userrules.NilIfEmpty(employeeID), SupervisorID: req.SupervisorID,
			ApprovalLevel: req.ApprovalLevel,
		}); err != nil {
			return mapWriteError("updating user", err)
		}
		after, err := mustGet(ctx, st, id)
		if err != nil {
			return err
		}
		out = *after
		return writeAudit(ctx, st, a, "user_updated", id, before, after)
	})
	return out, err
}

// Disable soft-deletes a Managed_User; already disabled = no write, no audit (Req 7.2).
func (s *Service) Disable(ctx context.Context, a Actor, id int64) error {
	return s.tx.InTx(ctx, func(st Store) error {
		locked, err := lockManaged(ctx, st, id)
		if err != nil {
			return err
		}
		if locked.Deleted {
			return nil
		}
		_, err = change(ctx, st, a, id, "user_deactivated", st.Deactivate)
		return err
	})
}

// Enable reactivates a Managed_User; already active = no write, no audit (Req 8.2).
func (s *Service) Enable(ctx context.Context, a Actor, id int64) (User, error) {
	var out User
	err := s.tx.InTx(ctx, func(st Store) error {
		locked, err := lockManaged(ctx, st, id)
		if err != nil {
			return err
		}
		var u *User
		if locked.Deleted {
			u, err = change(ctx, st, a, id, "user_reactivated", st.Reactivate)
		} else {
			u, err = mustGet(ctx, st, id)
		}
		if err != nil {
			return err
		}
		out = *u
		return nil
	})
	return out, err
}

// Get returns a Managed_User, disabled ones included (Req 8.1).
func (s *Service) Get(ctx context.Context, id int64) (User, error) {
	u, err := s.reader.GetUser(ctx, id)
	return readable(u, err)
}

// GetByUsername is Get by exact username.
func (s *Service) GetByUsername(ctx context.Context, username string) (User, error) {
	u, err := s.reader.GetUserByUsername(ctx, strings.TrimSpace(username))
	return readable(u, err)
}

// readable: reads never reveal privileged users or non-ldap accounts.
func readable(u *User, err error) (User, error) {
	if err != nil {
		return User{}, fmt.Errorf("reading user: %w", err)
	}
	if u == nil || !managedSource(u.AuthSource) || userrules.IsPrivileged(u.Role) {
		return User{}, ErrUserNotFound
	}
	return *u, nil
}

// lockManaged: 404 for missing/non-ldap, 403 for a privileged current role.
func lockManaged(ctx context.Context, st Store, id int64) (*LockedUser, error) {
	locked, err := st.LockUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("locking user: %w", err)
	}
	if locked == nil || !managedSource(locked.AuthSource) {
		return nil, ErrUserNotFound
	}
	if userrules.IsPrivileged(locked.Role) {
		return nil, ErrRoleNotAllowed
	}
	return locked, nil
}

// change snapshots before/after around one state write, audits it, and
// returns the after snapshot.
func change(ctx context.Context, st Store, a Actor, id int64, action string, write func(context.Context, int64) error) (*User, error) {
	before, err := mustGet(ctx, st, id)
	if err != nil {
		return nil, err
	}
	if err := write(ctx, id); err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	after, err := mustGet(ctx, st, id)
	if err != nil {
		return nil, err
	}
	return after, writeAudit(ctx, st, a, action, id, before, after)
}

// resolveRefs: role must exist; a supervisor must be an active Managed_User (Req 5.5).
func resolveRefs(ctx context.Context, st Store, role string, supervisorID *int64) (int64, error) {
	roleID, err := st.RoleIDByName(ctx, role)
	if err != nil {
		return 0, fmt.Errorf("resolving role: %w", err)
	}
	if roleID == nil {
		return 0, &userrules.ReferenceError{Field: "role", Message: "tidak ditemukan"}
	}
	if supervisorID != nil {
		sup, err := st.LockUser(ctx, *supervisorID)
		if err != nil {
			return 0, fmt.Errorf("locking supervisor: %w", err)
		}
		if sup == nil || !sup.active() || !managedSource(sup.AuthSource) || userrules.IsPrivileged(sup.Role) {
			return 0, &userrules.ReferenceError{Field: "supervisor_id", Message: "tidak ditemukan"}
		}
	}
	return *roleID, nil
}

// uniqueOnUpdate re-checks only values that changed (same as the admin API).
func uniqueOnUpdate(ctx context.Context, st Store, before *User, email, employeeID string) error {
	if email != before.Email {
		id, err := st.FindIDByEmail(ctx, email, &before.ID)
		if err := userrules.Uniqueness("email", id, err); err != nil {
			return err
		}
	}
	if employeeID != "" && (before.EmployeeID == nil || employeeID != *before.EmployeeID) {
		id, err := st.FindIDByEmployeeID(ctx, employeeID, &before.ID)
		if err := userrules.Uniqueness("employee_id", id, err); err != nil {
			return err
		}
	}
	return nil
}

func mustGet(ctx context.Context, st Store, id int64) (*User, error) {
	u, err := st.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading user: %w", err)
	}
	if u == nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func writeAudit(ctx context.Context, st Store, a Actor, action string, id int64, before, after *User) error {
	e := audit.Entry{ActorID: a.ServiceUserID, Action: action, EntityType: "user", EntityID: id, IP: a.IP}
	// A typed nil *User in an `any` field would marshal as JSON null, not SQL NULL.
	if before != nil {
		e.Before = before
	}
	if after != nil {
		e.After = after
	}
	if err := st.WriteAudit(ctx, e); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

// mapWriteError turns a 23505 the pre-check missed (race) into a 409.
func mapWriteError(op string, err error) error {
	if errors.Is(err, ErrUserNotFound) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == userrules.PgUniqueViolation {
		return &userrules.ConflictError{Field: userrules.ConflictFieldFromConstraint(pgErr.ConstraintName), Message: "sudah digunakan"}
	}
	return fmt.Errorf("%s: %w", op, err)
}
