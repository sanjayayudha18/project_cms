package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// -- test doubles -----------------------------------------------------------

// noopBlacklist never blacklists anything -- these tests only need a valid
// signed JWT to reach the handler, not real revocation behavior.
type noopBlacklist struct{}

func (noopBlacklist) Add(_ context.Context, _ string, _ time.Duration) error { return nil }
func (noopBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type fakeApprovalOrchestrator struct {
	submitResult  db.ApprovalRequest
	submitCreated bool
	submitErr     error
	approveResult db.ApprovalRequest
	approveErr    error
	rejectResult  db.ApprovalRequest
	rejectErr     error
}

func (f *fakeApprovalOrchestrator) SubmitForApproval(_ context.Context, _ int64, _ string, _ int64, _ pgtype.Numeric, _ string) (db.ApprovalRequest, bool, error) {
	return f.submitResult, f.submitCreated, f.submitErr
}

func (f *fakeApprovalOrchestrator) Approve(_ context.Context, _, _ int64, _ string) (db.ApprovalRequest, error) {
	return f.approveResult, f.approveErr
}

func (f *fakeApprovalOrchestrator) Reject(_ context.Context, _, _ int64, _ string) (db.ApprovalRequest, error) {
	return f.rejectResult, f.rejectErr
}

type fakeApprovalReader struct {
	getResult   db.ApprovalRequest
	getErr      error
	inboxResult []approval.InboxItem
	inboxErr    error
}

func (f *fakeApprovalReader) GetRequest(_ context.Context, _ int64) (db.ApprovalRequest, error) {
	return f.getResult, f.getErr
}

func (f *fakeApprovalReader) ListInboxForApprover(_ context.Context, _ int64) ([]approval.InboxItem, error) {
	return f.inboxResult, f.inboxErr
}

// -- harness ------------------------------------------------------------

func mountApprovalHandler(orch ApprovalOrchestrator, reader ApprovalReader) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewApprovalHandler(orch, reader)
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/approvals", h.Routes())
	return r, tokenSvc
}

func tokenFor(t *testing.T, ts *pkgauth.TokenService, userID int64) string {
	t.Helper()
	access, _, err := ts.GenerateTokenPair(&pkgauth.AuthIdentity{
		UserID:     userID,
		Username:   "user",
		Role:       "ATM-USER",
		IsKaryawan: true,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return access
}

func doRequest(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// -- tests ------------------------------------------------------------

func TestApprovalHandler_Submit_HappyPath(t *testing.T) {
	orch := &fakeApprovalOrchestrator{
		submitResult:  db.ApprovalRequest{ID: 1, MakerID: 5, DocumentType: "invoice", DocumentID: 9001, RequiredLevel: 3, Status: "pending"},
		submitCreated: true,
	}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/submit", tokenFor(t, ts, 5),
		`{"document_type":"invoice","document_id":9001,"amount":"150000000.00"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body approvalRequestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != 1 || body.Status != "pending" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestApprovalHandler_Submit_DoubleSubmitConflict(t *testing.T) {
	orch := &fakeApprovalOrchestrator{
		submitResult:  db.ApprovalRequest{ID: 1, DocumentType: "invoice", DocumentID: 9001, Status: "pending"},
		submitCreated: false, // orchestrator found an existing request -- not newly created
	}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/submit", tokenFor(t, ts, 5),
		`{"document_type":"invoice","document_id":9001,"amount":"150000000.00"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestApprovalHandler_Approve_HappyPath(t *testing.T) {
	orch := &fakeApprovalOrchestrator{
		approveResult: db.ApprovalRequest{ID: 1, Status: "approved"},
	}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/1/approve", tokenFor(t, ts, 30), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body approvalRequestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "approved" {
		t.Errorf("status in body = %q, want approved", body.Status)
	}
}

func TestApprovalHandler_Approve_NotAuthorized(t *testing.T) {
	orch := &fakeApprovalOrchestrator{approveErr: approval.ErrNotAuthorized}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/1/approve", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestApprovalHandler_Reject_HappyPath(t *testing.T) {
	orch := &fakeApprovalOrchestrator{
		rejectResult: db.ApprovalRequest{ID: 1, Status: "rejected"},
	}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/1/reject", tokenFor(t, ts, 20), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body approvalRequestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "rejected" {
		t.Errorf("status in body = %q, want rejected", body.Status)
	}
}

func TestApprovalHandler_Inbox_HappyPath(t *testing.T) {
	reader := &fakeApprovalReader{
		inboxResult: []approval.InboxItem{
			{StepID: 1, RequestID: 1, StepLevel: 2, DocumentType: "invoice", DocumentID: 9001, MakerID: 5},
		},
	}
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, reader)

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/inbox", tokenFor(t, ts, 20), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body []inboxItemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 1 || body[0].RequestID != 1 {
		t.Errorf("unexpected inbox body: %+v", body)
	}
}

func TestApprovalHandler_NoToken_Unauthorized(t *testing.T) {
	router, _ := mountApprovalHandler(&fakeApprovalOrchestrator{}, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/inbox", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestApprovalHandler_Get_HappyPath(t *testing.T) {
	reader := &fakeApprovalReader{getResult: db.ApprovalRequest{ID: 1, Status: "pending"}}
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, reader)

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/1", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestApprovalHandler_Get_NotFound(t *testing.T) {
	reader := &fakeApprovalReader{getErr: errors.New("no rows")}
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, reader)

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/999", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestApprovalHandler_Get_InvalidID(t *testing.T) {
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/not-a-number", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestApprovalHandler_Submit_BadRequest(t *testing.T) {
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, &fakeApprovalReader{})
	token := tokenFor(t, ts, 5)

	cases := []struct {
		name string
		body string
	}{
		{"missing fields", `{}`},
		{"malformed json", `{"document_type":`},
		{"invalid amount", `{"document_type":"invoice","document_id":1,"amount":"not-a-number"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(router, http.MethodPost, "/api/v1/approvals/submit", token, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestApprovalHandler_Submit_PolicyNotFound(t *testing.T) {
	orch := &fakeApprovalOrchestrator{submitErr: approval.ErrPolicyNotFound}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/submit", tokenFor(t, ts, 5),
		`{"document_type":"unknown","document_id":1,"amount":"100"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestApprovalHandler_Reject_RequestNotPending(t *testing.T) {
	orch := &fakeApprovalOrchestrator{rejectErr: approval.ErrRequestNotPending}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/1/reject", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestApprovalHandler_Reject_InvalidID(t *testing.T) {
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/abc/reject", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestApprovalHandler_Inbox_InternalError(t *testing.T) {
	reader := &fakeApprovalReader{inboxErr: errors.New("db down")}
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, reader)

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/inbox", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestApprovalHandler_Approve_InvalidID(t *testing.T) {
	router, ts := mountApprovalHandler(&fakeApprovalOrchestrator{}, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/abc/approve", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestApprovalHandler_HandleError_DefaultInternalError(t *testing.T) {
	orch := &fakeApprovalOrchestrator{approveErr: errors.New("something unexpected")}
	router, ts := mountApprovalHandler(orch, &fakeApprovalReader{})

	rec := doRequest(router, http.MethodPost, "/api/v1/approvals/1/approve", tokenFor(t, ts, 5), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
