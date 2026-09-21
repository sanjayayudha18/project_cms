package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// --- fakes ------------------------------------------------------------

// fakeVendorAdminRepo is read-only, like VendorAdminRepo itself: the service
// has no write path to the vendors table (T4.1), so there is nothing to
// record other than what the fake Submitter sees.
type fakeVendorAdminRepo struct {
	listFunc             func(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error)
	countFunc            func(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error)
	getByIDFunc          func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error)
	findByCodeFunc       func(ctx context.Context, code string) (*int64, error)
	countActiveUsersFunc func(ctx context.Context, vendorID int64) (int64, error)
}

func (f *fakeVendorAdminRepo) List(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx, arg)
	}
	return nil, nil
}

func (f *fakeVendorAdminRepo) Count(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error) {
	if f.countFunc != nil {
		return f.countFunc(ctx, arg)
	}
	return 0, nil
}

func (f *fakeVendorAdminRepo) GetByID(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
	if f.getByIDFunc != nil {
		return f.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (f *fakeVendorAdminRepo) FindByCode(ctx context.Context, code string) (*int64, error) {
	if f.findByCodeFunc != nil {
		return f.findByCodeFunc(ctx, code)
	}
	return nil, nil
}

func (f *fakeVendorAdminRepo) CountActiveUsers(ctx context.Context, vendorID int64) (int64, error) {
	if f.countActiveUsersFunc != nil {
		return f.countActiveUsersFunc(ctx, vendorID)
	}
	return 0, nil
}

func activeVendor(id int64, code string) *db.GetVendorAdminByIDRow {
	return &db.GetVendorAdminByIDRow{ID: id, Code: code, Name: "Vendor " + code, IsActive: true}
}

func disabledVendor(id int64, code string) *db.GetVendorAdminByIDRow {
	return &db.GetVendorAdminByIDRow{ID: id, Code: code, Name: "Vendor " + code, IsActive: false, DeletedAt: pgtype.Timestamptz{Valid: true}}
}

func vendorRepoWith(row func(id int64) *db.GetVendorAdminByIDRow) *fakeVendorAdminRepo {
	return &fakeVendorAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) { return row(id), nil }}
}

// --- Create -------------------------------------------------------------

func TestVendorAdminService_Create_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateVendorRequest
		wantErr string // ValidationError.Field
	}{
		{"missing code", CreateVendorRequest{Name: "Acme"}, "code"},
		{"missing name", CreateVendorRequest{Code: "ACM"}, "name"},
		{"blank code (whitespace only)", CreateVendorRequest{Code: "   ", Name: "Acme"}, "code"},
		{"invalid contact_email", CreateVendorRequest{Code: "ACM", Name: "Acme", ContactEmail: "not-an-email"}, "contact_email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewVendorAdminService(&fakeVendorAdminRepo{}, sub)

			_, err := svc.Create(context.Background(), 1, tt.req, "10.0.0.1")
			var valErr *ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("Create(%+v) err = %v, want *ValidationError", tt.req, err)
			}
			if valErr.Field != tt.wantErr {
				t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, tt.wantErr)
			}
			if sub.submitCalled {
				t.Error("a change request was staged despite validation failure")
			}
		})
	}
}

