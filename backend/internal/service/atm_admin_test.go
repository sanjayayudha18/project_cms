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

// fakeATMAdminRepo is read-only, like ATMAdminRepo itself: the service has no
// write path to the atms table (T4.2), so the only side effects to observe
// are the change requests the fake Submitter receives.
type fakeATMAdminRepo struct {
	listFunc           func(ctx context.Context, arg db.ListATMsAdminParams) ([]db.ListATMsAdminRow, error)
	countFunc          func(ctx context.Context, arg db.CountATMsAdminParams) (int64, error)
	getByIDFunc        func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error)
	findByTerminalIDFn func(ctx context.Context, terminalID string) (*int64, error)
	listLocationsFunc  func(ctx context.Context) ([]db.ListLocationsForSelectRow, error)
	locationExistsFunc func(ctx context.Context, locationID int64) (bool, error)
}

func (f *fakeATMAdminRepo) List(ctx context.Context, arg db.ListATMsAdminParams) ([]db.ListATMsAdminRow, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx, arg)
	}
	return nil, nil
}

func (f *fakeATMAdminRepo) Count(ctx context.Context, arg db.CountATMsAdminParams) (int64, error) {
	if f.countFunc != nil {
		return f.countFunc(ctx, arg)
	}
	return 0, nil
}

func (f *fakeATMAdminRepo) GetByID(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
	if f.getByIDFunc != nil {
		return f.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (f *fakeATMAdminRepo) FindByTerminalID(ctx context.Context, terminalID string) (*int64, error) {
	if f.findByTerminalIDFn != nil {
		return f.findByTerminalIDFn(ctx, terminalID)
	}
	return nil, nil
}

func (f *fakeATMAdminRepo) ListLocations(ctx context.Context) ([]db.ListLocationsForSelectRow, error) {
	if f.listLocationsFunc != nil {
		return f.listLocationsFunc(ctx)
	}
	return nil, nil
}

func (f *fakeATMAdminRepo) LocationExists(ctx context.Context, locationID int64) (bool, error) {
	if f.locationExistsFunc != nil {
		return f.locationExistsFunc(ctx, locationID)
	}
	return true, nil
}

func activeATM(id int64, terminalID string) *db.GetATMAdminByIDRow {
	return &db.GetATMAdminByIDRow{
		ID: id, TerminalID: terminalID, LocationID: 10, MachineType: "ATM", Brand: "NCR",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite", IsActive: true,
	}
}

func disabledATM(id int64, terminalID string) *db.GetATMAdminByIDRow {
	row := activeATM(id, terminalID)
	row.IsActive = false
	row.DeletedAt = pgtype.Timestamptz{Valid: true}
	return row
}

func atmRepoWith(row func(id int64) *db.GetATMAdminByIDRow) *fakeATMAdminRepo {
	return &fakeATMAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) { return row(id), nil }}
}

func validCreateATMRequest() CreateATMRequest {
	return CreateATMRequest{
		TerminalID: "TATM001", LocationID: 10, MachineType: "ATM", Brand: "NCR",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite",
	}
}

func validUpdateATMRequest() UpdateATMRequest {
	return UpdateATMRequest{
		LocationID: 10, MachineType: "ATM", Brand: "NCR", Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite",
	}
}

// --- Create: validation ------------------------------------------------

