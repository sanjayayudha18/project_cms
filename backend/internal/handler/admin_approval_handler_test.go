package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeAdminStore struct {
	hierarchyResult  db.UpdateUserHierarchyRow
	hierarchyErr     error
	delegationResult db.ApprovalDelegation
	delegationErr    error
	revokeResult     *db.ApprovalDelegation
	revokeErr        error
	leaveResult      db.UserLeave
	leaveErr         error
}

func (f *fakeAdminStore) SetUserHierarchy(_ context.Context, _ int64, _ *int64, _ *int32) (db.UpdateUserHierarchyRow, error) {
	return f.hierarchyResult, f.hierarchyErr
}

func (f *fakeAdminStore) CreateDelegation(_ context.Context, _, _ int64, _, _ time.Time, _ *string) (db.ApprovalDelegation, error) {
	return f.delegationResult, f.delegationErr
}

func (f *fakeAdminStore) RevokeDelegation(_ context.Context, _ int64, _ time.Time) (*db.ApprovalDelegation, error) {
	return f.revokeResult, f.revokeErr
}

func (f *fakeAdminStore) CreateLeave(_ context.Context, _ int64, _, _ time.Time, _ *string) (db.UserLeave, error) {
	return f.leaveResult, f.leaveErr
}

type fakeAdminAuditWriter struct{ entries []audit.Entry }

func (f *fakeAdminAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	f.entries = append(f.entries, entry)
	return nil
}

func mountAdminApprovalHandler(store AdminStore, auditW AdminAuditWriter) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})

	h := NewAdminApprovalHandler(store, auditW)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/approval", h.Routes())
	return r, tokenSvc
}