func TestVendorAdminService_Create_CodeConflict_StagesNothing(t *testing.T) {
	existingID := int64(42)
	repo := &fakeVendorAdminRepo{findByCodeFunc: func(ctx context.Context, code string) (*int64, error) { return &existingID, nil }}
	sub := &fakeVendorBranchSubmitter{}

	_, err := NewVendorAdminService(repo, sub).Create(context.Background(), 1, CreateVendorRequest{Code: "ACM", Name: "Acme"}, "10.0.0.1")

	if !errors.Is(err, ErrVendorCodeConflict) {
		t.Fatalf("err = %v, want ErrVendorCodeConflict", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite a code conflict")
	}
}

func TestVendorAdminService_Create_StagesTrimmedPayload(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(&fakeVendorAdminRepo{}, sub)

	change, err := svc.Create(context.Background(), 7, CreateVendorRequest{
		Code: "  ACM ", Name: " Acme ", ContactEmail: " ops@acme.test ", ContactPhone: " 0812 ", HqAddress: "  Jl. A ",
	}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if change.ID == 0 || change.Status != "pending" {
		t.Errorf("want the staged pending change returned, got %+v", change)
	}
	if sub.lastRequest.EntityType != "vendor" || sub.lastRequest.Op != "create" || sub.lastRequest.EntityID != nil {
		t.Errorf("unexpected submit: %+v", sub.lastRequest)
	}
	want := vendorCreatePayload{Code: "ACM", Name: "Acme", ContactEmail: "ops@acme.test", ContactPhone: "0812", HqAddress: "Jl. A"}
	if got, ok := sub.lastRequest.Payload.(vendorCreatePayload); !ok || got != want {
		t.Errorf("payload = %+v, want trimmed %+v", sub.lastRequest.Payload, want)
	}
}

func TestVendorAdminService_Create_SubmitErrorPropagates(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{submitFunc: func(context.Context, int64, SubmitRequest, string) (db.MasterDataChangeRequest, error) {
		return db.MasterDataChangeRequest{}, ErrMasterDataForbidden
	}}
	_, err := NewVendorAdminService(&fakeVendorAdminRepo{}, sub).Create(context.Background(), 1, CreateVendorRequest{Code: "ACM", Name: "Acme"}, "ip")
	if !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("err = %v, want ErrMasterDataForbidden passed through", err)
	}
}

// --- Update ---------------------------------------------------------------

func TestVendorAdminService_Update_NotFound(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeVendorAdminRepo
	}{
		{"missing id", &fakeVendorAdminRepo{}},
		{"soft-disabled target", vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return disabledVendor(id, "ACM") })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			_, err := NewVendorAdminService(tt.repo, sub).Update(context.Background(), 1, 99, UpdateVendorRequest{Name: "New Name"}, "10.0.0.1")
			if !errors.Is(err, ErrVendorNotFound) {
				t.Fatalf("err = %v, want ErrVendorNotFound", err)
			}
			if sub.submitCalled {
				t.Error("a change request was staged despite a not-found target")
			}
		})
	}
}

func TestVendorAdminService_Update_ImmutableCodeGuard(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return activeVendor(id, "ACM") }), sub)

	newCode := "DIFFERENT"
	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Code: &newCode, Name: "Acme"}, "10.0.0.1")

	if !errors.Is(err, ErrVendorCodeImmutable) {
		t.Fatalf("err = %v, want ErrVendorCodeImmutable", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite an immutable-code change attempt")
	}
}

func TestVendorAdminService_Update_SameCodeInPayloadIsAllowed(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return activeVendor(id, "ACM") }), sub)

	sameCode := " ACM "
	if _, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Code: &sameCode, Name: "Acme Renamed"}, "10.0.0.1"); err != nil {
		t.Fatalf("Update with unchanged code: %v", err)
	}
	if !sub.submitCalled {
		t.Error("expected a change request to be staged")
	}
}

func TestVendorAdminService_Update_Validation(t *testing.T) {
	svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return activeVendor(id, "ACM") }), &fakeVendorBranchSubmitter{})

	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Name: "", ContactEmail: ""}, "10.0.0.1")
	var valErr *ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "name" {
		t.Fatalf("err = %v, want *ValidationError{Field: name}", err)
	}

	_, err = svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Name: "Acme", ContactEmail: "bad"}, "10.0.0.1")
	if !errors.As(err, &valErr) || valErr.Field != "contact_email" {
		t.Fatalf("err = %v, want *ValidationError{Field: contact_email}", err)
	}
}

// The "before" snapshot is what the T2.5 staleness check compares against, so
// it must be the vendor row exactly as loaded, and the payload the trimmed new
// values.
func TestVendorAdminService_Update_StagesPayloadWithBeforeSnapshot(t *testing.T) {
	before := activeVendor(1, "ACM")
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return before }), sub)

	if _, err := svc.Update(context.Background(), 7, 1, UpdateVendorRequest{Name: " Acme Renamed ", ContactPhone: " 0812 "}, "10.0.0.1"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	req := sub.lastRequest
	if req.EntityType != "vendor" || req.Op != "update" || req.EntityID == nil || *req.EntityID != 1 {
		t.Errorf("unexpected submit: %+v", req)
	}
	if req.Before != before {
		t.Errorf("Before = %+v, want the loaded vendor row %+v", req.Before, before)
	}
	want := vendorUpdatePayload{Name: "Acme Renamed", ContactPhone: "0812"}
	if got, ok := req.Payload.(vendorUpdatePayload); !ok || got != want {
		t.Errorf("payload = %+v, want %+v", req.Payload, want)
	}
}

// --- Disable / Enable -------------------------------------------------