func TestATMAdminService_Create_Validation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *CreateATMRequest)
		wantErr string
	}{
		{"missing terminal_id", func(r *CreateATMRequest) { r.TerminalID = "" }, "terminal_id"},
		{"blank terminal_id (whitespace)", func(r *CreateATMRequest) { r.TerminalID = "   " }, "terminal_id"},
		{"missing location_id", func(r *CreateATMRequest) { r.LocationID = 0 }, "location_id"},
		{"missing machine_type", func(r *CreateATMRequest) { r.MachineType = "" }, "machine_type"},
		{"missing brand", func(r *CreateATMRequest) { r.Brand = "" }, "brand"},
		{"missing model", func(r *CreateATMRequest) { r.Model = "" }, "model"},
		{"missing operation_hours", func(r *CreateATMRequest) { r.OperationHours = "" }, "operation_hours"},
		{"missing deployment_type", func(r *CreateATMRequest) { r.DeploymentType = "" }, "deployment_type"},
		{"invalid priority_class", func(r *CreateATMRequest) { r.PriorityClass = sp("Gold") }, "priority_class"},
		{"non-numeric capacity_amount", func(r *CreateATMRequest) { r.CapacityAmount = sp("not-a-number") }, "capacity_amount"},
		{"fraction syntax capacity_amount", func(r *CreateATMRequest) { r.CapacityAmount = sp("1/2") }, "capacity_amount"},
		{"negative low_threshold_amount", func(r *CreateATMRequest) { r.LowThresholdAmount = sp("-1.00") }, "low_threshold_amount"},
		{"negative critical_threshold_amount", func(r *CreateATMRequest) { r.CriticalThresholdAmount = sp("-0.01") }, "critical_threshold_amount"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateATMRequest()
			tt.mutate(&req)
			sub := &fakeVendorBranchSubmitter{}

			_, err := NewATMAdminService(&fakeATMAdminRepo{}, sub).Create(context.Background(), 1, req, "10.0.0.1")

			var valErr *ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("Create(%+v) err = %v, want *ValidationError", req, err)
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

func TestATMAdminService_Create_ValidPriorityClasses(t *testing.T) {
	for _, pc := range []string{"VIP", "Non VIP", "Industri"} {
		t.Run(pc, func(t *testing.T) {
			req := validCreateATMRequest()
			req.PriorityClass = &pc
			sub := &fakeVendorBranchSubmitter{}

			if _, err := NewATMAdminService(&fakeATMAdminRepo{}, sub).Create(context.Background(), 1, req, "10.0.0.1"); err != nil {
				t.Fatalf("Create with priority_class=%q: %v", pc, err)
			}
			if got := sub.lastRequest.Payload.(atmCreatePayload).PriorityClass; got == nil || *got != pc {
				t.Errorf("staged priority_class = %v, want %q", got, pc)
			}
		})
	}
}

// Money travels as the exact decimal string the caller sent (trimmed), never
// via float, and blank optionals become NULL.
func TestATMAdminService_Create_MoneyStaysExactAndBlanksBecomeNil(t *testing.T) {
	amount := " 1234567890123456.78 " // up to numeric(20,2)
	req := validCreateATMRequest()
	req.CapacityAmount = &amount
	req.LowThresholdAmount = sp("   ")
	req.EscrowAccount = sp("  ")
	sub := &fakeVendorBranchSubmitter{}

	if _, err := NewATMAdminService(&fakeATMAdminRepo{}, sub).Create(context.Background(), 1, req, "10.0.0.1"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	p := sub.lastRequest.Payload.(atmCreatePayload)
	if p.CapacityAmount == nil || *p.CapacityAmount != "1234567890123456.78" {
		t.Errorf("CapacityAmount = %v, want exact 1234567890123456.78", p.CapacityAmount)
	}
	if p.LowThresholdAmount != nil || p.EscrowAccount != nil || p.CriticalThresholdAmount != nil {
		t.Errorf("blank/absent optionals must be nil, got low=%v escrow=%v critical=%v", p.LowThresholdAmount, p.EscrowAccount, p.CriticalThresholdAmount)
	}
}

// --- Create: location reference -----------------------------------------

func TestATMAdminService_Create_InvalidLocationReference(t *testing.T) {
	repo := &fakeATMAdminRepo{locationExistsFunc: func(ctx context.Context, locationID int64) (bool, error) { return false, nil }}
	sub := &fakeVendorBranchSubmitter{}

	_, err := NewATMAdminService(repo, sub).Create(context.Background(), 1, validCreateATMRequest(), "10.0.0.1")

	if !errors.Is(err, ErrATMInvalidReference) {
		t.Fatalf("err = %v, want ErrATMInvalidReference", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite an invalid location reference")
	}
}

// --- Create: terminal_id conflict ---------------------------------------

func TestATMAdminService_Create_TerminalIDConflict_StagesNothing(t *testing.T) {
	existingID := int64(42)
	repo := &fakeATMAdminRepo{findByTerminalIDFn: func(ctx context.Context, terminalID string) (*int64, error) { return &existingID, nil }}
	sub := &fakeVendorBranchSubmitter{}

	_, err := NewATMAdminService(repo, sub).Create(context.Background(), 1, validCreateATMRequest(), "10.0.0.1")

	if !errors.Is(err, ErrATMTerminalIDConflict) {
		t.Fatalf("err = %v, want ErrATMTerminalIDConflict", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite a terminal_id conflict")
	}
}

// --- Create: staged ----------------------------------------------------

func TestATMAdminService_Create_StagesPayload(t *testing.T) {
	req := validCreateATMRequest()
	req.TerminalID = "  TATM001 "
	req.Brand = " NCR "
	req.Blacklisted = true
	sub := &fakeVendorBranchSubmitter{}

	change, err := NewATMAdminService(&fakeATMAdminRepo{}, sub).Create(context.Background(), 7, req, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if change.ID == 0 || change.Status != "pending" {
		t.Errorf("want the staged pending change returned, got %+v", change)
	}
	if sub.lastRequest.EntityType != "atm" || sub.lastRequest.Op != "create" || sub.lastRequest.EntityID != nil {
		t.Errorf("unexpected submit: %+v", sub.lastRequest)
	}
	p, ok := sub.lastRequest.Payload.(atmCreatePayload)
	if !ok || p.TerminalID != "TATM001" || p.Brand != "NCR" || !p.Blacklisted || p.LocationID != 10 {
		t.Errorf("payload = %+v, want trimmed terminal_id/brand, blacklisted, location 10", sub.lastRequest.Payload)
	}

	// What the applier will unmarshal: flat snake_case keys incl. terminal_id.
	raw, _ := json.Marshal(sub.lastRequest.Payload)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"terminal_id", "location_id", "machine_type", "brand", "blacklisted", "capacity_amount"} {
		if _, present := m[k]; !present {
			t.Errorf("create payload JSON missing key %q: %s", k, raw)
		}
	}
}

func TestATMAdminService_Create_SubmitErrorPropagates(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{submitFunc: func(context.Context, int64, SubmitRequest, string) (db.MasterDataChangeRequest, error) {
		return db.MasterDataChangeRequest{}, ErrMasterDataForbidden
	}}
	_, err := NewATMAdminService(&fakeATMAdminRepo{}, sub).Create(context.Background(), 1, validCreateATMRequest(), "ip")
	if !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("err = %v, want ErrMasterDataForbidden passed through", err)
	}
}

// --- Update -------------------------------------------------------------

func TestATMAdminService_Update_NotFound(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeATMAdminRepo
	}{
		{"missing id", &fakeATMAdminRepo{}},
		{"soft-disabled target", atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return disabledATM(id, "TATM001") })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			_, err := NewATMAdminService(tt.repo, sub).Update(context.Background(), 1, 99, validUpdateATMRequest(), "10.0.0.1")
			if !errors.Is(err, ErrATMNotFound) {
				t.Fatalf("err = %v, want ErrATMNotFound", err)
			}
			if sub.submitCalled {
				t.Error("a change request was staged despite a not-found target")
			}
		})
	}
}

