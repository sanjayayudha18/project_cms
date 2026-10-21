package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// vendor-party-scope task 5: vendor role + vendor_branch_id rules and the
// APPACCESS/ADMIN re-check (Req 1.4, 2.2-2.4, 2.6, D8).

type branchCall struct{ branchID, vendorID int64 }

// recordingBranchRepo returns a fake whose branch check answers belongs and
// records every call.
func recordingBranchRepo(belongs bool, err error, calls *[]branchCall) *fakeUserAdminRepo {
	return &fakeUserAdminRepo{
		branchBelongsFunc: func(_ context.Context, branchID, vendorID int64) (bool, error) {
			*calls = append(*calls, branchCall{branchID, vendorID})
			return belongs, err
		},
	}
}

func vendorCreateReq(role, authSource string, vendorID, branchID *int64) CreateUserRequest {
	req := CreateUserRequest{
		Username: "vendor.user", FullName: "Vendor User", Email: "vendor.user@example.com",
		Role: role, AuthSource: authSource, VendorID: vendorID, VendorBranchID: branchID,
	}
	if authSource == "local" {
		req.TemporaryPassword = "Passw0rd1"
	}
	return req
}

func TestUserAdminService_Create_VendorMatrix(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		authSource string
		vendorID   *int64
		branchID   *int64
		belongs    bool
		wantField  string // "" = success
		wantRef    bool   // ReferenceError (400) instead of ValidationError (422)
		wantCheck  []branchCall
	}{
		{name: "atm user vendor-wide", role: "VENDOR-ATM-USER", authSource: "local", vendorID: ptrInt64(5)},
		{name: "vault user own branch", role: "VENDOR-VAULT-USER", authSource: "local", vendorID: ptrInt64(5), branchID: ptrInt64(7), belongs: true, wantCheck: []branchCall{{7, 5}}},
		{name: "legacy vendor user own branch", role: "VENDOR-USER", authSource: "local", vendorID: ptrInt64(5), branchID: ptrInt64(7), belongs: true, wantCheck: []branchCall{{7, 5}}},
		{name: "vendor role ldap", role: "VENDOR-USER", authSource: "ldap", wantField: "auth_source"},
		{name: "vendor role without vendor", role: "VENDOR-ATM-USER", authSource: "local", wantField: "vendor_id"},
		{name: "branch of other vendor or inactive", role: "VENDOR-ATM-USER", authSource: "local", vendorID: ptrInt64(5), branchID: ptrInt64(8), wantField: "vendor_branch_id", wantRef: true, wantCheck: []branchCall{{8, 5}}},
		{name: "internal ldap role with branch", role: "APPACCESS", authSource: "ldap", branchID: ptrInt64(7), wantField: "vendor_branch_id", wantRef: true},
		{name: "internal local role with branch", role: "OPERATOR", authSource: "local", vendorID: ptrInt64(5), branchID: ptrInt64(7), wantField: "vendor_branch_id", wantRef: true},
		{name: "internal local role without branch", role: "OPERATOR", authSource: "local", vendorID: ptrInt64(5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []branchCall
			repo := recordingBranchRepo(tt.belongs, nil, &calls)
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(repo, nil, auditW)

			got, err := svc.Create(context.Background(), 1, "APPACCESS", vendorCreateReq(tt.role, tt.authSource, tt.vendorID, tt.branchID), "10.0.0.1")

			if !equalCalls(calls, tt.wantCheck) {
				t.Errorf("branch checks = %v, want %v", calls, tt.wantCheck)
			}
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !samePtr(repo.createArg.VendorBranchID, tt.branchID) || !samePtr(got.VendorBranchID, tt.branchID) {
					t.Errorf("vendor_branch_id = %v, want %v", repo.createArg.VendorBranchID, tt.branchID)
				}
				if len(auditW.entries) != 1 {
					t.Fatalf("audit entries = %d, want 1", len(auditW.entries))
				}
				after, ok := auditW.entries[0].After.(db.CreateUserAdminRow)
				if !ok || !samePtr(after.VendorBranchID, tt.branchID) {
					t.Errorf("audit after vendor_branch_id = %v, want %v", after.VendorBranchID, tt.branchID)
				}
				return
			}
			assertFieldError(t, err, tt.wantField, tt.wantRef)
			if repo.createCalled || len(auditW.entries) != 0 {
				t.Error("rejected request must not insert or audit")
			}
		})
	}
}