func tokenForRole(t *testing.T, ts *pkgauth.TokenService, userID int64, role string) string {
	t.Helper()
	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{
		UserID:   userID,
		Username: "user",
		Role:     role,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return access
}

func TestAdminApprovalHandler_NonAdminRole_Forbidden(t *testing.T) {
	router, ts := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/leaves", tokenForRole(t, ts, 1, "ATM-USER"),
		`{"user_id":5,"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAdminApprovalHandler_AdminRole_SetHierarchy_HappyPath(t *testing.T) {
	level := int32(3)
	supervisorID := int64(20)
	store := &fakeAdminStore{
		hierarchyResult: db.UpdateUserHierarchyRow{ID: 5, SupervisorID: &supervisorID, ApprovalLevel: &level},
	}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountAdminApprovalHandler(store, auditW)

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/users/5/hierarchy", tokenForRole(t, ts, 1, "ADMIN"),
		`{"supervisor_id":20,"approval_level":3}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(auditW.entries) != 1 || auditW.entries[0].Action != "admin_set_hierarchy" {
		t.Errorf("audit entries = %+v, want 1 entry with action admin_set_hierarchy", auditW.entries)
	}
}

func TestAdminApprovalHandler_CreateDelegation_OverlapConflict(t *testing.T) {
	store := &fakeAdminStore{delegationErr: approval.ErrDelegationOverlap}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountAdminApprovalHandler(store, auditW)

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/delegations", tokenForRole(t, ts, 1, "ADMIN_PARAM"),
		`{"from_user_id":10,"to_user_id":20,"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if len(auditW.entries) != 0 {
		t.Errorf("audit entries = %+v, want none (failed create should not write audit)", auditW.entries)
	}
}

func TestAdminApprovalHandler_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/leaves", "",
		`{"user_id":5,"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestAdminApprovalHandler_SetHierarchy_SelfSupervisorRejected(t *testing.T) {
	router, ts := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/users/5/hierarchy", tokenForRole(t, ts, 1, "ADMIN"),
		`{"supervisor_id":5}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminApprovalHandler_SetHierarchy_InvalidID(t *testing.T) {
	router, ts := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/users/abc/hierarchy", tokenForRole(t, ts, 1, "ADMIN"),
		`{"approval_level":2}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminApprovalHandler_CreateDelegation_BadRequest(t *testing.T) {
	router, ts := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})
	token := tokenForRole(t, ts, 1, "ADMIN")

	cases := []struct {
		name string
		body string
	}{
		{"missing ids", `{"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`},
		{"bad date format", `{"from_user_id":1,"to_user_id":2,"start_at":"not-a-date","end_at":"2026-09-15T00:00:00Z"}`},
		{"start after end", `{"from_user_id":1,"to_user_id":2,"start_at":"2026-09-15T00:00:00Z","end_at":"2026-09-10T00:00:00Z"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/delegations", token, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestAdminApprovalHandler_RevokeDelegation_HappyPath(t *testing.T) {
	store := &fakeAdminStore{
		revokeResult: &db.ApprovalDelegation{ID: 3, FromUserID: 1, ToUserID: 2},
	}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountAdminApprovalHandler(store, auditW)

	rec := doRequest(router, http.MethodDelete, "/api/v1/admin/approval/delegations/3", tokenForRole(t, ts, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(auditW.entries) != 1 || auditW.entries[0].Action != "admin_revoke_delegation" {
		t.Errorf("audit entries = %+v, want 1 entry with action admin_revoke_delegation", auditW.entries)
	}
}

func TestAdminApprovalHandler_RevokeDelegation_NotFound(t *testing.T) {
	store := &fakeAdminStore{revokeResult: nil}
	router, ts := mountAdminApprovalHandler(store, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodDelete, "/api/v1/admin/approval/delegations/999", tokenForRole(t, ts, 1, "ADMIN"), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAdminApprovalHandler_RevokeDelegation_InvalidID(t *testing.T) {
	router, ts := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodDelete, "/api/v1/admin/approval/delegations/abc", tokenForRole(t, ts, 1, "ADMIN"), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminApprovalHandler_CreateLeave_HappyPath(t *testing.T) {
	store := &fakeAdminStore{leaveResult: db.UserLeave{ID: 7, UserID: 5}}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountAdminApprovalHandler(store, auditW)

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/leaves", tokenForRole(t, ts, 1, "ADMIN_PARAM"),
		`{"user_id":5,"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(auditW.entries) != 1 || auditW.entries[0].Action != "admin_create_leave" {
		t.Errorf("audit entries = %+v, want 1 entry with action admin_create_leave", auditW.entries)
	}
}

func TestAdminApprovalHandler_CreateDelegation_HappyPath(t *testing.T) {
	store := &fakeAdminStore{
		delegationResult: db.ApprovalDelegation{ID: 3, FromUserID: 1, ToUserID: 2},
	}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountAdminApprovalHandler(store, auditW)

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/delegations", tokenForRole(t, ts, 1, "ADMIN"),
		`{"from_user_id":1,"to_user_id":2,"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(auditW.entries) != 1 || auditW.entries[0].Action != "admin_create_delegation" {
		t.Errorf("audit entries = %+v, want 1 entry with action admin_create_delegation", auditW.entries)
	}
}

func TestAdminApprovalHandler_SetHierarchy_StoreError(t *testing.T) {
	store := &fakeAdminStore{hierarchyErr: fmt.Errorf("db down")}
	router, ts := mountAdminApprovalHandler(store, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/users/5/hierarchy", tokenForRole(t, ts, 1, "ADMIN"),
		`{"approval_level":2}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestAdminApprovalHandler_RevokeDelegation_StoreError(t *testing.T) {
	store := &fakeAdminStore{revokeErr: fmt.Errorf("db down")}
	router, ts := mountAdminApprovalHandler(store, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodDelete, "/api/v1/admin/approval/delegations/3", tokenForRole(t, ts, 1, "ADMIN"), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestAdminApprovalHandler_CreateLeave_StoreError(t *testing.T) {
	store := &fakeAdminStore{leaveErr: fmt.Errorf("db down")}
	router, ts := mountAdminApprovalHandler(store, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/leaves", tokenForRole(t, ts, 1, "ADMIN"),
		`{"user_id":5,"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestAdminApprovalHandler_CreateLeave_BadRequest(t *testing.T) {
	router, ts := mountAdminApprovalHandler(&fakeAdminStore{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/leaves", tokenForRole(t, ts, 1, "ADMIN"),
		`{"start_at":"2026-09-10T00:00:00Z","end_at":"2026-09-15T00:00:00Z"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