func TestATMAdminService_Update_ImmutableTerminalIDGuard(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewATMAdminService(atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return activeATM(id, "TATM001") }), sub)

	req := validUpdateATMRequest()
	req.TerminalID = sp("DIFFERENT")
	_, err := svc.Update(context.Background(), 1, 1, req, "10.0.0.1")

	if !errors.Is(err, ErrATMTerminalIDImmutable) {
		t.Fatalf("err = %v, want ErrATMTerminalIDImmutable", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite an immutable-terminal_id change attempt")
	}
}

func TestATMAdminService_Update_SameOrOmittedTerminalIDIsAllowed(t *testing.T) {
	for name, tid := range map[string]*string{"same (padded)": sp(" TATM001 "), "omitted": nil} {
		t.Run(name, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			svc := NewATMAdminService(atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return activeATM(id, "TATM001") }), sub)

			req := validUpdateATMRequest()
			req.TerminalID = tid
			if _, err := svc.Update(context.Background(), 1, 1, req, "10.0.0.1"); err != nil {
				t.Fatalf("Update: %v", err)
			}
			if !sub.submitCalled {
				t.Error("expected a change request to be staged")
			}
		})
	}
}

func TestATMAdminService_Update_Validation(t *testing.T) {
	svc := NewATMAdminService(atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return activeATM(id, "TATM001") }), &fakeVendorBranchSubmitter{})

	req := validUpdateATMRequest()
	req.Brand = ""
	_, err := svc.Update(context.Background(), 1, 1, req, "10.0.0.1")
	var valErr *ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "brand" {
		t.Fatalf("err = %v, want *ValidationError{Field: brand}", err)
	}

	req = validUpdateATMRequest()
	req.CapacityAmount = sp("-5")
	_, err = svc.Update(context.Background(), 1, 1, req, "10.0.0.1")
	if !errors.As(err, &valErr) || valErr.Field != "capacity_amount" {
		t.Fatalf("err = %v, want *ValidationError{Field: capacity_amount}", err)
	}
}

