package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeVendorPksServicer exercises HTTP wiring only; logic is covered by
// internal/service/vendor_pks_admin_test.go. calls counts every service call.
type fakeVendorPksServicer struct {
	err   error
	calls int

	gotMaker, gotVendor, gotPks int64
	gotReason                   string
	gotNew                      *service.SubmitVendorPksRequest
}

func (f *fakeVendorPksServicer) Get(_ context.Context, vendorID int64) (*service.VendorPksDetail, error) {
	f.calls++
	f.gotVendor = vendorID
	if f.err != nil {
		return nil, f.err
	}
	return &service.VendorPksDetail{VendorID: vendorID}, nil
}

func (f *fakeVendorPksServicer) SubmitNew(_ context.Context, makerID, vendorID int64, req service.SubmitVendorPksRequest, _ string) (db.MasterDataChangeRequest, error) {
	f.calls++
	f.gotMaker, f.gotVendor, f.gotNew = makerID, vendorID, &req
	return db.MasterDataChangeRequest{ID: 11, EntityType: "vendor_pks", Op: "create", Status: "pending"}, f.err
}

func (f *fakeVendorPksServicer) SubmitTerminate(_ context.Context, makerID, vendorID, pksID int64, reason, _ string) (db.MasterDataChangeRequest, error) {
	f.calls++
	f.gotMaker, f.gotVendor, f.gotPks, f.gotReason = makerID, vendorID, pksID, reason
	return db.MasterDataChangeRequest{ID: 12, EntityType: "vendor_pks", Op: "terminate", Status: "pending"}, f.err
}

func (f *fakeVendorPksServicer) SubmitUnterminate(_ context.Context, makerID, vendorID, pksID int64, reason, _ string) (db.MasterDataChangeRequest, error) {
	f.calls++
	f.gotMaker, f.gotVendor, f.gotPks, f.gotReason = makerID, vendorID, pksID, reason
	return db.MasterDataChangeRequest{ID: 13, EntityType: "vendor_pks", Op: "unterminate", Status: "pending"}, f.err
}

func mountAdminVendorPksHandler(svc *fakeVendorPksServicer) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors/{vendorID}/pks", NewAdminVendorPksHandler(svc).Routes())
	return r, tokenSvc
}

const (
	pksBase      = "/api/v1/admin/vendors/3/pks"
	pksCreateReq = `{"pks_name":"PKS 2026","effective_date":"2026-01-01","expiry_date":"2027-01-01"}`
	pksReasonReq = `{"reason":"alasan yang cukup panjang"}`
)

// pksEndpoints are the four operations of the PKS tab.
var pksEndpoints = []struct {
	name, method, path, body string
}{
	{"get", http.MethodGet, pksBase, ""},
	{"create", http.MethodPost, pksBase, pksCreateReq},
	{"terminate", http.MethodPost, pksBase + "/5/terminate", pksReasonReq},
	{"unterminate", http.MethodPost, pksBase + "/5/unterminate", pksReasonReq},
}

type pksErrBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details []struct {
		Field string `json:"field"`
	} `json:"details"`
}

func TestAdminVendorPksHandler_Success(t *testing.T) {
	svc := &fakeVendorPksServicer{}
	router, ts := mountAdminVendorPksHandler(svc)
	tok := tokenForRole(t, ts, 9, "ADMIN")

	rec := doRequest(router, http.MethodGet, pksBase, tok, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"vendor_id":3`) {
		t.Fatalf("get: want 200 with detail, got %d: %s", rec.Code, rec.Body.String())
	}

	for _, tc := range pksEndpoints[1:] {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(router, tc.method, tc.path, tok, tc.body)
			if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"change_request_id"`) {
				t.Fatalf("want 202 with change_request_id, got %d: %s", rec.Code, rec.Body.String())
			}
			if svc.gotMaker != 9 || svc.gotVendor != 3 {
				t.Errorf("maker/vendor = %d/%d, want 9/3", svc.gotMaker, svc.gotVendor)
			}
		})
	}
	if svc.gotPks != 5 || svc.gotReason != "alasan yang cukup panjang" {
		t.Errorf("unterminate got pks=%d reason=%q", svc.gotPks, svc.gotReason)
	}
}

func TestAdminVendorPksHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string // substring, "" = skip
		wantField  string
	}{
		{"validation", &service.ValidationError{Field: "pks_name", Message: "wajib diisi"}, 422, "validation_error", "", "pks_name"},
		{"internal vendor", service.ErrVendorPksInternalVendor, 422, "validation_error", "", "vendor"},
		{"vendor not found", service.ErrVendorNotFound, 404, "not_found", "Vendor tidak ditemukan", ""},
		{"pks not found", service.ErrVendorPksNotFound, 404, "not_found", "PKS tidak ditemukan", ""},
		{"change pending", service.ErrMasterDataChangePending, 409, "conflict", "menunggu persetujuan", ""},
		{"vendor inactive", service.ErrVendorPksVendorInactive, 409, "conflict", "Vendor nonaktif", ""},
		{"not active", service.ErrVendorPksNotActive, 409, "conflict", "sudah digantikan", ""},
		{"already terminated", service.ErrVendorPksAlreadyTerminated, 409, "conflict", "sudah Terminated", ""},
		{"not terminated", service.ErrVendorPksNotTerminated, 409, "conflict", "tidak dalam status Terminated", ""},
		{"forbidden", service.ErrMasterDataForbidden, 403, "forbidden", "tidak berhak", ""},
		{"chain incomplete", fmt.Errorf("submit: %w", approval.ErrChainIncomplete), 422, "approval_chain_incomplete", "", ""},
		{"wrapped sentinel", fmt.Errorf("x: %w", service.ErrVendorPksNotFound), 404, "not_found", "", ""},
		{"unexpected", errors.New("pq: connection refused secret-detail"), 500, "internal_error", "Terjadi kesalahan internal", ""},
	}
	for _, c := range cases {
		for _, ep := range pksEndpoints {
			t.Run(c.name+"/"+ep.name, func(t *testing.T) {
				router, ts := mountAdminVendorPksHandler(&fakeVendorPksServicer{err: c.err})
				rec := doRequest(router, ep.method, ep.path, tokenForRole(t, ts, 1, "ADMIN"), ep.body)
				if rec.Code != c.wantStatus {
					t.Fatalf("status = %d, want %d: %s", rec.Code, c.wantStatus, rec.Body.String())
				}
				var body pksErrBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if body.Error != c.wantCode {
					t.Errorf("error = %q, want %q", body.Error, c.wantCode)
				}
				if c.wantMsg != "" && !strings.Contains(body.Message, c.wantMsg) {
					t.Errorf("message = %q, want substring %q", body.Message, c.wantMsg)
				}
				if c.wantField != "" && (len(body.Details) != 1 || body.Details[0].Field != c.wantField) {
					t.Errorf("details = %+v, want field %q", body.Details, c.wantField)
				}
				if c.wantStatus == 500 && strings.Contains(rec.Body.String(), "secret-detail") {
					t.Errorf("500 body leaks error detail: %s", rec.Body.String())
				}
			})
		}
	}
}

func TestAdminVendorPksHandler_BadRequests(t *testing.T) {
	cases := []struct{ name, method, path, body string }{
		{"create body not JSON", http.MethodPost, pksBase, `{not json`},
		{"terminate body not JSON", http.MethodPost, pksBase + "/5/terminate", `nope`},
		{"unterminate body not JSON", http.MethodPost, pksBase + "/5/unterminate", `nope`},
		{"bad pksID", http.MethodPost, pksBase + "/abc/terminate", pksReasonReq},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &fakeVendorPksServicer{}
			router, ts := mountAdminVendorPksHandler(svc)
			rec := doRequest(router, c.method, c.path, tokenForRole(t, ts, 1, "ADMIN"), c.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
				t.Fatalf("want 400 bad_request, got %d: %s", rec.Code, rec.Body.String())
			}
			if svc.calls != 0 {
				t.Errorf("service called %d times on bad request", svc.calls)
			}
		})
	}
}

// Property 10 (backend part): only ADMIN / ADMIN_PARAM reach the service.
func TestAdminVendorPksHandler_RoleGuard(t *testing.T) {
	roles := []struct {
		role    string
		allowed bool
	}{
		{"ADMIN", true},
		{"ADMIN_PARAM", true},
		{"APPACCESS", false},
		{"ATM-USER", false},
		{"ATM-SPV", false},
		{"BRANCH-ATM-SPV", false},
		{"ACM-USER", false},
		{"ACM-SPV", false},
		{"VENDOR-USER", false},
		{"VENDOR-ATM-USER", false},
		{"VENDOR-VAULT-USER", false},
	}
	for _, r := range roles {
		for _, ep := range pksEndpoints {
			t.Run(r.role+"/"+ep.name, func(t *testing.T) {
				svc := &fakeVendorPksServicer{}
				router, ts := mountAdminVendorPksHandler(svc)
				rec := doRequest(router, ep.method, ep.path, tokenForRole(t, ts, 1, r.role), ep.body)
				if r.allowed {
					if rec.Code == http.StatusForbidden || svc.calls != 1 {
						t.Fatalf("allowed role: status %d, calls %d", rec.Code, svc.calls)
					}
					return
				}
				if rec.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
				}
				if svc.calls != 0 {
					t.Errorf("service called %d times for forbidden role", svc.calls)
				}
			})
		}
	}
}

func TestAdminVendorPksHandler_NoToken_Unauthorized(t *testing.T) {
	svc := &fakeVendorPksServicer{}
	router, _ := mountAdminVendorPksHandler(svc)
	for _, ep := range pksEndpoints {
		rec := doRequest(router, ep.method, ep.path, "", ep.body)
		if rec.Code != http.StatusUnauthorized || svc.calls != 0 {
			t.Errorf("%s: status %d, calls %d, want 401 / 0", ep.name, rec.Code, svc.calls)
		}
	}
}
