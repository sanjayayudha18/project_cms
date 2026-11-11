package extuser

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cimb-niaga/cms/backend-extapi/internal/audit"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

// --- fake Tx / Store / Reader ------------------------------------------------

// errReturnNil makes a lookup return (nil, nil) instead of an error.
var errReturnNil = errors.New("return nil")

var errBoom = errors.New("boom")

// fakeDB is an in-memory users + audit_logs store. InTx snapshots the state
// and restores it when fn fails, so a failed request leaves no trace.
type fakeDB struct {
	users  map[int64]User
	roles  map[string]int64
	audits []audit.Entry
	nextID int64

	// hook is called before every Store/Reader method with the method name and
	// its 1-based call count; a non-nil error is returned by that method.
	hook  func(method string, n int) error
	calls map[string]int

	txCalls, commits, rollbacks int
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		users:  map[int64]User{},
		roles:  map[string]int64{"ATM-USER": 10, "ATM-SPV": 11, "ADMIN": 1, "APPACCESS": 2, "ADMIN_PARAM": 3, "EXT-CLIENT": 4},
		nextID: 100,
		calls:  map[string]int{},
	}
}

func (d *fakeDB) add(u User) User {
	if u.AuthSource == "" {
		u.AuthSource = userrules.AuthSourceLDAP
	}
	d.users[u.ID] = u
	return u
}

func (d *fakeDB) call(method string) error {
	d.calls[method]++
	if d.hook == nil {
		return nil
	}
	return d.hook(method, d.calls[method])
}

func (d *fakeDB) writes() int {
	return d.calls["Create"] + d.calls["Update"] + d.calls["Deactivate"] + d.calls["Reactivate"] + d.calls["WriteAudit"]
}

func (d *fakeDB) InTx(ctx context.Context, fn func(s Store) error) error {
	d.txCalls++
	users, audits, next := maps.Clone(d.users), slices.Clone(d.audits), d.nextID
	if err := fn(d); err != nil {
		d.users, d.audits, d.nextID = users, audits, next
		d.rollbacks++
		return err
	}
	d.commits++
	return nil
}

func (d *fakeDB) LockUser(_ context.Context, id int64) (*LockedUser, error) {
	if err := d.call("LockUser"); err != nil {
		return nil, err
	}
	u, ok := d.users[id]
	if !ok {
		return nil, nil
	}
	return &LockedUser{ID: u.ID, AuthSource: u.AuthSource, Role: u.Role, IsActive: u.IsActive, Deleted: u.DeletedAt != nil}, nil
}

func (d *fakeDB) getUser(method string, id int64) (*User, error) {
	if err := d.call(method); err != nil {
		if errors.Is(err, errReturnNil) {
			return nil, nil
		}
		return nil, err
	}
	u, ok := d.users[id]
	if !ok {
		return nil, nil
	}
	return &u, nil
}

func (d *fakeDB) GetUser(_ context.Context, id int64) (*User, error) { return d.getUser("GetUser", id) }

func (d *fakeDB) findID(method string, match func(User) bool, excludeID *int64) (*int64, error) {
	if err := d.call(method); err != nil {
		return nil, err
	}
	for id, u := range d.users {
		if excludeID != nil && id == *excludeID {
			continue
		}
		if match(u) {
			return &id, nil
		}
	}
	return nil, nil
}

func (d *fakeDB) FindIDByUsername(_ context.Context, username string) (*int64, error) {
	return d.findID("FindIDByUsername", func(u User) bool { return u.Username == username }, nil)
}

func (d *fakeDB) FindIDByEmail(_ context.Context, email string, excludeID *int64) (*int64, error) {
	return d.findID("FindIDByEmail", func(u User) bool { return u.Email == email }, excludeID)
}

func (d *fakeDB) FindIDByEmployeeID(_ context.Context, employeeID string, excludeID *int64) (*int64, error) {
	return d.findID("FindIDByEmployeeID", func(u User) bool { return u.EmployeeID != nil && *u.EmployeeID == employeeID }, excludeID)
}

func (d *fakeDB) RoleIDByName(_ context.Context, role string) (*int64, error) {
	if err := d.call("RoleIDByName"); err != nil {
		return nil, err
	}
	id, ok := d.roles[role]
	if !ok {
		return nil, nil
	}
	return &id, nil
}

