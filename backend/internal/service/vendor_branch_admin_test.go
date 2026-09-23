package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// --- fakes ------------------------------------------------------------

type fakeVendorBranchAdminRepo struct {
	listFunc       func(ctx context.Context, arg db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error)
	countFunc      func(ctx context.Context, arg db.CountVendorBranchesAdminParams) (int64, error)
	getByIDFunc    func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error)
	findByCodeFunc func(ctx context.Context, code string) (*int64, error)
}

func (f *fakeVendorBranchAdminRepo) List(ctx context.Context, arg db.ListVendorBranchesAdminParams) ([]db.ListVendorBranchesAdminRow, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx, arg)
	}
	return nil, nil
}

func (f *fakeVendorBranchAdminRepo) Count(ctx context.Context, arg db.CountVendorBranchesAdminParams) (int64, error) {
	if f.countFunc != nil {
		return f.countFunc(ctx, arg)
	}
	return 0, nil
}

func (f *fakeVendorBranchAdminRepo) GetByID(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) {
	if f.getByIDFunc != nil {
		return f.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (f *fakeVendorBranchAdminRepo) FindByCode(ctx context.Context, code string) (*int64, error) {
	if f.findByCodeFunc != nil {
		return f.findByCodeFunc(ctx, code)
	}
	return nil, nil
}

type fakeVendorBranchSubmitter struct {
	submitFunc   func(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error)
	lastRequest  SubmitRequest
	submitCalled bool
}

func (f *fakeVendorBranchSubmitter) Submit(ctx context.Context, makerID int64, req SubmitRequest, actorIP string) (db.MasterDataChangeRequest, error) {
	f.submitCalled = true
	f.lastRequest = req
	if f.submitFunc != nil {
		return f.submitFunc(ctx, makerID, req, actorIP)
	}
	return db.MasterDataChangeRequest{ID: 1, EntityType: req.EntityType, Op: req.Op, Status: "pending"}, nil
}

// fakeVendorBranchChildCounter defaults every count to 0 (no active
// children) unless a case-specific func override is set.
type fakeVendorBranchChildCounter struct {
	vaultsFunc   func(ctx context.Context, branchID int64) (int64, error)
	picsFunc     func(ctx context.Context, branchID int64) (int64, error)
	packagesFunc func(ctx context.Context, branchID int64) (int64, error)
}

func (f *fakeVendorBranchChildCounter) CountActiveVaultsByBranch(ctx context.Context, branchID int64) (int64, error) {
	if f.vaultsFunc != nil {
		return f.vaultsFunc(ctx, branchID)
	}
	return 0, nil
}

func (f *fakeVendorBranchChildCounter) CountActivePicsByBranch(ctx context.Context, branchID int64) (int64, error) {
	if f.picsFunc != nil {
		return f.picsFunc(ctx, branchID)
	}
	return 0, nil
}

func (f *fakeVendorBranchChildCounter) CountActivePackagesByBranch(ctx context.Context, branchID int64) (int64, error) {
	if f.packagesFunc != nil {
		return f.packagesFunc(ctx, branchID)
	}
	return 0, nil
}

// --- tests --------------------------------------------------------------

func TestVendorBranchAdminService_Create_HappyPath_SubmitsCorrectRequest(t *testing.T) {
	repo := &fakeVendorBranchAdminRepo{}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorBranchAdminService(repo, sub, &fakeVendorBranchChildCounter{})

	change, err := svc.Create(context.Background(), 7, VendorBranchPayload{
		VendorID: 3, BranchCode: "  BR1  ", BranchName: "  Branch One  ", Category: "ATM",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if change.Status != "pending" {
		t.Errorf("expected pending change request, got %+v", change)
	}
	if !sub.submitCalled {
		t.Fatal("expected Submit to be called")
	}
	if sub.lastRequest.EntityType != "vendor_branch" || sub.lastRequest.Op != "create" {
		t.Errorf("unexpected submit request: %+v", sub.lastRequest)
	}
	payload, ok := sub.lastRequest.Payload.(VendorBranchPayload)
	if !ok || payload.BranchCode != "BR1" || payload.BranchName != "Branch One" {
		t.Errorf("expected trimmed payload, got %+v", sub.lastRequest.Payload)
	}
}

func TestVendorBranchAdminService_Create_MissingFields_ValidationError(t *testing.T) {
	svc := NewVendorBranchAdminService(&fakeVendorBranchAdminRepo{}, &fakeVendorBranchSubmitter{}, &fakeVendorBranchChildCounter{})

	cases := []struct {
		name string
		req  VendorBranchPayload
		want string
	}{
		{"missing vendor_id", VendorBranchPayload{BranchCode: "BR1", BranchName: "Branch", Category: "ATM"}, "vendor_id"},
		{"missing branch_code", VendorBranchPayload{VendorID: 1, BranchName: "Branch", Category: "ATM"}, "branch_code"},
		{"missing branch_name", VendorBranchPayload{VendorID: 1, BranchCode: "BR1", Category: "ATM"}, "branch_name"},
		{"missing category", VendorBranchPayload{VendorID: 1, BranchCode: "BR1", BranchName: "Branch"}, "category"},
		{"invalid category", VendorBranchPayload{VendorID: 1, BranchCode: "BR1", BranchName: "Branch", Category: "GOLD"}, "category"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), 7, tc.req, "127.0.0.1")
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.want {
				t.Fatalf("expected ValidationError on %s, got %v", tc.want, err)
			}
		})
	}
}

func TestVendorBranchAdminService_Create_DuplicateCode_Conflict(t *testing.T) {
	existingID := int64(9)
	repo := &fakeVendorBranchAdminRepo{
		findByCodeFunc: func(ctx context.Context, code string) (*int64, error) { return &existingID, nil },
	}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorBranchAdminService(repo, sub, &fakeVendorBranchChildCounter{})

	_, err := svc.Create(context.Background(), 7, VendorBranchPayload{
		VendorID: 1, BranchCode: "DUP", BranchName: "Branch", Category: "ATM",
	}, "127.0.0.1")

	if !errors.Is(err, ErrVendorBranchCodeConflict) {
		t.Fatalf("expected ErrVendorBranchCodeConflict, got %v", err)
	}
	if sub.submitCalled {
		t.Error("expected Submit not to be called on duplicate code")
	}
}

func TestVendorBranchAdminService_Update_NotFound(t *testing.T) {
	svc := NewVendorBranchAdminService(&fakeVendorBranchAdminRepo{}, &fakeVendorBranchSubmitter{}, &fakeVendorBranchChildCounter{})

	_, err := svc.Update(context.Background(), 7, 1, VendorBranchUpdatePayload{BranchName: "New"}, "127.0.0.1")

	if !errors.Is(err, ErrVendorBranchNotFound) {
		t.Fatalf("expected ErrVendorBranchNotFound, got %v", err)
	}
}

func TestVendorBranchAdminService_Update_SoftDisabledTarget_NotFound(t *testing.T) {
	repo := &fakeVendorBranchAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) {
			return &db.GetVendorBranchAdminByIDRow{ID: id, DeletedAt: pgtype.Timestamptz{Valid: true}}, nil
		},
	}
	svc := NewVendorBranchAdminService(repo, &fakeVendorBranchSubmitter{}, &fakeVendorBranchChildCounter{})

	_, err := svc.Update(context.Background(), 7, 1, VendorBranchUpdatePayload{BranchName: "New"}, "127.0.0.1")

	if !errors.Is(err, ErrVendorBranchNotFound) {
		t.Fatalf("expected ErrVendorBranchNotFound for soft-disabled target, got %v", err)
	}
}

