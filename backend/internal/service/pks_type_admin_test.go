package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Spec vendor-pks-type task 3.4 (unit): PksTypeAdminService validation, update-only-name,
// 409 paths, Before/EntityID wiring and the Submit RBAC re-check for this entity type.

type fakePksTypeRepo struct {
	byID      *db.VendorPksType
	byCode    *db.VendorPksType
	pending   bool
	inUse     bool
	listCalls int
}

func (f *fakePksTypeRepo) List(context.Context) ([]db.ListPksTypesRow, error) {
	f.listCalls++
	return []db.ListPksTypesRow{{ID: 1, Code: "FLM", Name: "FLM", IsActive: true}}, nil
}
func (f *fakePksTypeRepo) GetByID(context.Context, int64) (*db.VendorPksType, error) {
	return f.byID, nil
}
func (f *fakePksTypeRepo) FindByCode(context.Context, string) (*db.VendorPksType, error) {
	return f.byCode, nil
}
func (f *fakePksTypeRepo) CodePending(context.Context, string) (bool, error) { return f.pending, nil }
func (f *fakePksTypeRepo) InUse(context.Context, int64) (bool, error)        { return f.inUse, nil }

func TestPksTypeAdminService_Create(t *testing.T) {
	taken := &db.VendorPksType{ID: 9, Code: "CIT", Name: "CIT", IsActive: false}
	cases := []struct {
		name    string
		repo    fakePksTypeRepo
		req     PksTypeCreatePayload
		field   string // ValidationError field; "" = none
		wantErr error
	}{
		{"happy path, code upper-cased + name trimmed", fakePksTypeRepo{}, PksTypeCreatePayload{Code: " atm1 ", Name: "  ATM Cash "}, "", nil},
		{"code too short", fakePksTypeRepo{}, PksTypeCreatePayload{Code: "A", Name: "x"}, "code", nil},
		{"code too long", fakePksTypeRepo{}, PksTypeCreatePayload{Code: "ABCDEFGHIJK", Name: "x"}, "code", nil},
		{"code with symbol", fakePksTypeRepo{}, PksTypeCreatePayload{Code: "AB-1", Name: "x"}, "code", nil},
		{"blank name", fakePksTypeRepo{}, PksTypeCreatePayload{Code: "AB1", Name: "   "}, "name", nil},
		{"name over 100", fakePksTypeRepo{}, PksTypeCreatePayload{Code: "AB1", Name: strings.Repeat("x", 101)}, "name", nil},
		{"name exactly 100", fakePksTypeRepo{}, PksTypeCreatePayload{Code: "AB1", Name: strings.Repeat("x", 100)}, "", nil},
		{"code in table (even disabled)", fakePksTypeRepo{byCode: taken}, PksTypeCreatePayload{Code: "cit", Name: "x"}, "", ErrPksTypeConflict},
		{"code in a pending create", fakePksTypeRepo{pending: true}, PksTypeCreatePayload{Code: "AB1", Name: "x"}, "", ErrPksTypeConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewPksTypeAdminService(&tc.repo, sub)
			_, err := svc.Create(context.Background(), 7, tc.req, "ip")

			var ve *ValidationError
			switch {
			case tc.field != "":
				if !errors.As(err, &ve) || ve.Field != tc.field || sub.submitCalled {
					t.Fatalf("want %s ValidationError and no submit, got %v", tc.field, err)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) || sub.submitCalled {
					t.Fatalf("want %v and no submit, got %v", tc.wantErr, err)
				}
			default:
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				p := sub.lastRequest.Payload.(PksTypeCreatePayload)
				if sub.lastRequest.EntityType != "vendor_pks_type" || sub.lastRequest.Op != "create" ||
					sub.lastRequest.EntityID != nil || sub.lastRequest.Before != nil {
					t.Errorf("create must carry no entity id / before: %+v", sub.lastRequest)
				}
				if p.Code != strings.ToUpper(strings.TrimSpace(tc.req.Code)) || p.Name != strings.TrimSpace(tc.req.Name) {
					t.Errorf("payload not normalised: %+v", p)
				}
			}
		})
	}
}