func TestATMAdminService_Update_InvalidLocationReference(t *testing.T) {
	repo := atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return activeATM(id, "TATM001") })
	repo.locationExistsFunc = func(ctx context.Context, locationID int64) (bool, error) { return false, nil }
	sub := &fakeVendorBranchSubmitter{}

	_, err := NewATMAdminService(repo, sub).Update(context.Background(), 1, 1, validUpdateATMRequest(), "10.0.0.1")

	if !errors.Is(err, ErrATMInvalidReference) {
		t.Fatalf("err = %v, want ErrATMInvalidReference", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged despite an invalid location reference")
	}
}

// The "before" snapshot is what the T2.5 staleness check compares against, so
// it must be the ATM row exactly as loaded; the staged payload must not carry
// terminal_id (immutable -- it is written on create only).
func TestATMAdminService_Update_StagesPayloadWithBeforeSnapshot(t *testing.T) {
	before := activeATM(1, "TATM001")
	sub := &fakeVendorBranchSubmitter{}
	svc := NewATMAdminService(atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return before }), sub)

	req := validUpdateATMRequest()
	req.Brand = " Diebold "
	req.CapacityAmount = sp("900000000.50")
	if _, err := svc.Update(context.Background(), 7, 1, req, "10.0.0.1"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := sub.lastRequest
	if got.EntityType != "atm" || got.Op != "update" || got.EntityID == nil || *got.EntityID != 1 {
		t.Errorf("unexpected submit: %+v", got)
	}
	if got.Before != before {
		t.Errorf("Before = %+v, want the loaded ATM row", got.Before)
	}
	p, ok := got.Payload.(atmUpdatePayload)
	if !ok || p.Brand != "Diebold" || p.CapacityAmount == nil || *p.CapacityAmount != "900000000.50" {
		t.Errorf("payload = %+v, want trimmed brand and exact capacity", got.Payload)
	}
	raw, _ := json.Marshal(got.Payload)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if _, present := m["terminal_id"]; present {
		t.Errorf("update payload must not carry terminal_id: %s", raw)
	}
}

// --- Disable / Enable -------------------------------------------------

func TestATMAdminService_DisableEnable_NotFound_StagesNothing(t *testing.T) {
	sub := &fakeVendorBranchSubmitter{}
	svc := NewATMAdminService(&fakeATMAdminRepo{}, sub)

	if _, err := svc.Disable(context.Background(), 1, 99, "10.0.0.1"); !errors.Is(err, ErrATMNotFound) {
		t.Errorf("Disable err = %v, want ErrATMNotFound", err)
	}
	if _, err := svc.Enable(context.Background(), 1, 99, "10.0.0.1"); !errors.Is(err, ErrATMNotFound) {
		t.Errorf("Enable err = %v, want ErrATMNotFound", err)
	}
	if sub.submitCalled {
		t.Error("a change request was staged for a non-existent id")
	}
}