func TestVendorBranchAdminService_Update_HappyPath_IncludesBeforeSnapshot(t *testing.T) {
	before := &db.GetVendorBranchAdminByIDRow{ID: 1, BranchCode: "BR1", BranchName: "Old Name"}
	repo := &fakeVendorBranchAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) { return before, nil },
	}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorBranchAdminService(repo, sub, &fakeVendorBranchChildCounter{})

	_, err := svc.Update(context.Background(), 7, 1, VendorBranchUpdatePayload{BranchName: "New Name", Category: "CASH"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if sub.lastRequest.EntityID == nil || *sub.lastRequest.EntityID != 1 {
		t.Errorf("expected EntityID=1, got %+v", sub.lastRequest.EntityID)
	}
	if sub.lastRequest.Before != before {
		t.Errorf("expected Before snapshot to be the fetched row, got %+v", sub.lastRequest.Before)
	}
}

func TestVendorBranchAdminService_Disable_NotFound(t *testing.T) {
	svc := NewVendorBranchAdminService(&fakeVendorBranchAdminRepo{}, &fakeVendorBranchSubmitter{}, &fakeVendorBranchChildCounter{})

	_, err := svc.Disable(context.Background(), 7, 1, "127.0.0.1")

	if !errors.Is(err, ErrVendorBranchNotFound) {
		t.Fatalf("expected ErrVendorBranchNotFound, got %v", err)
	}
}

func TestVendorBranchAdminService_Disable_HappyPath_NoActiveChildren(t *testing.T) {
	existing := &db.GetVendorBranchAdminByIDRow{ID: 1}
	repo := &fakeVendorBranchAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) { return existing, nil },
	}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorBranchAdminService(repo, sub, &fakeVendorBranchChildCounter{})

	_, err := svc.Disable(context.Background(), 7, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if !sub.submitCalled || sub.lastRequest.Op != "disable" {
		t.Errorf("expected op=disable to be submitted, got called=%v req=%+v", sub.submitCalled, sub.lastRequest)
	}
}