func TestVendorAdminService_Disable_NotFound(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	_, err := NewVendorAdminService(&fakeVendorAdminRepo{}, sub).Disable(context.Background(), 1, 99, "10.0.0.1")
	if !errors.Is(err, ErrVendorNotFound) {
		t.Fatalf("err = %v, want ErrVendorNotFound", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite a not-found target")
	}
}

func TestVendorAdminService_Disable_StagesAndSurfacesLinkedUsersWarning(t *testing.T) {
	tests := []struct {
		name         string
		linkedActive int64
	}{
		{"no linked active users", 0},
		{"linked active users still present", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return activeVendor(id, "ACM") })
			repo.countActiveUsersFunc = func(ctx context.Context, vendorID int64) (int64, error) { return tt.linkedActive, nil }
			sub := &fakeVendorBranchSubmitter{}

			result, err := NewVendorAdminService(repo, sub).Disable(context.Background(), 7, 1, "10.0.0.1")
			if err != nil {
				t.Fatalf("Disable: %v", err)
			}
			if result.LinkedUsersWarning != tt.linkedActive {
				t.Errorf("LinkedUsersWarning = %d, want %d (staging must still succeed)", result.LinkedUsersWarning, tt.linkedActive)
			}
			if result.Change.Status != "pending" {
				t.Errorf("Change = %+v, want the pending staged change", result.Change)
			}
			if sub.lastRequest.Op != "disable" || sub.lastRequest.EntityType != "vendor" || sub.lastRequest.Before == nil {
				t.Errorf("unexpected submit: %+v", sub.lastRequest)
			}
		})
	}
}

func TestVendorAdminService_Enable_NotFound_StagesNothing(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	_, err := NewVendorAdminService(&fakeVendorAdminRepo{}, sub).Enable(context.Background(), 1, 99, "10.0.0.1")
	if !errors.Is(err, ErrVendorNotFound) {
		t.Fatalf("err = %v, want ErrVendorNotFound", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged for a non-existent id")
	}
}

func TestVendorAdminService_Enable_Stages(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return disabledVendor(id, "ACM") }), sub)

	change, err := svc.Enable(context.Background(), 7, 1, "10.0.0.1")
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if change.Status != "pending" || sub.lastRequest.Op != "enable" || sub.lastRequest.EntityID == nil || *sub.lastRequest.EntityID != 1 || sub.lastRequest.Before == nil {
		t.Errorf("unexpected: change=%+v submit=%+v", change, sub.lastRequest)
	}
}

// --- NPWP / legal name (T4.3) ------------------------------------------

func TestNormalizeNPWP(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"   ", "", false},
		{"012345678901234", "012345678901234", false},   // 15 digits
		{"0123456789012345", "0123456789012345", false}, // 16 digits
		{"01.234.567.8-901.000", "012345678901000", false},
		{" 01 234 567 8 901 000 ", "012345678901000", false},
		{"01234567890123", "", true},    // 14 digits
		{"01234567890123456", "", true}, // 17 digits
		{"01.234.567.8-901.00A", "", true},
		{"abcdefghijklmno", "", true},
		{"0123456789012３４", "", true}, // full-width digits are not ASCII digits
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := normalizeNPWP(tt.in)
			var valErr *ValidationError
			if tt.wantErr {
				if !errors.As(err, &valErr) || valErr.Field != "npwp" {
					t.Fatalf("normalizeNPWP(%q) err = %v, want *ValidationError{npwp}", tt.in, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("normalizeNPWP(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestVendorAdminService_Create_StagesLegalNameAndNormalizedNPWP(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(&fakeVendorAdminRepo{}, sub)

	_, err := svc.Create(context.Background(), 7, CreateVendorRequest{
		Code: "ACM", Name: "Acme", LegalName: "  PT Acme Sejahtera ", NPWP: "01.234.567.8-901.000",
	}, "ip")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got := sub.lastRequest.Payload.(vendorCreatePayload)
	if got.LegalName != "PT Acme Sejahtera" || got.NPWP != "012345678901000" {
		t.Errorf("payload legal_name=%q npwp=%q, want trimmed name and digits-only NPWP", got.LegalName, got.NPWP)
	}
}

func TestVendorAdminService_Create_InvalidNPWP_StagesNothing(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	_, err := NewVendorAdminService(&fakeVendorAdminRepo{}, sub).Create(context.Background(), 1, CreateVendorRequest{Code: "ACM", Name: "Acme", NPWP: "123"}, "ip")
	var valErr *ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "npwp" || sub.submitCalled {
		t.Fatalf("err = %v submitCalled=%v, want *ValidationError{npwp} and nothing staged", err, sub.submitCalled)
	}
}

// PUT is a full overwrite for most fields, but legal_name/npwp are tri-state so
// a client that predates them (sends neither) cannot wipe them.
func TestVendorAdminService_Update_LegalNameAndNPWP_TriState(t *testing.T) {
	existing := activeVendor(1, "ACM")
	existing.LegalName, existing.Npwp = sp("PT Lama"), sp("012345678901000")

	tests := []struct {
		name      string
		legal     *string
		npwp      *string
		wantLegal string
		wantNPWP  string
	}{
		{"omitted keeps both", nil, nil, "PT Lama", "012345678901000"},
		{"blank clears both", sp(""), sp("  "), "", ""},
		{"value replaces both (npwp normalized)", sp(" PT Baru "), sp("99.999.999.9-999.999"), "PT Baru", "999999999999999"},
		{"only npwp sent, legal_name kept", nil, sp("0123456789012345"), "PT Lama", "0123456789012345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return existing }), sub)

			if _, err := svc.Update(context.Background(), 7, 1, UpdateVendorRequest{Name: "Acme", LegalName: tt.legal, NPWP: tt.npwp}, "ip"); err != nil {
				t.Fatalf("Update: %v", err)
			}
			got := sub.lastRequest.Payload.(vendorUpdatePayload)
			if got.LegalName != tt.wantLegal || got.NPWP != tt.wantNPWP {
				t.Errorf("payload legal_name=%q npwp=%q, want %q / %q", got.LegalName, got.NPWP, tt.wantLegal, tt.wantNPWP)
			}
		})
	}
}

func TestVendorAdminService_Update_InvalidNPWP_StagesNothing(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewVendorAdminService(vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return activeVendor(id, "ACM") }), sub)

	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Name: "Acme", NPWP: sp("12345")}, "ip")

	var valErr *ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "npwp" || sub.submitCalled {
		t.Fatalf("err = %v submitCalled=%v, want *ValidationError{npwp} and nothing staged", err, sub.submitCalled)
	}
}