func (d *fakeDB) roleName(id int64) string {
	for name, rid := range d.roles {
		if rid == id {
			return name
		}
	}
	return ""
}

func (d *fakeDB) Create(_ context.Context, p CreateParams) (int64, error) {
	if err := d.call("Create"); err != nil {
		return 0, err
	}
	d.nextID++
	d.users[d.nextID] = User{
		ID: d.nextID, Username: p.Username, FullName: p.FullName, Email: p.Email, Role: d.roleName(p.RoleID),
		EmployeeID: p.EmployeeID, SupervisorID: p.SupervisorID, ApprovalLevel: p.ApprovalLevel,
		IsActive: true, AuthSource: userrules.AuthSourceLDAP,
	}
	return d.nextID, nil
}

func (d *fakeDB) Update(_ context.Context, p UpdateParams) error {
	if err := d.call("Update"); err != nil {
		return err
	}
	u := d.users[p.ID]
	u.Role, u.FullName, u.Email = d.roleName(p.RoleID), p.FullName, p.Email
	u.EmployeeID, u.SupervisorID, u.ApprovalLevel = p.EmployeeID, p.SupervisorID, p.ApprovalLevel
	d.users[p.ID] = u
	return nil
}

func (d *fakeDB) Deactivate(_ context.Context, id int64) error {
	if err := d.call("Deactivate"); err != nil {
		return err
	}
	u := d.users[id]
	now := time.Now()
	u.IsActive, u.DeletedAt = false, &now
	d.users[id] = u
	return nil
}

func (d *fakeDB) Reactivate(_ context.Context, id int64) error {
	if err := d.call("Reactivate"); err != nil {
		return err
	}
	u := d.users[id]
	u.IsActive, u.DeletedAt = true, nil
	d.users[id] = u
	return nil
}

func (d *fakeDB) WriteAudit(_ context.Context, e audit.Entry) error {
	if err := d.call("WriteAudit"); err != nil {
		return err
	}
	d.audits = append(d.audits, e)
	return nil
}

// fakeReader serves Service.Get / GetByUsername from the same fakeDB.
type fakeReader struct{ *fakeDB }

func (r fakeReader) GetUser(_ context.Context, id int64) (*User, error) {
	return r.getUser("ReaderGetUser", id)
}

func (r fakeReader) GetUserByUsername(_ context.Context, username string) (*User, error) {
	if err := r.call("ReaderGetUserByUsername"); err != nil {
		return nil, err
	}
	for _, u := range r.users {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, nil
}

// failOn fails the n-th call of method with err.
func failOn(method string, n int, err error) func(string, int) error {
	return func(m string, c int) error {
		if m == method && c == n {
			return err
		}
		return nil
	}
}

// --- fixtures ----------------------------------------------------------------

var testActor = Actor{ServiceUserID: 7, ClientID: "idm", AssignableRoles: []string{"ATM-USER", "ATM-SPV", "ADMIN", "GHOST"}, IP: "10.0.0.9"}

const (
	idManaged  int64 = 1
	idDisabled int64 = 2
	idAdmin    int64 = 3
	idVendor   int64 = 4
	idService  int64 = 5
	idSup      int64 = 6
)

func ptr[T any](v T) *T { return &v }

// seeded returns a fakeDB with one user per kind of target.
func seeded() *fakeDB {
	d := newFakeDB()
	past := time.Now().Add(-time.Hour)
	d.add(User{ID: idManaged, Username: "andi", FullName: "Andi", Email: "andi@x.id", Role: "ATM-USER", EmployeeID: ptr("E1"), IsActive: true})
	d.add(User{ID: idDisabled, Username: "budi", FullName: "Budi", Email: "budi@x.id", Role: "ATM-USER", DeletedAt: &past})
	d.add(User{ID: idAdmin, Username: "root", FullName: "Root", Email: "root@x.id", Role: "ADMIN", IsActive: true})
	d.add(User{ID: idVendor, Username: "vend", FullName: "Vend", Email: "vend@x.id", Role: "VENDOR-USER", IsActive: true, AuthSource: "local"})
	d.add(User{ID: idService, Username: "svc", FullName: "Svc", Email: "svc@x.id", Role: userrules.ServiceRole, IsActive: true, AuthSource: userrules.AuthSourceService})
	d.add(User{ID: idSup, Username: "sup", FullName: "Sup", Email: "sup@x.id", Role: "ATM-SPV", EmployeeID: ptr("E6"), IsActive: true})
	return d
}

func newSvc(d *fakeDB) *Service { return NewService(d, fakeReader{d}) }

func validCreate() CreateRequest {
	return CreateRequest{Username: " citra ", FullName: " Citra ", Email: " citra@x.id ", Role: " ATM-USER ", EmployeeID: " E9 "}
}

func validUpdate() UpdateRequest {
	return UpdateRequest{FullName: "Andi B", Email: "andi@x.id", Role: "ATM-SPV", EmployeeID: "E1"}
}

// --- assertions ---------------------------------------------------------------

func errorsIs(err, target error) bool { return errors.Is(err, target) }

func wantErrIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v; want %v", err, target)
	}
}

