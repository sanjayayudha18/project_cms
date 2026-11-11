package extuser

import (
	"maps"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/pkg/userrules"
)

// privilegedRole draws a privileged role with random case and padding.
func privilegedRole(t *rapid.T) string {
	r := rapid.SampledFrom(userrules.PrivilegedRoles).Draw(t, "role")
	if rapid.Bool().Draw(t, "lower") {
		r = strings.ToLower(r)
	}
	return rapid.SampledFrom([]string{"", " ", "  "}).Draw(t, "pad") + r
}

// forbiddenRole: privileged, or a role outside the client's allowlist.
func forbiddenRole(t *rapid.T) string {
	if rapid.Bool().Draw(t, "privileged") {
		return privilegedRole(t)
	}
	return rapid.SampledFrom([]string{"ACM-USER", "VENDOR-USER", "BRANCH-ATM-SPV", "X"}).Draw(t, "outside")
}

// Property 2: a privileged or non-allowlisted role is never assigned.
func TestProperty2_PrivilegedRoleNeverAssigned(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := seeded()
		before := maps.Clone(d.users)
		role := forbiddenRole(t)
		svc := newSvc(d)
		req := validCreate()
		req.Role = role
		_, errC := svc.Create(ctx, testActor, req)
		up := validUpdate()
		up.Role = role
		_, errU := svc.Update(ctx, testActor, idManaged, up)
		if !errorsIs(errC, ErrRoleNotAllowed) || !errorsIs(errU, ErrRoleNotAllowed) {
			t.Fatalf("role %q: create=%v update=%v; want ErrRoleNotAllowed", role, errC, errU)
		}
		if d.writes() != 0 || d.commits != 0 || !maps.EqualFunc(d.users, before, usersEqual) {
			t.Fatalf("role %q: writes=%d commits=%d; want nothing written", role, d.writes(), d.commits)
		}
	})
}

// Property 3: users holding a privileged role are never touched or revealed.
func TestProperty3_PrivilegedUsersUntouched(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := seeded()
		const id int64 = 77
		d.add(User{ID: id, Username: "p", FullName: "P", Email: "p@x.id", Role: privilegedRole(t), IsActive: true})
		before := maps.Clone(d.users)
		svc := newSvc(d)
		_, errU := svc.Update(ctx, testActor, id, validUpdate())
		errD := svc.Disable(ctx, testActor, id)
		_, errE := svc.Enable(ctx, testActor, id)
		_, errG := svc.Get(ctx, id)
		_, errN := svc.GetByUsername(ctx, "p")
		for _, err := range []error{errU, errD, errE} {
			if !errorsIs(err, ErrRoleNotAllowed) {
				t.Fatalf("write err = %v; want ErrRoleNotAllowed", err)
			}
		}
		if !errorsIs(errG, ErrUserNotFound) || !errorsIs(errN, ErrUserNotFound) {
			t.Fatalf("read errs = %v, %v; want ErrUserNotFound", errG, errN)
		}
		if d.writes() != 0 || !maps.EqualFunc(d.users, before, usersEqual) {
			t.Fatalf("privileged user changed")
		}
	})
}

// Property 10: final state = last call; audits = real state changes.
func TestProperty10_DisableEnableIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := seeded()
		id := rapid.SampledFrom([]int64{idManaged, idDisabled}).Draw(t, "start")
		ops := rapid.SliceOfN(rapid.Bool(), 1, 12).Draw(t, "enableOps")
		svc := newSvc(d)
		active := d.users[id].IsActive
		changes := 0
		for _, enable := range ops {
			var err error
			if enable {
				_, err = svc.Enable(ctx, testActor, id)
			} else {
				err = svc.Disable(ctx, testActor, id)
			}
			if err != nil {
				t.Fatalf("op enable=%v: %v", enable, err)
			}
			if enable != active {
				changes++
				active = enable
			}
		}
		u := d.users[id]
		last := ops[len(ops)-1]
		if u.IsActive != last || (u.DeletedAt == nil) != last {
			t.Fatalf("final %+v; want active=%v", u, last)
		}
		if len(d.audits) != changes {
			t.Fatalf("audits = %d; want %d", len(d.audits), changes)
		}
	})
}

// Property 5 (rollback half): a failed audit write commits nothing.
func TestProperty5_AuditFailureRollsBack(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := seeded()
		d.hook = failOn("WriteAudit", 1, errBoom)
		before := maps.Clone(d.users)
		svc := newSvc(d)
		var err error
		switch rapid.IntRange(0, 3).Draw(t, "op") {
		case 0:
			_, err = svc.Create(ctx, testActor, validCreate())
		case 1:
			_, err = svc.Update(ctx, testActor, idManaged, validUpdate())
		case 2:
			err = svc.Disable(ctx, testActor, idManaged)
		case 3:
			_, err = svc.Enable(ctx, testActor, idDisabled)
		}
		if !errorsIs(err, errBoom) || d.rollbacks != 1 || d.commits != 0 {
			t.Fatalf("err=%v rollbacks=%d commits=%d; want audit error, 1 rollback, 0 commits", err, d.rollbacks, d.commits)
		}
		if len(d.audits) != 0 || !maps.EqualFunc(d.users, before, usersEqual) {
			t.Fatalf("state changed after rollback")
		}
	})
}
