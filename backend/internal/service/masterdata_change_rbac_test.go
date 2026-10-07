package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/pkg/middleware"
)

// adminCtx is a request context as RequireAuth would build it for an ADMIN
// user with the given id.
func adminCtx(userID int64) context.Context {
	return ctxWithRole(userID, "ADMIN")
}

func ctxWithRole(userID int64, role string) context.Context {
	return middleware.WithAuthContext(context.Background(), &middleware.AuthContext{UserID: userID, Username: "u", Role: role})
}

// The service-layer recheck (plan.md T3.6): even if a route is mounted without
// RequireRoles, or a new caller appears, Submit must refuse -- and must refuse
// before touching the repo, orchestrator or audit log.
func TestMasterDataChangeService_Submit_ServiceLayerRBAC(t *testing.T) {
	req := SubmitRequest{EntityType: "vendor", Op: "create", Payload: map[string]string{"code": "VEN1"}}

	cases := []struct {
		name string
		ctx  context.Context
		want error // nil = allowed
	}{
		{"ADMIN allowed", ctxWithRole(7, "ADMIN"), nil},
		{"ADMIN_PARAM allowed", ctxWithRole(7, "ADMIN_PARAM"), nil},
		{"role match is case/space-insensitive like RequireRoles", ctxWithRole(7, " admin_param "), nil},
		{"no auth context", context.Background(), ErrMasterDataForbidden},
		{"nil auth context", middleware.WithAuthContext(context.Background(), nil), ErrMasterDataForbidden},
		{"vendor role denied", ctxWithRole(7, "VENDOR"), ErrMasterDataForbidden},
		{"other internal role denied", ctxWithRole(7, "ATM-USER"), ErrMasterDataForbidden},
		{"APPACCESS is not a master-data role", ctxWithRole(7, "APPACCESS"), ErrMasterDataForbidden},
		{"empty role denied", ctxWithRole(7, ""), ErrMasterDataForbidden},
		{"maker id must be the authenticated caller", ctxWithRole(8, "ADMIN"), ErrMasterDataForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeMasterDataChangeRepo{}
			orch := &fakeMasterDataOrchestrator{}
			auditWriter := &fakeMasterDataAuditWriter{}
			svc := NewMasterDataChangeService(repo, orch, auditWriter)

			_, err := svc.Submit(tc.ctx, 7, req, "127.0.0.1")

			if tc.want == nil {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
			if repo.createCalled || repo.findPendingCalled || orch.called || len(auditWriter.entries) != 0 {
				t.Errorf("a forbidden submit must have no side effects: create=%v findPending=%v orch=%v audits=%d",
					repo.createCalled, repo.findPendingCalled, orch.called, len(auditWriter.entries))
			}
		})
	}
}