func wantField[E interface {
	*auth.ValidationError | *userrules.ConflictError | *userrules.ReferenceError
}](t *testing.T, err error, field string) {
	t.Helper()
	var e E
	if !errors.As(err, &e) {
		t.Fatalf("err = %v (%T); want %T", err, err, e)
	}
	var got string
	switch v := any(e).(type) {
	case *auth.ValidationError:
		got = v.Field
	case *userrules.ConflictError:
		got = v.Field
	case *userrules.ReferenceError:
		got = v.Field
	}
	if got != field {
		t.Fatalf("field = %q; want %q", got, field)
	}
}

// wantUntouched: the request failed and nothing was committed.
func wantUntouched(t *testing.T, d *fakeDB, before map[int64]User) {
	t.Helper()
	if d.commits != 0 {
		t.Fatalf("commits = %d; want 0", d.commits)
	}
	if len(d.audits) != 0 {
		t.Fatalf("audits = %d; want 0", len(d.audits))
	}
	if !maps.EqualFunc(d.users, before, usersEqual) {
		t.Fatalf("users changed:\n got %+v\nwant %+v", d.users, before)
	}
}

func usersEqual(a, b User) bool {
	return a.ID == b.ID && a.Username == b.Username && a.FullName == b.FullName && a.Email == b.Email &&
		a.Role == b.Role && eqPtr(a.EmployeeID, b.EmployeeID) && eqPtr(a.SupervisorID, b.SupervisorID) &&
		eqPtr(a.ApprovalLevel, b.ApprovalLevel) && a.IsActive == b.IsActive && a.AuthSource == b.AuthSource &&
		(a.DeletedAt == nil) == (b.DeletedAt == nil)
}

func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func wantAudit(t *testing.T, e audit.Entry, action string, id int64, hasBefore bool) {
	t.Helper()
	if e.ActorID != testActor.ServiceUserID || e.IP != testActor.IP || e.Action != action || e.EntityType != "user" || e.EntityID != id {
		t.Fatalf("audit = %+v; want actor %d ip %s action %s user %d", e, testActor.ServiceUserID, testActor.IP, action, id)
	}
	// A typed nil *User would marshal as JSON null instead of SQL NULL.
	if hasBefore != (e.Before != nil) {
		t.Fatalf("audit.Before = %#v; want present=%v", e.Before, hasBefore)
	}
	if e.After == nil {
		t.Fatalf("audit.After is nil")
	}
}

func uniqueViolation(constraint string) error {
	return &pgconn.PgError{Code: userrules.PgUniqueViolation, ConstraintName: constraint}
}

var ctx = context.Background()

// --- Create -------------------------------------------------------------------

