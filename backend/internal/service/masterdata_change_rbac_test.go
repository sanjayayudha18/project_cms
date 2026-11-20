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

// ScDD 5.1 AC-5.1-14: cash count entities have their own maker role; every
// other entity type keeps ADMIN/ADMIN_PARAM.
func TestMasterDataChangeService_Submit_MakerRolePerEntityType(t *testing.T) {
	cases := []struct {
		entityType string
		role       string
		allowed    bool
	}{
		// existing entities unchanged
		{"vendor", "ADMIN", true},
		{"vendor", "ADMIN_PARAM", true},
		{"atm", "ADMIN_PARAM", true},
		{"vendor", "CC-CWM", false},
		{"atm", "CC-CCR", false},
		// CC-CWM entities
		{"cash_count_category_params", "CC-CWM", true},
		{"cash_count_area", "cc-cwm", true},
		{"cash_count_pic", "CC-CWM", true},
		{"cash_count_pic", "CC-CCR", false},
		{"cash_count_area", "ADMIN", false},
		{"cash_count_category_params", "ADMIN_PARAM", false},
		// CC-CCR entities
		{"cash_count_anomaly_category", "CC-CCR", true},
		{"cash_count_checklist_item", " CC-CCR ", true},
		{"cash_count_checklist_item", "CC-CWM", false},
		{"cash_count_anomaly_category", "ADMIN", false},
		// other CC roles never make
		{"cash_count_pic", "CC-COORDINATOR", false},
		{"cash_count_checklist_item", "CC-EXECUTOR", false},
		// unlisted cash_count_* type fails closed
		{"cash_count_unknown", "ADMIN", false},
		{"cash_count_unknown", "CC-CWM", false},
	}
	for _, tc := range cases {
		t.Run(tc.entityType+"/"+tc.role, func(t *testing.T) {
			repo := &fakeMasterDataChangeRepo{}
			orch := &fakeMasterDataOrchestrator{}
			auditWriter := &fakeMasterDataAuditWriter{}
			svc := NewMasterDataChangeService(repo, orch, auditWriter)
			req := SubmitRequest{EntityType: tc.entityType, Op: "create", Payload: map[string]string{"code": "X1"}}

			_, err := svc.Submit(ctxWithRole(7, tc.role), 7, req, "127.0.0.1")

			if tc.allowed {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrMasterDataForbidden) {
				t.Fatalf("want ErrMasterDataForbidden, got %v", err)
			}
			if repo.createCalled || orch.called || len(auditWriter.entries) != 0 {
				t.Errorf("a forbidden submit must have no side effects")
			}
		})
	}

	// the entity-less check (RetryApply, import Confirm, PIC roles) keeps the default set
	if err := authorizeMasterDataMaker(ctxWithRole(7, "CC-CWM"), 7); !errors.Is(err, ErrMasterDataForbidden) {
		t.Errorf("authorizeMasterDataMaker(CC-CWM) = %v, want ErrMasterDataForbidden", err)
	}
	if err := authorizeMasterDataMaker(ctxWithRole(7, "ADMIN"), 7); err != nil {
		t.Errorf("authorizeMasterDataMaker(ADMIN) = %v, want nil", err)
	}
}