func TestATMAdminService_DisableEnable_Stage(t *testing.T) {
	tests := []struct {
		op   string
		row  func(id int64) *db.GetATMAdminByIDRow
		call func(s *ATMAdminService) (db.MasterDataChangeRequest, error)
	}{
		{"disable", func(id int64) *db.GetATMAdminByIDRow { return activeATM(id, "TATM001") },
			func(s *ATMAdminService) (db.MasterDataChangeRequest, error) {
				return s.Disable(context.Background(), 7, 1, "10.0.0.1")
			}},
		{"enable", func(id int64) *db.GetATMAdminByIDRow { return disabledATM(id, "TATM001") },
			func(s *ATMAdminService) (db.MasterDataChangeRequest, error) {
				return s.Enable(context.Background(), 7, 1, "10.0.0.1")
			}},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			sub := &fakeVendorBranchSubmitter{}
			change, err := tt.call(NewATMAdminService(atmRepoWith(tt.row), sub))
			if err != nil {
				t.Fatalf("%s: %v", tt.op, err)
			}
			got := sub.lastRequest
			if change.Status != "pending" || got.EntityType != "atm" || got.Op != tt.op || got.EntityID == nil || *got.EntityID != 1 || got.Before == nil {
				t.Errorf("unexpected: change=%+v submit=%+v", change, got)
			}
		})
	}
}

// --- List / Count / Get / ListLocations pass-throughs (Req 6.3, 7.4) ------

func TestATMAdminService_List_PassesThroughToRepo(t *testing.T) {
	loc := "Jakarta Pusat"
	want := []db.ListATMsAdminRow{{ID: 1, TerminalID: "TATM001", LocationName: &loc}}
	repo := &fakeATMAdminRepo{listFunc: func(ctx context.Context, arg db.ListATMsAdminParams) ([]db.ListATMsAdminRow, error) {
		if arg.Status != "active" {
			t.Errorf("Status = %q, want active", arg.Status)
		}
		return want, nil
	}}

	got, err := NewATMAdminService(repo, &fakeVendorBranchSubmitter{}).List(context.Background(), db.ListATMsAdminParams{Status: "active"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].LocationName == nil || *got[0].LocationName != loc {
		t.Errorf("List = %+v, want id=1 location_name=%s", got, loc)
	}
}

func TestATMAdminService_Count_PassesThroughToRepo(t *testing.T) {
	repo := &fakeATMAdminRepo{countFunc: func(ctx context.Context, arg db.CountATMsAdminParams) (int64, error) { return 42, nil }}

	got, err := NewATMAdminService(repo, &fakeVendorBranchSubmitter{}).Count(context.Background(), db.CountATMsAdminParams{Status: "all"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 42 {
		t.Errorf("Count = %d, want 42", got)
	}
}

func TestATMAdminService_Get_NotFound(t *testing.T) {
	_, err := NewATMAdminService(&fakeATMAdminRepo{}, &fakeVendorBranchSubmitter{}).Get(context.Background(), 999)
	if !errors.Is(err, ErrATMNotFound) {
		t.Fatalf("err = %v, want ErrATMNotFound", err)
	}
}

func TestATMAdminService_Get_PassesThroughToRepo(t *testing.T) {
	repo := atmRepoWith(func(id int64) *db.GetATMAdminByIDRow { return activeATM(id, "TATM001") })

	got, err := NewATMAdminService(repo, &fakeVendorBranchSubmitter{}).Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != 1 || got.TerminalID != "TATM001" {
		t.Errorf("Get = %+v, want id=1 terminal_id=TATM001", got)
	}
}

func TestATMAdminService_ListLocations_PassesThroughToRepo(t *testing.T) {
	repo := &fakeATMAdminRepo{listLocationsFunc: func(ctx context.Context) ([]db.ListLocationsForSelectRow, error) {
		return []db.ListLocationsForSelectRow{{ID: 10, Name: "Jakarta Pusat", CityOrRegency: "Jakarta Pusat", Province: "DKI Jakarta"}}, nil
	}}
	sub := &fakeVendorBranchSubmitter{}

	got, err := NewATMAdminService(repo, sub).ListLocations(context.Background())
	if err != nil {
		t.Fatalf("ListLocations: %v", err)
	}
	if len(got) != 1 || got[0].ID != 10 || got[0].Name != "Jakarta Pusat" {
		t.Errorf("ListLocations = %+v, want one option id=10 name=Jakarta Pusat", got)
	}
	if sub.submitCalled {
		t.Error("a read-only ListLocations must not stage anything")
	}
}