func TestCreate_Success(t *testing.T) {
	d := seeded()
	got, err := newSvc(d).Create(ctx, testActor, CreateRequest{
		Username: " citra ", FullName: " Citra ", Email: " citra@x.id ", Role: " ATM-USER ",
		EmployeeID: " E9 ", SupervisorID: ptr(idSup), ApprovalLevel: ptr(int32(2)),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := User{ID: got.ID, Username: "citra", FullName: "Citra", Email: "citra@x.id", Role: "ATM-USER",
		EmployeeID: ptr("E9"), SupervisorID: ptr(idSup), ApprovalLevel: ptr(int32(2)), IsActive: true, AuthSource: userrules.AuthSourceLDAP}
	if !usersEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	if d.commits != 1 || len(d.audits) != 1 {
		t.Fatalf("commits=%d audits=%d; want 1,1", d.commits, len(d.audits))
	}
	wantAudit(t, d.audits[0], "user_created", got.ID, false)
}

func TestCreate_EmptyEmployeeIDSkipsLookupAndStoresNull(t *testing.T) {
	d := seeded()
	req := validCreate()
	req.EmployeeID = "  "
	got, err := newSvc(d).Create(ctx, testActor, req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.EmployeeID != nil || d.calls["FindIDByEmployeeID"] != 0 {
		t.Fatalf("employee_id = %v, lookups = %d; want nil, 0", got.EmployeeID, d.calls["FindIDByEmployeeID"])
	}
}

func TestCreate_RejectedBeforeDB(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*CreateRequest)
		check func(*testing.T, error)
	}{
		{"empty username", func(r *CreateRequest) { r.Username = " " }, func(t *testing.T, err error) { wantField[*auth.ValidationError](t, err, "username") }},
		{"empty full_name", func(r *CreateRequest) { r.FullName = "" }, func(t *testing.T, err error) { wantField[*auth.ValidationError](t, err, "full_name") }},
		{"bad email", func(r *CreateRequest) { r.Email = "nope" }, func(t *testing.T, err error) { wantField[*auth.ValidationError](t, err, "email") }},
		{"empty role", func(r *CreateRequest) { r.Role = "" }, func(t *testing.T, err error) { wantField[*auth.ValidationError](t, err, "role") }},
		{"privileged role in allowlist", func(r *CreateRequest) { r.Role = "ADMIN" }, func(t *testing.T, err error) { wantErrIs(t, err, ErrRoleNotAllowed) }},
		{"role outside allowlist", func(r *CreateRequest) { r.Role = "ACM-USER" }, func(t *testing.T, err error) { wantErrIs(t, err, ErrRoleNotAllowed) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			req := validCreate()
			tt.edit(&req)
			_, err := newSvc(d).Create(ctx, testActor, req)
			tt.check(t, err)
			if d.txCalls != 0 {
				t.Fatalf("InTx called %d times; want 0 (rejected before DB)", d.txCalls)
			}
		})
	}
}

func TestCreate_FailuresRollBack(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*CreateRequest)
		hook  func(string, int) error
		check func(*testing.T, error)
	}{
		{"username taken", func(r *CreateRequest) { r.Username = "andi" }, nil, func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "username") }},
		{"email taken", func(r *CreateRequest) { r.Email = "andi@x.id" }, nil, func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "email") }},
		{"employee_id taken", func(r *CreateRequest) { r.EmployeeID = "E1" }, nil, func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "employee_id") }},
		{"username lookup error", nil, failOn("FindIDByUsername", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"role missing in DB", func(r *CreateRequest) { r.Role = "GHOST" }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "role") }},
		{"role lookup error", nil, failOn("RoleIDByName", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"supervisor missing", func(r *CreateRequest) { r.SupervisorID = ptr(int64(999)) }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "supervisor_id") }},
		{"supervisor disabled", func(r *CreateRequest) { r.SupervisorID = ptr(idDisabled) }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "supervisor_id") }},
		{"supervisor privileged", func(r *CreateRequest) { r.SupervisorID = ptr(idAdmin) }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "supervisor_id") }},
		{"supervisor non-ldap", func(r *CreateRequest) { r.SupervisorID = ptr(idVendor) }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "supervisor_id") }},
		{"supervisor service account", func(r *CreateRequest) { r.SupervisorID = ptr(idService) }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "supervisor_id") }},
		{"supervisor lock error", func(r *CreateRequest) { r.SupervisorID = ptr(idSup) }, failOn("LockUser", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"insert race 23505", nil, failOn("Create", 1, uniqueViolation("users_email_key")), func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "email") }},
		{"insert 23505 unknown constraint", nil, failOn("Create", 1, uniqueViolation("other")), func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "unknown") }},
		{"insert error", nil, failOn("Create", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"reload error", nil, failOn("GetUser", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"reload missing", nil, failOn("GetUser", 1, errReturnNil), func(t *testing.T, err error) { wantErrIs(t, err, ErrUserNotFound) }},
		{"audit error", nil, failOn("WriteAudit", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			d.hook = tt.hook
			before := maps.Clone(d.users)
			req := validCreate()
			if tt.edit != nil {
				tt.edit(&req)
			}
			_, err := newSvc(d).Create(ctx, testActor, req)
			tt.check(t, err)
			wantUntouched(t, d, before)
		})
	}
}