func TestPksTypeAdminService_Update_OnlyNameAndGuards(t *testing.T) {
	ctx := context.Background()
	active := &db.VendorPksType{ID: 5, Code: "FLM", Name: "Old", IsActive: true}

	t.Run("stages name only, with Before", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewPksTypeAdminService(&fakePksTypeRepo{byID: active}, sub)
		if _, err := svc.Update(ctx, 7, 5, PksTypeUpdatePayload{Name: " New "}, "ip"); err != nil {
			t.Fatalf("Update: %v", err)
		}
		r := sub.lastRequest
		p, ok := r.Payload.(PksTypeUpdatePayload)
		if !ok || p.Name != "New" || r.Op != "update" || r.EntityID == nil || *r.EntityID != 5 || r.Before != active {
			t.Errorf("unexpected submit: %+v", r)
		}
	})
	t.Run("blank name", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewPksTypeAdminService(&fakePksTypeRepo{byID: active}, sub)
		var ve *ValidationError
		if _, err := svc.Update(ctx, 7, 5, PksTypeUpdatePayload{Name: " "}, "ip"); !errors.As(err, &ve) || ve.Field != "name" || sub.submitCalled {
			t.Fatalf("want name ValidationError, got %v", err)
		}
	})
	t.Run("unknown id", func(t *testing.T) {
		svc := NewPksTypeAdminService(&fakePksTypeRepo{}, &fakeVendorBranchSubmitter{})
		if _, err := svc.Update(ctx, 7, 404, PksTypeUpdatePayload{Name: "x"}, "ip"); !errors.Is(err, ErrPksTypeNotFound) {
			t.Fatalf("want not found, got %v", err)
		}
		if _, err := svc.Get(ctx, 404); !errors.Is(err, ErrPksTypeNotFound) {
			t.Fatalf("Get: want not found, got %v", err)
		}
	})
	t.Run("disabled type", func(t *testing.T) {
		off := &db.VendorPksType{ID: 5, Code: "FLM", Name: "Old", IsActive: false}
		sub := &fakeVendorBranchSubmitter{}
		svc := NewPksTypeAdminService(&fakePksTypeRepo{byID: off}, sub)
		if _, err := svc.Update(ctx, 7, 5, PksTypeUpdatePayload{Name: "x"}, "ip"); !errors.Is(err, ErrPksTypeNotFound) || sub.submitCalled {
			t.Fatalf("want not found and no submit, got %v", err)
		}
	})
}

func TestPksTypeAdminService_DisableEnable(t *testing.T) {
	ctx := context.Background()
	row := &db.VendorPksType{ID: 5, Code: "FLM", Name: "FLM", IsActive: true}

	t.Run("disable in use -> 409 and no submit", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewPksTypeAdminService(&fakePksTypeRepo{byID: row, inUse: true}, sub)
		if _, err := svc.Disable(ctx, 7, 5, "ip"); !errors.Is(err, ErrPksTypeInUse) || sub.submitCalled {
			t.Fatalf("want ErrPksTypeInUse and no submit, got %v", err)
		}
	})
	t.Run("disable free type stages with Before", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewPksTypeAdminService(&fakePksTypeRepo{byID: row}, sub)
		if _, err := svc.Disable(ctx, 7, 5, "ip"); err != nil {
			t.Fatalf("Disable: %v", err)
		}
		if r := sub.lastRequest; r.Op != "disable" || r.EntityID == nil || *r.EntityID != 5 || r.Before != row || r.Payload != nil {
			t.Errorf("unexpected submit: %+v", r)
		}
	})
	t.Run("enable stages with Before", func(t *testing.T) {
		sub := &fakeVendorBranchSubmitter{}
		svc := NewPksTypeAdminService(&fakePksTypeRepo{byID: row}, sub)
		if _, err := svc.Enable(ctx, 7, 5, "ip"); err != nil {
			t.Fatalf("Enable: %v", err)
		}
		if r := sub.lastRequest; r.Op != "enable" || r.EntityID == nil || *r.EntityID != 5 || r.Before != row {
			t.Errorf("unexpected submit: %+v", r)
		}
	})
	t.Run("unknown id", func(t *testing.T) {
		svc := NewPksTypeAdminService(&fakePksTypeRepo{}, &fakeVendorBranchSubmitter{})
		if _, err := svc.Disable(ctx, 7, 404, "ip"); !errors.Is(err, ErrPksTypeNotFound) {
			t.Errorf("Disable: want not found, got %v", err)
		}
		if _, err := svc.Enable(ctx, 7, 404, "ip"); !errors.Is(err, ErrPksTypeNotFound) {
			t.Errorf("Enable: want not found, got %v", err)
		}
	})
}

func TestPksTypeAdminService_ListUsesRepo(t *testing.T) {
	repo := &fakePksTypeRepo{}
	rows, err := NewPksTypeAdminService(repo, &fakeVendorBranchSubmitter{}).List(context.Background())
	if err != nil || len(rows) != 1 || repo.listCalls != 1 {
		t.Fatalf("List: rows=%d calls=%d err=%v", len(rows), repo.listCalls, err)
	}
}

// RBAC at the service layer for this entity type: only ADMIN/ADMIN_PARAM stage it
// (not the cash count roles), and the check runs before any repo/orchestrator/audit use
// (the engine is built with nil deps, so touching them would panic).
func TestMasterDataChangeService_Submit_PksType_RBAC(t *testing.T) {
	req := SubmitRequest{EntityType: "vendor_pks_type", Op: "create", Payload: PksTypeCreatePayload{Code: "AB1", Name: "x"}}
	svc := NewMasterDataChangeService(nil, nil, nil)

	for _, role := range []string{"ATM-USER", "VENDOR-USER", "CC-CWM", "CC-CCR", "ACM-SPV"} {
		if _, err := svc.Submit(ctxWithRole(7, role), 7, req, "ip"); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("role %s: want ErrMasterDataForbidden, got %v", role, err)
		}
	}
	if _, err := svc.Submit(context.Background(), 7, req, "ip"); !errors.Is(err, ErrMasterDataForbidden) {
		t.Errorf("no auth context: want ErrMasterDataForbidden, got %v", err)
	}
	if _, err := svc.Submit(adminCtx(8), 7, req, "ip"); !errors.Is(err, ErrMasterDataForbidden) {
		t.Errorf("maker id != caller: want ErrMasterDataForbidden, got %v", err)
	}
}