func TestVendorBranchAdminService_Disable_ActiveVaults_Refused(t *testing.T) {
	existing := &db.GetVendorBranchAdminByIDRow{ID: 1}
	repo := &fakeVendorBranchAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) { return existing, nil },
	}
	sub := &fakeVendorBranchSubmitter{}
	children := &fakeVendorBranchChildCounter{
		vaultsFunc: func(ctx context.Context, branchID int64) (int64, error) { return 1, nil },
	}
	svc := NewVendorBranchAdminService(repo, sub, children)

	_, err := svc.Disable(context.Background(), 7, 1, "127.0.0.1")

	if !errors.Is(err, ErrVendorBranchHasActiveChildren) {
		t.Fatalf("expected ErrVendorBranchHasActiveChildren, got %v", err)
	}
	if sub.submitCalled {
		t.Error("expected Submit not to be called when branch has active vaults")
	}
}

func TestVendorBranchAdminService_Disable_ActivePicsOrPackages_Refused(t *testing.T) {
	existing := &db.GetVendorBranchAdminByIDRow{ID: 1}
	repo := &fakeVendorBranchAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) { return existing, nil },
	}
	cases := []struct {
		name     string
		children *fakeVendorBranchChildCounter
	}{
		{"active pics", &fakeVendorBranchChildCounter{picsFunc: func(ctx context.Context, branchID int64) (int64, error) { return 1, nil }}},
		{"active packages", &fakeVendorBranchChildCounter{packagesFunc: func(ctx context.Context, branchID int64) (int64, error) { return 1, nil }}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewVendorBranchAdminService(repo, sub, tc.children)

			_, err := svc.Disable(context.Background(), 7, 1, "127.0.0.1")

			if !errors.Is(err, ErrVendorBranchHasActiveChildren) {
				t.Fatalf("expected ErrVendorBranchHasActiveChildren, got %v", err)
			}
			if sub.submitCalled {
				t.Error("expected Submit not to be called when branch has active children")
			}
		})
	}
}

func TestVendorBranchAdminService_Enable_HappyPath(t *testing.T) {
	existing := &db.GetVendorBranchAdminByIDRow{ID: 1}
	repo := &fakeVendorBranchAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorBranchAdminByIDRow, error) { return existing, nil },
	}
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorBranchAdminService(repo, sub, &fakeVendorBranchChildCounter{})

	_, err := svc.Enable(context.Background(), 7, 1, "127.0.0.1")
	if err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if sub.lastRequest.Op != "enable" {
		t.Errorf("expected op=enable, got %q", sub.lastRequest.Op)
	}
}