// --- Update -------------------------------------------------------------------

func TestUpdate_Success(t *testing.T) {
	d := seeded()
	req := UpdateRequest{FullName: " Andi B ", Email: " andi.b@x.id ", Role: " ATM-SPV ", EmployeeID: " E2 ", SupervisorID: ptr(idSup), ApprovalLevel: ptr(int32(1))}
	got, err := newSvc(d).Update(ctx, testActor, idManaged, req)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	want := User{ID: idManaged, Username: "andi", FullName: "Andi B", Email: "andi.b@x.id", Role: "ATM-SPV",
		EmployeeID: ptr("E2"), SupervisorID: ptr(idSup), ApprovalLevel: ptr(int32(1)), IsActive: true, AuthSource: userrules.AuthSourceLDAP}
	if !usersEqual(got, want) || !usersEqual(d.users[idManaged], want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	if d.commits != 1 || len(d.audits) != 1 {
		t.Fatalf("commits=%d audits=%d; want 1,1", d.commits, len(d.audits))
	}
	wantAudit(t, d.audits[0], "user_updated", idManaged, true)
	if b := d.audits[0].Before.(*User); b.Role != "ATM-USER" || b.Email != "andi@x.id" {
		t.Fatalf("audit.Before = %+v; want the pre-update row", b)
	}
}

func TestUpdate_UniqueChecksOnlyChangedValues(t *testing.T) {
	const idNoEmp int64 = 50
	tests := []struct {
		name             string
		id               int64
		email, empID     string
		wantEmail, wantE int
	}{
		{"nothing changed", idManaged, "andi@x.id", "E1", 0, 0},
		{"employee_id cleared", idManaged, "andi@x.id", "", 0, 0},
		{"email changed", idManaged, "new@x.id", "E1", 1, 0},
		{"employee_id changed", idManaged, "andi@x.id", "E7", 0, 1},
		{"employee_id set from null", idNoEmp, "c@x.id", "E8", 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			d.add(User{ID: idNoEmp, Username: "c", FullName: "C", Email: "c@x.id", Role: "ATM-USER", IsActive: true})
			_, err := newSvc(d).Update(ctx, testActor, tt.id, UpdateRequest{FullName: "X", Email: tt.email, Role: "ATM-USER", EmployeeID: tt.empID})
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if d.calls["FindIDByEmail"] != tt.wantEmail || d.calls["FindIDByEmployeeID"] != tt.wantE {
				t.Fatalf("lookups email=%d employee=%d; want %d,%d", d.calls["FindIDByEmail"], d.calls["FindIDByEmployeeID"], tt.wantEmail, tt.wantE)
			}
		})
	}
}

func TestUpdate_ValidationBeforeDB(t *testing.T) {
	d := seeded()
	req := validUpdate()
	req.Email = "bad"
	_, err := newSvc(d).Update(ctx, testActor, idManaged, req)
	wantField[*auth.ValidationError](t, err, "email")
	if d.txCalls != 0 {
		t.Fatalf("InTx called; want validation before DB")
	}
}