func TestUserAdminService_Create_BranchCheckDBError(t *testing.T) {
	var calls []branchCall
	dbErr := errors.New("db down")
	svc := NewUserAdminService(recordingBranchRepo(false, dbErr, &calls), nil, &fakeAuditWriter{})

	_, err := svc.Create(context.Background(), 1, "ADMIN", vendorCreateReq("VENDOR-ATM-USER", "local", ptrInt64(5), ptrInt64(7)), "")

	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want wrapped db error", err)
	}
	var refErr *ReferenceError
	if errors.As(err, &refErr) {
		t.Error("a DB failure must not look like a 400 reference error")
	}
}

func TestUserAdminService_ActorRoleRecheck(t *testing.T) {
	tests := []struct {
		role    string
		allowed bool
	}{
		{"APPACCESS", true},
		{"ADMIN", true},
		{"appaccess", true},
		{"ADMIN_PARAM", false},
		{"VENDOR-USER", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run("role="+tt.role, func(t *testing.T) {
			repo := &fakeUserAdminRepo{getByIDFunc: func(context.Context, int64) (*db.GetUserAdminByIDRow, error) {
				return activeUserRow(1, "u", "ldap", "u@example.com"), nil
			}}
			svc := NewUserAdminService(repo, nil, &fakeAuditWriter{})

			_, createErr := svc.Create(context.Background(), 1, tt.role, validCreateReq(), "")
			_, updateErr := svc.Update(context.Background(), 1, tt.role, 2, UpdateUserRequest{FullName: "X", Email: "u@example.com", Role: "APPACCESS"}, "")

			for name, err := range map[string]error{"create": createErr, "update": updateErr} {
				if got := errors.Is(err, ErrActorNotAllowed); got == tt.allowed {
					t.Errorf("%s: ErrActorNotAllowed = %v, want %v (err %v)", name, got, !tt.allowed, err)
				}
			}
			if !tt.allowed && (repo.createCalled || repo.updateCalled) {
				t.Error("denied actor must not write")
			}
		})
	}
}

// vendorUserRow: an existing VENDOR-ATM-USER of vendor 5 scoped to branch 7.
func vendorUserRow() *db.GetUserAdminByIDRow {
	row := activeUserRow(1, "vendor.user", "local", "vendor.user@example.com")
	row.Role = "VENDOR-ATM-USER"
	row.VendorID = ptrInt64(5)
	row.VendorBranchID = ptrInt64(7)
	return row
}

func TestUserAdminService_Update_VendorBranch(t *testing.T) {
	tests := []struct {
		name       string
		before     func() *db.GetUserAdminByIDRow
		role       string
		vendorID   *int64
		branchSet  bool
		branchID   *int64
		belongs    bool
		wantBranch *int64
		wantField  string
		wantRef    bool
		wantCheck  []branchCall
	}{
		{name: "absent keeps branch, no re-check", before: vendorUserRow, role: "VENDOR-ATM-USER", vendorID: ptrInt64(5), wantBranch: ptrInt64(7)},
		{name: "null clears to vendor-wide", before: vendorUserRow, role: "VENDOR-ATM-USER", vendorID: ptrInt64(5), branchSet: true},
		{name: "new branch checked", before: vendorUserRow, role: "VENDOR-VAULT-USER", vendorID: ptrInt64(5), branchSet: true, branchID: ptrInt64(9), belongs: true, wantBranch: ptrInt64(9), wantCheck: []branchCall{{9, 5}}},
		{name: "vendor changed, branch kept -> rejected", before: vendorUserRow, role: "VENDOR-ATM-USER", vendorID: ptrInt64(6), wantField: "vendor_branch_id", wantRef: true, wantCheck: []branchCall{{7, 6}}},
		{name: "role to internal with kept branch -> rejected", before: vendorUserRow, role: "OPERATOR", vendorID: ptrInt64(5), wantField: "vendor_branch_id", wantRef: true},
		{name: "role to internal clearing branch", before: vendorUserRow, role: "OPERATOR", vendorID: ptrInt64(5), branchSet: true},
		{name: "vendor role on ldap user", before: func() *db.GetUserAdminByIDRow { return activeUserRow(1, "ldap.user", "ldap", "vendor.user@example.com") }, role: "VENDOR-USER", vendorID: ptrInt64(5), wantField: "auth_source"},
		{name: "vendor role without vendor", before: vendorUserRow, role: "VENDOR-ATM-USER", branchSet: true, wantField: "vendor_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []branchCall
			var gotArg db.UpdateUserAdminParams
			repo := recordingBranchRepo(tt.belongs, nil, &calls)
			repo.getByIDFunc = func(context.Context, int64) (*db.GetUserAdminByIDRow, error) { return tt.before(), nil }
			repo.updateFunc = func(_ context.Context, arg db.UpdateUserAdminParams) (db.UpdateUserAdminRow, error) {
				gotArg = arg
				return db.UpdateUserAdminRow{ID: arg.ID, VendorID: arg.VendorID, VendorBranchID: arg.VendorBranchID}, nil
			}
			auditW := &fakeAuditWriter{}
			svc := NewUserAdminService(repo, nil, auditW)

			_, err := svc.Update(context.Background(), 2, "ADMIN", 1, UpdateUserRequest{
				FullName: "Vendor User", Email: "vendor.user@example.com", Role: tt.role, VendorID: tt.vendorID,
				VendorBranchID: tt.branchID, VendorBranchIDSet: tt.branchSet,
			}, "")

			if !equalCalls(calls, tt.wantCheck) {
				t.Errorf("branch checks = %v, want %v", calls, tt.wantCheck)
			}
			if tt.wantField != "" {
				assertFieldError(t, err, tt.wantField, tt.wantRef)
				if repo.updateCalled {
					t.Error("rejected request must not update")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !samePtr(gotArg.VendorBranchID, tt.wantBranch) {
				t.Errorf("vendor_branch_id = %v, want %v", gotArg.VendorBranchID, tt.wantBranch)
			}
			entry := auditW.entries[0]
			before, _ := entry.Before.(*db.GetUserAdminByIDRow)
			after, _ := entry.After.(db.UpdateUserAdminRow)
			if before == nil || !samePtr(before.VendorBranchID, tt.before().VendorBranchID) || !samePtr(after.VendorBranchID, tt.wantBranch) {
				t.Errorf("audit before/after branch = %v/%v", before, after.VendorBranchID)
			}
		})
	}
}

type fakeVendorOptionsReader struct {
	vendors     []db.ListVendorOptionsAdminRow
	branches    []db.ListVendorBranchOptionsAdminRow
	vendorErr   error
	branchesErr error
}

func (f fakeVendorOptionsReader) ListVendorOptions(context.Context) ([]db.ListVendorOptionsAdminRow, error) {
	return f.vendors, f.vendorErr
}

func (f fakeVendorOptionsReader) ListVendorBranchOptions(context.Context) ([]db.ListVendorBranchOptionsAdminRow, error) {
	return f.branches, f.branchesErr
}

func TestUserAdminService_VendorOptions(t *testing.T) {
	boom := errors.New("replica down")
	reader := fakeVendorOptionsReader{
		vendors:  []db.ListVendorOptionsAdminRow{{ID: 5, Code: "ABA", Name: "Abacus"}},
		branches: []db.ListVendorBranchOptionsAdminRow{{ID: 7, VendorID: 5, BranchCode: "MDN", BranchName: "Medan"}},
	}
	got, err := NewUserAdminService(&fakeUserAdminRepo{}, reader, nil).VendorOptions(context.Background())
	if err != nil || len(got.Vendors) != 1 || len(got.Branches) != 1 || got.Branches[0].VendorID != 5 {
		t.Fatalf("VendorOptions = %+v, %v", got, err)
	}

	for name, r := range map[string]fakeVendorOptionsReader{
		"vendors":  {vendorErr: boom},
		"branches": {branchesErr: boom},
	} {
		if _, err := NewUserAdminService(&fakeUserAdminRepo{}, r, nil).VendorOptions(context.Background()); !errors.Is(err, boom) {
			t.Errorf("%s error = %v, want wrapped", name, err)
		}
	}
}

func assertFieldError(t *testing.T, err error, field string, wantRef bool) {
	t.Helper()
	var refErr *ReferenceError
	var valErr *pkgauth.ValidationError
	switch {
	case wantRef && errors.As(err, &refErr):
		if refErr.Field != field {
			t.Errorf("ReferenceError field = %q, want %q", refErr.Field, field)
		}
	case !wantRef && errors.As(err, &valErr):
		if valErr.Field != field {
			t.Errorf("ValidationError field = %q, want %q", valErr.Field, field)
		}
	default:
		t.Errorf("err = %v (%T), want field error on %q (reference=%v)", err, err, field, wantRef)
	}
}

func equalCalls(a, b []branchCall) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
