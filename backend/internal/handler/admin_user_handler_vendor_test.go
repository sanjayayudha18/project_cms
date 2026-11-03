package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// vendor-party-scope task 5: D8 guard, vendor-options, vendor_branch_id wiring.

func TestAdminUserHandler_Guard_D8(t *testing.T) {
	endpoints := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/admin/users", ""},
		{http.MethodPost, "/api/v1/admin/users", `{"username":"u","full_name":"U","email":"u@example.com","role":"APPACCESS","auth_source":"ldap"}`},
		{http.MethodPut, "/api/v1/admin/users/7", `{"full_name":"U","email":"u@example.com","role":"APPACCESS"}`},
		{http.MethodPost, "/api/v1/admin/users/7/set-initial-password", `{"new_password":"InitialPass1"}`},
		{http.MethodGet, "/api/v1/admin/users/vendor-options", ""},
	}
	roles := []struct {
		role    string
		allowed bool
	}{
		{"APPACCESS", true},
		{"ADMIN", true},
		{"ADMIN_PARAM", false},
		{"VENDOR-ATM-USER", false},
	}
	for _, ep := range endpoints {
		for _, rl := range roles {
			t.Run(rl.role+" "+ep.method+" "+ep.path, func(t *testing.T) {
				router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, &fakeUserAdminServicer{}, &fakeDeactivateService{})
				rec := doRequest(router, ep.method, ep.path, tokenForRole(t, tokenSvc, 1, rl.role), ep.body)

				if forbidden := rec.Code == http.StatusForbidden; forbidden == rl.allowed {
					t.Fatalf("status %d, allowed=%v: %s", rec.Code, rl.allowed, rec.Body.String())
				}
			})
		}
	}
}

func TestAdminUserHandler_VendorOptions(t *testing.T) {
	region := "MDN"
	userSvc := &fakeUserAdminServicer{optionsResult: auth.VendorOptions{
		Vendors:  []db.ListVendorOptionsAdminRow{{ID: 5, Code: "ABA", Name: "Abacus"}},
		Branches: []db.ListVendorBranchOptionsAdminRow{{ID: 7, VendorID: 5, BranchCode: "MDN01", BranchName: "Medan", RegionCode: &region}},
	}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users/vendor-options", tokenForRole(t, tokenSvc, 1, "APPACCESS"), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Vendors  []map[string]any `json:"vendors"`
		Branches []map[string]any `json:"branches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Vendors) != 1 || body.Vendors[0]["code"] != "ABA" || body.Vendors[0]["name"] != "Abacus" {
		t.Errorf("vendors = %v", body.Vendors)
	}
	b := body.Branches
	if len(b) != 1 || b[0]["vendor_id"] != float64(5) || b[0]["branch_code"] != "MDN01" || b[0]["branch_name"] != "Medan" || b[0]["region_code"] != "MDN" {
		t.Errorf("branches = %v", b)
	}
}

func TestAdminUserHandler_VendorOptions_Error500(t *testing.T) {
	userSvc := &fakeUserAdminServicer{optionsErr: errors.New("replica down")}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/users/vendor-options", tokenForRole(t, tokenSvc, 1, "ADMIN"), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestAdminUserHandler_Create_PassesBranchAndActorRole(t *testing.T) {
	branchName := "Medan"
	userSvc := &fakeUserAdminServicer{createResult: db.CreateUserAdminRow{ID: 10, VendorID: ptrI64(5), VendorBranchID: ptrI64(7), VendorBranchName: &branchName}}
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})

	rec := doRequest(router, http.MethodPost, "/api/v1/admin/users", tokenForRole(t, tokenSvc, 1, "ADMIN"),
		`{"username":"v","full_name":"V","email":"v@example.com","role":"VENDOR-ATM-USER","auth_source":"local","temporary_password":"Passw0rd1","vendor_id":5,"vendor_branch_id":7}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if userSvc.actorRole != "ADMIN" {
		t.Errorf("actorRole = %q, want ADMIN", userSvc.actorRole)
	}
	if userSvc.createReq.VendorBranchID == nil || *userSvc.createReq.VendorBranchID != 7 {
		t.Errorf("VendorBranchID = %v, want 7", userSvc.createReq.VendorBranchID)
	}
	if !strings.Contains(rec.Body.String(), `"vendor_branch_id":7`) || !strings.Contains(rec.Body.String(), `"vendor_branch_name":"Medan"`) {
		t.Errorf("response missing branch fields: %s", rec.Body.String())
	}
}

func TestAdminUserHandler_Update_BranchAbsentNullValue(t *testing.T) {
	base := `"full_name":"V","email":"v@example.com","role":"VENDOR-ATM-USER","vendor_id":5`
	tests := []struct {
		name    string
		body    string
		wantSet bool
		wantID  *int64
	}{
		{"absent keeps", "{" + base + "}", false, nil},
		{"null clears", "{" + base + `,"vendor_branch_id":null}`, true, nil},
		{"value sets", "{" + base + `,"vendor_branch_id":9}`, true, ptrI64(9)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := &fakeUserAdminServicer{updateResult: db.UpdateUserAdminRow{ID: 7}}
			router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})

			rec := doRequest(router, http.MethodPut, "/api/v1/admin/users/7", tokenForRole(t, tokenSvc, 1, "APPACCESS"), tt.body)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			req := userSvc.updateReq
			if req.VendorBranchIDSet != tt.wantSet || !equalI64Ptr(req.VendorBranchID, tt.wantID) {
				t.Errorf("Set=%v ID=%v, want Set=%v ID=%v", req.VendorBranchIDSet, req.VendorBranchID, tt.wantSet, tt.wantID)
			}
			if userSvc.actorRole != "APPACCESS" {
				t.Errorf("actorRole = %q", userSvc.actorRole)
			}
			if !strings.Contains(rec.Body.String(), `"vendor_branch_id":null`) {
				t.Errorf("response must carry vendor_branch_id: %s", rec.Body.String())
			}
		})
	}
}

func TestAdminUserHandler_Update_BadBranchType_400(t *testing.T) {
	router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, &fakeUserAdminServicer{}, &fakeDeactivateService{})

	rec := doRequest(router, http.MethodPut, "/api/v1/admin/users/7", tokenForRole(t, tokenSvc, 1, "APPACCESS"),
		`{"full_name":"V","email":"v@example.com","role":"VENDOR-ATM-USER","vendor_branch_id":"x"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAdminUserHandler_ServiceErrors_VendorBranch(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"actor re-check", auth.ErrActorNotAllowed, http.StatusForbidden},
		{"branch reference", &auth.ReferenceError{Field: "vendor_branch_id", Message: "bukan cabang aktif milik vendor"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := &fakeUserAdminServicer{createErr: tt.err, updateErr: tt.err}
			router, tokenSvc := mountAdminUserHandler(&fakeSetInitialPasswordService{}, userSvc, &fakeDeactivateService{})
			token := tokenForRole(t, tokenSvc, 1, "APPACCESS")

			create := doRequest(router, http.MethodPost, "/api/v1/admin/users", token, `{"username":"v"}`)
			update := doRequest(router, http.MethodPut, "/api/v1/admin/users/7", token, `{"full_name":"V"}`)

			if create.Code != tt.want || update.Code != tt.want {
				t.Errorf("create=%d update=%d, want %d", create.Code, update.Code, tt.want)
			}
		})
	}
}

func ptrI64(v int64) *int64 { return &v }

func equalI64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