func TestUpdate_FailuresRollBack(t *testing.T) {
	tests := []struct {
		name  string
		id    int64
		edit  func(*UpdateRequest)
		hook  func(string, int) error
		check func(*testing.T, error)
	}{
		{"lock error", idManaged, nil, failOn("LockUser", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"missing", 999, nil, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrUserNotFound) }},
		{"disabled", idDisabled, nil, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrUserNotFound) }},
		{"non-ldap", idVendor, nil, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrUserNotFound) }},
		{"service account", idService, nil, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrUserNotFound) }},
		{"privileged target", idAdmin, nil, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrRoleNotAllowed) }},
		{"privileged new role", idManaged, func(r *UpdateRequest) { r.Role = "admin" }, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrRoleNotAllowed) }},
		{"role outside allowlist", idManaged, func(r *UpdateRequest) { r.Role = "ACM-USER" }, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrRoleNotAllowed) }},
		{"self supervision", idManaged, func(r *UpdateRequest) { r.SupervisorID = ptr(idManaged) }, nil, func(t *testing.T, err error) { wantErrIs(t, err, ErrSelfSupervision) }},
		{"before load error", idManaged, nil, failOn("GetUser", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"email taken", idManaged, func(r *UpdateRequest) { r.Email = "sup@x.id" }, nil, func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "email") }},
		{"employee_id taken", idManaged, func(r *UpdateRequest) { r.EmployeeID = "E6" }, nil, func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "employee_id") }},
		{"role missing in DB", idManaged, func(r *UpdateRequest) { r.Role = "GHOST" }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "role") }},
		{"supervisor disabled", idManaged, func(r *UpdateRequest) { r.SupervisorID = ptr(idDisabled) }, nil, func(t *testing.T, err error) { wantField[*userrules.ReferenceError](t, err, "supervisor_id") }},
		{"update not found", idManaged, nil, failOn("Update", 1, ErrUserNotFound), func(t *testing.T, err error) { wantErrIs(t, err, ErrUserNotFound) }},
		{"update race 23505", idManaged, nil, failOn("Update", 1, uniqueViolation("users_employee_id_key")), func(t *testing.T, err error) { wantField[*userrules.ConflictError](t, err, "employee_id") }},
		{"update error", idManaged, nil, failOn("Update", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"after load error", idManaged, nil, failOn("GetUser", 2, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
		{"audit error", idManaged, nil, failOn("WriteAudit", 1, errBoom), func(t *testing.T, err error) { wantErrIs(t, err, errBoom) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			d.hook = tt.hook
			before := maps.Clone(d.users)
			req := validUpdate()
			if tt.edit != nil {
				tt.edit(&req)
			}
			_, err := newSvc(d).Update(ctx, testActor, tt.id, req)
			tt.check(t, err)
			wantUntouched(t, d, before)
		})
	}
}

// --- Disable / Enable -----------------------------------------------------------

func TestDisable_ActiveUser(t *testing.T) {
	d := seeded()
	if err := newSvc(d).Disable(ctx, testActor, idManaged); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	u := d.users[idManaged]
	if u.IsActive || u.DeletedAt == nil {
		t.Fatalf("user = %+v; want disabled", u)
	}
	if len(d.audits) != 1 {
		t.Fatalf("audits = %d; want 1", len(d.audits))
	}
	wantAudit(t, d.audits[0], "user_deactivated", idManaged, true)
}

func TestDisable_AlreadyDisabledIsNoop(t *testing.T) {
	d := seeded()
	before := maps.Clone(d.users)
	if err := newSvc(d).Disable(ctx, testActor, idDisabled); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if d.writes() != 0 || !maps.EqualFunc(d.users, before, usersEqual) {
		t.Fatalf("writes = %d; want a no-op", d.writes())
	}
}

func TestEnable_DisabledUser(t *testing.T) {
	d := seeded()
	got, err := newSvc(d).Enable(ctx, testActor, idDisabled)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !got.IsActive || got.DeletedAt != nil || !usersEqual(got, d.users[idDisabled]) {
		t.Fatalf("got %+v; want active", got)
	}
	if len(d.audits) != 1 {
		t.Fatalf("audits = %d; want 1", len(d.audits))
	}
	wantAudit(t, d.audits[0], "user_reactivated", idDisabled, true)
}

func TestEnable_AlreadyActiveReturnsUserWithoutWrite(t *testing.T) {
	d := seeded()
	got, err := newSvc(d).Enable(ctx, testActor, idManaged)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !usersEqual(got, d.users[idManaged]) || d.writes() != 0 {
		t.Fatalf("got %+v writes=%d; want current row, no write", got, d.writes())
	}
}

func TestDisableEnable_FailuresRollBack(t *testing.T) {
	type op func(*Service, int64) error
	disable := func(s *Service, id int64) error { return s.Disable(ctx, testActor, id) }
	enable := func(s *Service, id int64) error {
		u, err := s.Enable(ctx, testActor, id)
		if err != nil && u != (User{}) {
			return errors.New("non-zero User on error")
		}
		return err
	}
	tests := []struct {
		name string
		op   op
		id   int64
		hook func(string, int) error
		want error
	}{
		{"disable lock error", disable, idManaged, failOn("LockUser", 1, errBoom), errBoom},
		{"disable missing", disable, 999, nil, ErrUserNotFound},
		{"disable non-ldap", disable, idVendor, nil, ErrUserNotFound},
		{"disable service account", disable, idService, nil, ErrUserNotFound},
		{"disable privileged", disable, idAdmin, nil, ErrRoleNotAllowed},
		{"disable before load error", disable, idManaged, failOn("GetUser", 1, errBoom), errBoom},
		{"disable write error", disable, idManaged, failOn("Deactivate", 1, errBoom), errBoom},
		{"disable after load error", disable, idManaged, failOn("GetUser", 2, errBoom), errBoom},
		{"disable audit error", disable, idManaged, failOn("WriteAudit", 1, errBoom), errBoom},
		{"enable lock error", enable, idDisabled, failOn("LockUser", 1, errBoom), errBoom},
		{"enable missing", enable, 999, nil, ErrUserNotFound},
		{"enable non-ldap", enable, idVendor, nil, ErrUserNotFound},
		{"enable privileged", enable, idAdmin, nil, ErrRoleNotAllowed},
		{"enable write error", enable, idDisabled, failOn("Reactivate", 1, errBoom), errBoom},
		{"enable audit error", enable, idDisabled, failOn("WriteAudit", 1, errBoom), errBoom},
		{"enable active load error", enable, idManaged, failOn("GetUser", 1, errBoom), errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			d.hook = tt.hook
			before := maps.Clone(d.users)
			wantErrIs(t, tt.op(newSvc(d), tt.id), tt.want)
			wantUntouched(t, d, before)
		})
	}
}

