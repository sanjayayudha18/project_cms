//go:build integration

package pgstore_test

// Task 6.3 (ext-user-api): extuser.Service over pgstore against a real DB.
// Property 5 (one audit row per change, actor = service user; audit failure
// rolls back), FOR UPDATE serialising a concurrent role change, and a
// disabled user being closed to login + refresh like an admin disable.
//
// Rows really commit (TxStore runs on the pool), so every row is created under
// a per-run "it63-<hex>-" username prefix and removed in t.Cleanup, audit rows
// first (audit_logs.actor_id FK), like backend/internal/service integration tests.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-extapi/internal/audit"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser/pgstore"
	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

const testIP = "203.0.113.7"

type env struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	prefix string
	actor  extuser.Actor
	svc    *extuser.Service
}

func setup(t *testing.T) *env {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	prefix := "it63-" + hex.EncodeToString(b) + "-"
	t.Cleanup(func() {
		c := context.Background()
		like := prefix + "%"
		if _, err := pool.Exec(c, `DELETE FROM audit_logs
			WHERE actor_id IN (SELECT id FROM users WHERE username LIKE $1)
			   OR (entity_type = 'user' AND entity_id IN (SELECT id FROM users WHERE username LIKE $1))`, like); err != nil {
			t.Errorf("cleanup audit_logs: %v", err)
		}
		if _, err := pool.Exec(c, `UPDATE users SET supervisor_id = NULL WHERE username LIKE $1`, like); err != nil {
			t.Errorf("cleanup supervisor refs: %v", err)
		}
		if _, err := pool.Exec(c, `DELETE FROM users WHERE username LIKE $1`, like); err != nil {
			t.Errorf("cleanup users: %v", err)
		}
		pool.Close()
	})

	svcUser, err := repository.New(pool).CreateServiceUser(ctx, repository.CreateServiceUserParams{
		Username: prefix + "svc", FullName: "it63 service", Email: prefix + "svc@example.test",
	})
	if err != nil {
		t.Fatalf("service user (migration 039 applied?): %v", err)
	}
	return &env{
		ctx: ctx, pool: pool, prefix: prefix,
		actor: extuser.Actor{ServiceUserID: svcUser, ClientID: "crown_it63", AssignableRoles: []string{"ATM-USER", "ATM-SPV"}, IP: testIP},
		svc:   extuser.NewService(pgstore.NewTxStore(pool), pgstore.NewReader(pool)),
	}
}

func (e *env) createReq(name string) extuser.CreateRequest {
	return extuser.CreateRequest{
		Username: e.prefix + name, FullName: "IT63 " + name, Email: e.prefix + name + "@example.test",
		Role: "ATM-USER", EmployeeID: e.prefix + "emp-" + name,
	}
}

type auditRow struct {
	actorID       int64
	action, ip    string
	before, after bool
}