// A vendor change staged before T4.3 has a "before" snapshot without the
// legal_name/npwp keys. It must compare as NOT equal to the current (new-shape)
// row, so ApproveMasterData marks it stale instead of applying an update whose
// payload would null those columns.
func TestVendorSnapshotPreT43_IsStaleNotApplied(t *testing.T) {
	preT43 := []byte(`{"id":1,"code":"ACM","name":"Vendor ACM","contact_email":null,"contact_phone":null,"hq_address":null,"is_active":true,"deleted_at":null}`)

	equal, err := statesEqual(preT43, activeVendor(1, "ACM"))
	if err != nil {
		t.Fatal(err)
	}
	if equal {
		t.Error("a pre-T4.3 snapshot must not equal the current row: it has to be marked stale, not applied")
	}

	// Sanity: a snapshot captured from the current shape does match.
	raw, _ := json.Marshal(activeVendor(1, "ACM"))
	if equal, err := statesEqual(raw, activeVendor(1, "ACM")); err != nil || !equal {
		t.Errorf("a current-shape snapshot must equal itself, got equal=%v err=%v", equal, err)
	}
}

// --- List / Count / Get pass-throughs (Req 6, 9.5 -- read-only, no audit) -

func TestVendorAdminService_List_PassesThroughToRepo(t *testing.T) {
	want := []db.ListVendorsAdminRow{{ID: 1, Code: "ACM"}}
	repo := &fakeVendorAdminRepo{listFunc: func(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
		if arg.Status != "active" {
			t.Errorf("Status = %q, want active", arg.Status)
		}
		return want, nil
	}}

	got, err := NewVendorAdminService(repo, &fakeVendorBranchSubmitter{}).List(context.Background(), db.ListVendorsAdminParams{Status: "active"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("List = %+v, want %+v", got, want)
	}
}

func TestVendorAdminService_Count_PassesThroughToRepo(t *testing.T) {
	repo := &fakeVendorAdminRepo{countFunc: func(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error) { return 42, nil }}

	got, err := NewVendorAdminService(repo, &fakeVendorBranchSubmitter{}).Count(context.Background(), db.CountVendorsAdminParams{Status: "all"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 42 {
		t.Errorf("Count = %d, want 42", got)
	}
}

func TestVendorAdminService_Get_PassesThroughToRepo(t *testing.T) {
	repo := vendorRepoWith(func(id int64) *db.GetVendorAdminByIDRow { return activeVendor(id, "ACM") })
	svc := NewVendorAdminService(repo, &fakeVendorBranchSubmitter{})

	got, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.ID != 1 {
		t.Errorf("Get = %+v, want id=1", got)
	}

	repo.getByIDFunc = func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) { return nil, nil }
	got, err = svc.Get(context.Background(), 999)
	if err != nil {
		t.Fatalf("Get(missing): %v", err)
	}
	if got != nil {
		t.Errorf("Get(missing) = %+v, want nil", got)
	}
}