// --- Get / GetByUsername -------------------------------------------------------------

func TestGet(t *testing.T) {
	tests := []struct {
		name string
		id   int64
		hook func(string, int) error
		want error
	}{
		{"active managed", idManaged, nil, nil},
		{"disabled managed", idDisabled, nil, nil},
		{"missing", 999, nil, ErrUserNotFound},
		{"privileged", idAdmin, nil, ErrUserNotFound},
		{"non-ldap", idVendor, nil, ErrUserNotFound},
		{"service account", idService, nil, ErrUserNotFound},
		{"read error", idManaged, failOn("ReaderGetUser", 1, errBoom), errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			d.hook = tt.hook
			got, err := newSvc(d).Get(ctx, tt.id)
			if tt.want != nil {
				wantErrIs(t, err, tt.want)
				return
			}
			if err != nil || !usersEqual(got, d.users[tt.id]) {
				t.Fatalf("Get = %+v, %v; want %+v", got, err, d.users[tt.id])
			}
		})
	}
}

func TestGetByUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		hook     func(string, int) error
		want     error
	}{
		{"trimmed match", "  budi ", nil, nil},
		{"missing", "nobody", nil, ErrUserNotFound},
		{"privileged", "root", nil, ErrUserNotFound},
		{"service account", "svc", nil, ErrUserNotFound},
		{"read error", "budi", failOn("ReaderGetUserByUsername", 1, errBoom), errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := seeded()
			d.hook = tt.hook
			got, err := newSvc(d).GetByUsername(ctx, tt.username)
			if tt.want != nil {
				wantErrIs(t, err, tt.want)
				return
			}
			if err != nil || got.ID != idDisabled {
				t.Fatalf("GetByUsername = %+v, %v; want user %d", got, err, idDisabled)
			}
		})
	}
}

func TestActorCanAssign(t *testing.T) {
	for role, want := range map[string]bool{"ATM-USER": true, "ADMIN": false, " admin ": false, "ACM-USER": false, "": false} {
		if got := testActor.CanAssign(role); got != want {
			t.Errorf("CanAssign(%q) = %v; want %v", role, got, want)
		}
	}
}
