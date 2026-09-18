package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// -- test doubles -------------------------------------------------------

type fakeRbacReader struct {
	usersResult       []db.ListUserHierarchyRow
	usersErr          error
	delegationsResult []db.ApprovalDelegation
	delegationsErr    error
	leavesResult      []db.UserLeave
	leavesErr         error
	policiesResult    []db.ApprovalPolicy
	policiesErr       error
}

func (f *fakeRbacReader) ListUserHierarchy(_ context.Context) ([]db.ListUserHierarchyRow, error) {
	return f.usersResult, f.usersErr
}

func (f *fakeRbacReader) ListDelegations(_ context.Context) ([]db.ApprovalDelegation, error) {
	return f.delegationsResult, f.delegationsErr
}

func (f *fakeRbacReader) ListLeaves(_ context.Context) ([]db.UserLeave, error) {
	return f.leavesResult, f.leavesErr
}

func (f *fakeRbacReader) ListApprovalPolicies(_ context.Context) ([]db.ApprovalPolicy, error) {
	return f.policiesResult, f.policiesErr
}

type fakeApprovalPolicyWriter struct {
	createResult db.ApprovalPolicy
	createErr    error
	getResult    *db.ApprovalPolicy
	getErr       error
	updateResult db.ApprovalPolicy
	updateErr    error
}

func (f *fakeApprovalPolicyWriter) Create(_ context.Context, _ db.CreateApprovalPolicyParams) (db.ApprovalPolicy, error) {
	return f.createResult, f.createErr
}

func (f *fakeApprovalPolicyWriter) Get(_ context.Context, _ int64) (*db.ApprovalPolicy, error) {
	return f.getResult, f.getErr
}

func (f *fakeApprovalPolicyWriter) Update(_ context.Context, _ db.UpdateApprovalPolicyParams) (db.ApprovalPolicy, error) {
	return f.updateResult, f.updateErr
}

// numeric builds a pgtype.Numeric from a decimal string, failing the test on
// a bad literal (test-fixture helper only, not a code path under test).
func numeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("numeric(%q): %v", s, err)
	}
	return n
}

// -- harness --------------------------------------------------------------

func mountRbacListHandler(reader RbacReader, policyStore ApprovalPolicyWriter, auditW AdminAuditWriter) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewRbacListHandler(reader, policyStore, auditW)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM", "APPACCESS"),
	).Mount("/api/v1/admin/approval", h.Routes())
	return r, tokenSvc
}

// -- list endpoints: success shape, 401, 403 -------------------------------

func TestRbacListHandler_ListUserHierarchy_HappyPath(t *testing.T) {
	level := int32(2)
	supervisorID := int64(9)
	vendorID := int64(7)
	vendorName := "PT Advantage SCM"
	reader := &fakeRbacReader{usersResult: []db.ListUserHierarchyRow{
		{
			ID: 5, Username: "eko.juniarto", FullName: "Eko M Juniarto",
			SupervisorID: &supervisorID, ApprovalLevel: &level, Role: "ADMIN", AuthSource: "ldap",
			VendorID: &vendorID, VendorName: &vendorName,
		},
	}}
	router, ts := mountRbacListHandler(reader, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/users/hierarchy", tokenForRole(t, ts, 1, "ADMIN"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Users []userHierarchyResponse `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Users) != 1 || body.Users[0].ID != 5 || body.Users[0].Role != "ADMIN" || body.Users[0].AuthSource != "ldap" {
		t.Errorf("users = %+v, want one row for id=5 role=ADMIN auth_source=ldap", body.Users)
	}
	if body.Users[0].Username != "eko.juniarto" || body.Users[0].FullName != "Eko M Juniarto" {
		t.Errorf("users[0] username/full_name = %q/%q, want eko.juniarto/Eko M Juniarto", body.Users[0].Username, body.Users[0].FullName)
	}
	if body.Users[0].VendorID == nil || *body.Users[0].VendorID != vendorID || body.Users[0].VendorName == nil || *body.Users[0].VendorName != vendorName {
		t.Errorf("users[0] vendor_id/vendor_name = %v/%v, want %d/%q", body.Users[0].VendorID, body.Users[0].VendorName, vendorID, vendorName)
	}
}