func (e *env) audits(t *testing.T, userID int64) []auditRow {
	t.Helper()
	rows, err := e.pool.Query(e.ctx, `SELECT actor_id, action, COALESCE(host(ip::inet), ''), before IS NOT NULL, after IS NOT NULL
		FROM audit_logs WHERE entity_type = 'user' AND entity_id = $1 ORDER BY id`, userID)
	if err != nil {
		t.Fatalf("audits: %v", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (auditRow, error) {
		var a auditRow
		err := r.Scan(&a.actorID, &a.action, &a.ip, &a.before, &a.after)
		return a, err
	})
	if err != nil {
		t.Fatalf("audits scan: %v", err)
	}
	return out
}

// expectNewAudit asserts exactly one new audit row since `seen`, with the
// service user as actor, the caller's IP and the given action.
func (e *env) expectNewAudit(t *testing.T, userID int64, seen int, action string, wantBefore bool) int {
	t.Helper()
	got := e.audits(t, userID)
	if len(got) != seen+1 {
		t.Fatalf("%s: audit rows = %d, want %d", action, len(got), seen+1)
	}
	r := got[seen]
	if r.actorID != e.actor.ServiceUserID || r.action != action || r.ip != testIP || r.before != wantBefore || !r.after {
		t.Errorf("%s: audit = %+v, want actor %d ip %s before=%v after=true", action, r, e.actor.ServiceUserID, testIP, wantBefore)
	}
	return seen + 1
}

// loginAndRefreshOpen mirrors the lookups backend uses (backend-extapi may not
// import backend): login = FindUserByEmail + is_active check
// (backend/internal/auth/service.go), refresh = GetUserProfile; both filter
// deleted_at IS NULL (backend/queries/auth.sql).
func (e *env) loginAndRefreshOpen(t *testing.T, id int64, email string) (login, refresh bool) {
	t.Helper()
	if err := e.pool.QueryRow(e.ctx, `SELECT EXISTS (SELECT 1 FROM users u WHERE u.email = $1 AND u.deleted_at IS NULL AND u.is_active)`, email).Scan(&login); err != nil {
		t.Fatalf("login lookup: %v", err)
	}
	if err := e.pool.QueryRow(e.ctx, `SELECT EXISTS (SELECT 1 FROM users u WHERE u.id = $1 AND u.deleted_at IS NULL)`, id).Scan(&refresh); err != nil {
		t.Fatalf("refresh lookup: %v", err)
	}
	return login, refresh
}

func TestService_EachChangeWritesOneAuditAsServiceUser(t *testing.T) {
	e := setup(t)

	sup, err := e.svc.Create(e.ctx, e.actor, e.createReq("sup"))
	if err != nil {
		t.Fatalf("create supervisor: %v", err)
	}
	req := e.createReq("u1")
	req.SupervisorID = &sup.ID
	u, err := e.svc.Create(e.ctx, e.actor, req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.AuthSource != userrules.AuthSourceLDAP || !u.IsActive || u.Role != "ATM-USER" || u.SupervisorID == nil || *u.SupervisorID != sup.ID {
		t.Errorf("created user = %+v", u)
	}
	n := e.expectNewAudit(t, u.ID, 0, "user_created", false)

	// Duplicate username → conflict, no extra audit.
	if _, err := e.svc.Create(e.ctx, e.actor, req); !errors.As(err, new(*userrules.ConflictError)) {
		t.Errorf("duplicate create: err = %v, want ConflictError", err)
	}

	lvl := int32(2)
	newEmail := e.prefix + "u1-new@example.test"
	upd, err := e.svc.Update(e.ctx, e.actor, u.ID, extuser.UpdateRequest{
		FullName: "IT63 u1 renamed", Email: newEmail, Role: "ATM-SPV",
		EmployeeID: e.prefix + "emp-u1-new", ApprovalLevel: &lvl,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Role != "ATM-SPV" || upd.FullName != "IT63 u1 renamed" || upd.SupervisorID != nil || upd.ApprovalLevel == nil || *upd.ApprovalLevel != 2 {
		t.Errorf("updated user = %+v", upd)
	}
	n = e.expectNewAudit(t, u.ID, n, "user_updated", true)
	if _, err := e.svc.Create(e.ctx, e.actor, extuser.CreateRequest{
		Username: e.prefix + "dup-mail", FullName: "dup", Email: newEmail, Role: "ATM-USER",
	}); !errors.As(err, new(*userrules.ConflictError)) {
		t.Errorf("create with taken email: err = %v, want ConflictError", err)
	}

	if login, refresh := e.loginAndRefreshOpen(t, u.ID, newEmail); !login || !refresh {
		t.Fatalf("active user: login=%v refresh=%v, want both open", login, refresh)
	}
	if err := e.svc.Disable(e.ctx, e.actor, u.ID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	n = e.expectNewAudit(t, u.ID, n, "user_deactivated", true)
	// Same row state as the admin disable (backend DeactivateUser).
	if s := e.snap(t, u.ID); s.isActive || !s.deleted {
		t.Errorf("disabled row = %+v, want is_active=false and deleted_at set", s)
	}
	if login, refresh := e.loginAndRefreshOpen(t, u.ID, newEmail); login || refresh {
		t.Errorf("disabled user: login=%v refresh=%v, want both closed", login, refresh)
	}
	if err := e.svc.Disable(e.ctx, e.actor, u.ID); err != nil {
		t.Fatalf("disable again: %v", err)
	}
	if got := len(e.audits(t, u.ID)); got != n {
		t.Errorf("second disable wrote audit: rows = %d, want %d", got, n)
	}
	// Disabled users stay readable (Req 8.1).
	if g, err := e.svc.Get(e.ctx, u.ID); err != nil || g.IsActive || g.DeletedAt == nil {
		t.Errorf("get disabled = %+v, %v", g, err)
	}

	en, err := e.svc.Enable(e.ctx, e.actor, u.ID)
	if err != nil || !en.IsActive || en.DeletedAt != nil {
		t.Fatalf("enable = %+v, %v", en, err)
	}
	n = e.expectNewAudit(t, u.ID, n, "user_reactivated", true)
	if login, refresh := e.loginAndRefreshOpen(t, u.ID, newEmail); !login || !refresh {
		t.Errorf("re-enabled user: login=%v refresh=%v, want both open", login, refresh)
	}
	if _, err := e.svc.Enable(e.ctx, e.actor, u.ID); err != nil {
		t.Fatalf("enable again: %v", err)
	}
	if got := len(e.audits(t, u.ID)); got != n {
		t.Errorf("second enable wrote audit: rows = %d, want %d", got, n)
	}

	if g, err := e.svc.GetByUsername(e.ctx, " "+req.Username+" "); err != nil || g.ID != u.ID {
		t.Errorf("get by username = %+v, %v", g, err)
	}
	if _, err := e.svc.GetByUsername(e.ctx, e.prefix+"nobody"); !errors.Is(err, extuser.ErrUserNotFound) {
		t.Errorf("get unknown username: err = %v, want ErrUserNotFound", err)
	}
	if _, err := e.svc.Get(e.ctx, -1); !errors.Is(err, extuser.ErrUserNotFound) {
		t.Errorf("get missing id: err = %v, want ErrUserNotFound", err)
	}
	if err := e.svc.Disable(e.ctx, e.actor, -1); !errors.Is(err, extuser.ErrUserNotFound) {
		t.Errorf("disable missing id: err = %v, want ErrUserNotFound", err)
	}
	// The service account itself is not a Managed_User.
	if _, err := e.svc.Get(e.ctx, e.actor.ServiceUserID); !errors.Is(err, extuser.ErrUserNotFound) {
		t.Errorf("get service account: err = %v, want ErrUserNotFound", err)
	}
}

// failAuditTx wraps the real TxStore: every write runs for real, then
// WriteAudit fails, so the whole transaction must roll back.
type failAuditTx struct{ inner extuser.Tx }

func (f failAuditTx) InTx(ctx context.Context, fn func(extuser.Store) error) error {
	return f.inner.InTx(ctx, func(s extuser.Store) error { return fn(failAudit{s}) })
}

type failAudit struct{ extuser.Store }

var errForced = errors.New("forced audit failure")

func (failAudit) WriteAudit(context.Context, audit.Entry) error { return errForced }

type snapshot struct {
	fullName, email, role string
	isActive, deleted     bool
	updatedAt             time.Time
}

func (e *env) snap(t *testing.T, id int64) snapshot {
	t.Helper()
	var s snapshot
	if err := e.pool.QueryRow(e.ctx, `SELECT u.full_name, u.email, r.role, u.is_active, u.deleted_at IS NOT NULL, u.updated_at
		FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1`, id).
		Scan(&s.fullName, &s.email, &s.role, &s.isActive, &s.deleted, &s.updatedAt); err != nil {
		t.Fatalf("snapshot %d: %v", id, err)
	}
	return s
}

func TestService_AuditFailureRollsBackUserChange(t *testing.T) {
	e := setup(t)
	failing := extuser.NewService(failAuditTx{pgstore.NewTxStore(e.pool)}, pgstore.NewReader(e.pool))

	// Create: no user row is left behind.
	req := e.createReq("rb-new")
	if _, err := failing.Create(e.ctx, e.actor, req); !errors.Is(err, errForced) {
		t.Fatalf("create: err = %v, want forced", err)
	}
	var count int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM users WHERE username = $1`, req.Username).Scan(&count); err != nil || count != 0 {
		t.Errorf("rolled-back create left %d rows (%v)", count, err)
	}

	u, err := e.svc.Create(e.ctx, e.actor, e.createReq("rb"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	before := e.snap(t, u.ID)
	audits := len(e.audits(t, u.ID))

	if _, err := failing.Update(e.ctx, e.actor, u.ID, extuser.UpdateRequest{
		FullName: "changed", Email: e.prefix + "changed@example.test", Role: "ATM-SPV",
	}); !errors.Is(err, errForced) {
		t.Errorf("update: err = %v, want forced", err)
	}
	if after := e.snap(t, u.ID); after != before {
		t.Errorf("update: users row changed despite audit failure: %+v -> %+v", before, after)
	}
	if err := failing.Disable(e.ctx, e.actor, u.ID); !errors.Is(err, errForced) {
		t.Errorf("disable: err = %v, want forced", err)
	}
	if after := e.snap(t, u.ID); after != before {
		t.Errorf("disable: users row changed despite audit failure: %+v -> %+v", before, after)
	}

	// Enable from disabled: disable for real, then a failing enable keeps it disabled.
	if err := e.svc.Disable(e.ctx, e.actor, u.ID); err != nil {
		t.Fatalf("disable: %v", err)
	}
	disabled := e.snap(t, u.ID)
	if _, err := failing.Enable(e.ctx, e.actor, u.ID); !errors.Is(err, errForced) {
		t.Errorf("enable: err = %v, want forced", err)
	}
	if after := e.snap(t, u.ID); after != disabled {
		t.Errorf("enable: users row changed despite audit failure: %+v -> %+v", disabled, after)
	}
	if got := len(e.audits(t, u.ID)); got != audits+1 { // only the real disable
		t.Errorf("audit rows = %d, want %d", got, audits+1)
	}
}

// A role change committed by another transaction while the service waits on
// FOR UPDATE is seen by the service: promoting the user to a privileged role
// makes the blocked update fail with ErrRoleNotAllowed instead of overwriting it.
func TestService_ForUpdateSerialisesConcurrentRoleChange(t *testing.T) {
	e := setup(t)
	u, err := e.svc.Create(e.ctx, e.actor, e.createReq("lock"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	audits := len(e.audits(t, u.ID))

	tx, err := e.pool.Begin(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(e.ctx) }()
	var holderPID int
	if err := tx.QueryRow(e.ctx, `SELECT pg_backend_pid() FROM users WHERE id = $1 FOR UPDATE`, u.ID).Scan(&holderPID); err != nil {
		t.Fatalf("lock: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := e.svc.Update(e.ctx, e.actor, u.ID, extuser.UpdateRequest{
			FullName: "should not apply", Email: e.prefix + "lock@example.test", Role: "ATM-SPV",
		})
		done <- err
	}()

	// Wait until some backend is blocked by our transaction (the service's LockUser).
	deadline := time.Now().Add(10 * time.Second)
	for blocked := false; !blocked; {
		if err := e.pool.QueryRow(e.ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY (pg_blocking_pids(pid)))`, holderPID).Scan(&blocked); err != nil {
			t.Fatalf("pg_blocking_pids: %v", err)
		}
		select {
		case err := <-done:
			t.Fatalf("update finished while the row was locked: %v", err)
		default:
		}
		if !blocked && time.Now().After(deadline) {
			t.Fatal("service update never blocked on the row lock")
		}
		if !blocked {
			time.Sleep(20 * time.Millisecond)
		}
	}

	if _, err := tx.Exec(e.ctx, `UPDATE users SET role_id = (SELECT id FROM roles WHERE role = 'ADMIN') WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if err := tx.Commit(e.ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, extuser.ErrRoleNotAllowed) {
			t.Errorf("blocked update: err = %v, want ErrRoleNotAllowed", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("update still blocked after the lock was released")
	}
	if s := e.snap(t, u.ID); s.role != "ADMIN" || s.fullName == "should not apply" {
		t.Errorf("row after race = %+v, want role ADMIN and the old name", s)
	}
	if got := len(e.audits(t, u.ID)); got != audits {
		t.Errorf("audit rows = %d, want %d", got, audits)
	}
}