func TestRbacListHandler_ListDelegations_HappyPath(t *testing.T) {
	reader := &fakeRbacReader{delegationsResult: []db.ApprovalDelegation{
		{ID: 3, FromUserID: 1, ToUserID: 2, StartAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, EndAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
	}}
	router, ts := mountRbacListHandler(reader, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/delegations", tokenForRole(t, ts, 1, "ADMIN_PARAM"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Delegations []delegationResponse `json:"delegations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Delegations) != 1 || body.Delegations[0].ID != 3 {
		t.Errorf("delegations = %+v, want one row for id=3", body.Delegations)
	}
}

func TestRbacListHandler_ListLeaves_HappyPath(t *testing.T) {
	reader := &fakeRbacReader{leavesResult: []db.UserLeave{
		{ID: 7, UserID: 5, StartAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, EndAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
	}}
	router, ts := mountRbacListHandler(reader, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/leaves", tokenForRole(t, ts, 1, "APPACCESS"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Leaves []leaveResponse `json:"leaves"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Leaves) != 1 || body.Leaves[0].ID != 7 {
		t.Errorf("leaves = %+v, want one row for id=7", body.Leaves)
	}
}

func TestRbacListHandler_ListPolicies_HappyPath(t *testing.T) {
	reader := &fakeRbacReader{policiesResult: []db.ApprovalPolicy{
		{ID: 1, DocumentType: "invoice", MinAmount: numeric(t, "0"), MaxAmount: numeric(t, "100000000"), RequiredLevel: 2},
	}}
	router, ts := mountRbacListHandler(reader, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/policies", tokenForRole(t, ts, 1, "APPACCESS"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Policies []policyResponse `json:"policies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Policies) != 1 || body.Policies[0].DocumentType != "invoice" || body.Policies[0].RequiredLevel != 2 {
		t.Errorf("policies = %+v, want one invoice row at required_level=2", body.Policies)
	}
}

// TestRbacListHandler_APPACCESS_Allowed is the safety net for Requirement 3:
// APPACCESS must reach these endpoints, not just ADMIN/ADMIN_PARAM.
func TestRbacListHandler_APPACCESS_Allowed(t *testing.T) {
	router, ts := mountRbacListHandler(&fakeRbacReader{}, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/policies", tokenForRole(t, ts, 1, "APPACCESS"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestRbacListHandler_NonAuthorizedRole_Forbidden(t *testing.T) {
	router, ts := mountRbacListHandler(&fakeRbacReader{}, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/policies", tokenForRole(t, ts, 1, "ATM-USER"), "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestRbacListHandler_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountRbacListHandler(&fakeRbacReader{}, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/policies", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestRbacListHandler_ListEndpoint_StoreError(t *testing.T) {
	reader := &fakeRbacReader{usersErr: fmt.Errorf("db down")}
	router, ts := mountRbacListHandler(reader, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/approval/users/hierarchy", tokenForRole(t, ts, 1, "ADMIN"), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

// -- policy create/edit: validation (400), audit-on-write --------------------

func TestRbacListHandler_CreatePolicy_HappyPath(t *testing.T) {
	policyStore := &fakeApprovalPolicyWriter{
		createResult: db.ApprovalPolicy{ID: 4, DocumentType: "invoice", MinAmount: numeric(t, "0"), MaxAmount: numeric(t, "100"), RequiredLevel: 1},
	}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountRbacListHandler(&fakeRbacReader{}, policyStore, auditW)

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/policies", tokenForRole(t, ts, 1, "ADMIN"),
		`{"document_type":"invoice","min_amount":"0","max_amount":"100","required_level":1}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(auditW.entries) != 1 {
		t.Fatalf("audit entries = %+v, want 1 entry", auditW.entries)
	}
	entry := auditW.entries[0]
	if entry.Action != "admin_create_policy" || entry.EntityType != "approval_policy" || entry.EntityID != 4 || entry.After == nil {
		t.Errorf("audit entry = %+v, want admin_create_policy on approval_policy id=4 with After set", entry)
	}
}

func TestRbacListHandler_CreatePolicy_BadBody(t *testing.T) {
	router, ts := mountRbacListHandler(&fakeRbacReader{}, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/policies", tokenForRole(t, ts, 1, "ADMIN"), `not-json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRbacListHandler_CreatePolicy_Validation(t *testing.T) {
	router, ts := mountRbacListHandler(&fakeRbacReader{}, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})
	token := tokenForRole(t, ts, 1, "ADMIN")

	cases := []struct {
		name string
		body string
	}{
		{"empty document_type", `{"document_type":"","min_amount":"0","max_amount":"100","required_level":1}`},
		{"min_amount equals max_amount (DB CHECK requires strict <)", `{"document_type":"invoice","min_amount":"100","max_amount":"100","required_level":1}`},
		{"min_amount greater than max_amount", `{"document_type":"invoice","min_amount":"200","max_amount":"100","required_level":1}`},
		{"required_level zero", `{"document_type":"invoice","min_amount":"0","max_amount":"100","required_level":0}`},
		{"required_level negative", `{"document_type":"invoice","min_amount":"0","max_amount":"100","required_level":-1}`},
		{"min_amount not a number", `{"document_type":"invoice","min_amount":"abc","max_amount":"100","required_level":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(router, http.MethodPost, "/api/v1/admin/approval/policies", token, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestRbacListHandler_UpdatePolicy_HappyPath(t *testing.T) {
	before := db.ApprovalPolicy{ID: 4, DocumentType: "invoice", MinAmount: numeric(t, "0"), MaxAmount: numeric(t, "100"), RequiredLevel: 1}
	policyStore := &fakeApprovalPolicyWriter{
		getResult:    &before,
		updateResult: db.ApprovalPolicy{ID: 4, DocumentType: "invoice", MinAmount: numeric(t, "0"), MaxAmount: numeric(t, "200"), RequiredLevel: 2},
	}
	auditW := &fakeAdminAuditWriter{}
	router, ts := mountRbacListHandler(&fakeRbacReader{}, policyStore, auditW)

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/policies/4", tokenForRole(t, ts, 1, "ADMIN"),
		`{"document_type":"invoice","min_amount":"0","max_amount":"200","required_level":2}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(auditW.entries) != 1 {
		t.Fatalf("audit entries = %+v, want 1 entry", auditW.entries)
	}
	entry := auditW.entries[0]
	if entry.Action != "admin_update_policy" || entry.EntityType != "approval_policy" || entry.EntityID != 4 || entry.Before == nil || entry.After == nil {
		t.Errorf("audit entry = %+v, want admin_update_policy on approval_policy id=4 with Before and After set", entry)
	}
}

func TestRbacListHandler_UpdatePolicy_InvalidID(t *testing.T) {
	router, ts := mountRbacListHandler(&fakeRbacReader{}, &fakeApprovalPolicyWriter{}, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/policies/abc", tokenForRole(t, ts, 1, "ADMIN"),
		`{"document_type":"invoice","min_amount":"0","max_amount":"100","required_level":1}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRbacListHandler_UpdatePolicy_NotFound(t *testing.T) {
	policyStore := &fakeApprovalPolicyWriter{getResult: nil}
	router, ts := mountRbacListHandler(&fakeRbacReader{}, policyStore, &fakeAdminAuditWriter{})

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/approval/policies/999", tokenForRole(t, ts, 1, "ADMIN"),
		`{"document_type":"invoice","min_amount":"0","max_amount":"100","required_level":1}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
