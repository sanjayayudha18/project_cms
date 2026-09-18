package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// --- fakes ------------------------------------------------------------

type fakeATMAdminRepo struct {
	listFunc           func(ctx context.Context, arg db.ListATMsAdminParams) ([]db.ListATMsAdminRow, error)
	countFunc          func(ctx context.Context, arg db.CountATMsAdminParams) (int64, error)
	getByIDFunc        func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error)
	findByTerminalIDFn func(ctx context.Context, terminalID string) (*int64, error)
	createFunc         func(ctx context.Context, arg db.CreateATMAdminParams) (db.CreateATMAdminRow, error)
	updateFunc         func(ctx context.Context, arg db.UpdateATMAdminParams) (db.UpdateATMAdminRow, error)
	disableFunc        func(ctx context.Context, id int64) error
	enableFunc         func(ctx context.Context, id int64) error
	listLocationsFunc  func(ctx context.Context) ([]db.ListLocationsForSelectRow, error)
	locationExistsFunc func(ctx context.Context, locationID int64) (bool, error)

	createCalled  bool
	updateCalled  bool
	disableCalled bool
	enableCalled  bool
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

func (f *fakeATMAdminRepo) Create(ctx context.Context, arg db.CreateATMAdminParams) (db.CreateATMAdminRow, error) {
	f.createCalled = true
	if f.createFunc != nil {
		return f.createFunc(ctx, arg)
	}
	return db.CreateATMAdminRow{
		ID: 1, TerminalID: arg.TerminalID, LocationID: arg.LocationID, MachineType: arg.MachineType,
		Brand: arg.Brand, Model: arg.Model, OperationHours: arg.OperationHours, DeploymentType: arg.DeploymentType,
		CapacityAmount: arg.CapacityAmount, LowThresholdAmount: arg.LowThresholdAmount,
		CriticalThresholdAmount: arg.CriticalThresholdAmount, Blacklisted: arg.Blacklisted,
		EscrowAccount: arg.EscrowAccount, PriorityClass: arg.PriorityClass, IsActive: true,
	}, nil
}

func (f *fakeATMAdminRepo) Update(ctx context.Context, arg db.UpdateATMAdminParams) (db.UpdateATMAdminRow, error) {
	f.updateCalled = true
	if f.updateFunc != nil {
		return f.updateFunc(ctx, arg)
	}
	return db.UpdateATMAdminRow{
		ID: arg.ID, LocationID: arg.LocationID, MachineType: arg.MachineType, Brand: arg.Brand, Model: arg.Model,
		OperationHours: arg.OperationHours, DeploymentType: arg.DeploymentType, CapacityAmount: arg.CapacityAmount,
		LowThresholdAmount: arg.LowThresholdAmount, CriticalThresholdAmount: arg.CriticalThresholdAmount,
		Blacklisted: arg.Blacklisted, EscrowAccount: arg.EscrowAccount, PriorityClass: arg.PriorityClass, IsActive: true,
	}, nil
}

func (f *fakeATMAdminRepo) Disable(ctx context.Context, id int64) error {
	f.disableCalled = true
	if f.disableFunc != nil {
		return f.disableFunc(ctx, id)
	}
	return nil
}

func (f *fakeATMAdminRepo) Enable(ctx context.Context, id int64) error {
	f.enableCalled = true
	if f.enableFunc != nil {
		return f.enableFunc(ctx, id)
	}
	return nil
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

type fakeATMAuditWriter struct {
	calls []audit.Entry
	err   error
}

func (f *fakeATMAuditWriter) Write(ctx context.Context, entry audit.Entry) error {
	f.calls = append(f.calls, entry)
	return f.err
}

func activeATM(id int64, terminalID string) *db.GetATMAdminByIDRow {
	return &db.GetATMAdminByIDRow{
		ID: id, TerminalID: terminalID, LocationID: 10, MachineType: "ATM", Brand: "NCR",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite", IsActive: true,
	}
}

func disabledATM(id int64, terminalID string) *db.GetATMAdminByIDRow {
	return &db.GetATMAdminByIDRow{
		ID: id, TerminalID: terminalID, LocationID: 10, MachineType: "ATM", Brand: "NCR",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite",
		IsActive: false, DeletedAt: pgtype.Timestamptz{Valid: true},
	}
}

// fakeATMPgUniqueViolation builds an error isUniqueViolation() recognizes,
// simulating a race the pre-check missed.
func fakeATMPgUniqueViolation() error {
	return &pgconn.PgError{Code: pgUniqueViolation}
}

func validCreateATMRequest() CreateATMRequest {
	return CreateATMRequest{
		TerminalID: "TATM001", LocationID: 10, MachineType: "ATM", Brand: "NCR",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite",
	}
}

// --- Create: validation ------------------------------------------------

func TestATMAdminService_Create_Validation(t *testing.T) {
	amount := func(s string) *string { return &s }

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
		{"invalid priority_class", func(r *CreateATMRequest) { pc := "Gold"; r.PriorityClass = &pc }, "priority_class"},
		{"non-numeric capacity_amount", func(r *CreateATMRequest) { r.CapacityAmount = amount("not-a-number") }, "capacity_amount"},
		{"negative low_threshold_amount", func(r *CreateATMRequest) { r.LowThresholdAmount = amount("-1.00") }, "low_threshold_amount"},
		{"negative critical_threshold_amount", func(r *CreateATMRequest) { r.CriticalThresholdAmount = amount("-0.01") }, "critical_threshold_amount"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateATMRequest()
			tt.mutate(&req)

			repo := &fakeATMAdminRepo{}
			auditW := &fakeATMAuditWriter{}
			svc := NewATMAdminService(repo, auditW)

			_, err := svc.Create(context.Background(), 1, req, "10.0.0.1")
			var valErr *ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("Create(%+v) err = %v, want *ValidationError", req, err)
			}
			if valErr.Field != tt.wantErr {
				t.Errorf("ValidationError.Field = %q, want %q", valErr.Field, tt.wantErr)
			}
			if repo.createCalled {
				t.Error("repo.Create was called despite validation failure")
			}
			if len(auditW.calls) != 0 {
				t.Error("audit.Write was called despite validation failure")
			}
		})
	}
}

func TestATMAdminService_Create_ValidPriorityClasses(t *testing.T) {
	for _, pc := range []string{"VIP", "Non VIP", "Industri"} {
		t.Run(pc, func(t *testing.T) {
			req := validCreateATMRequest()
			req.PriorityClass = &pc
			repo := &fakeATMAdminRepo{}
			svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

			_, err := svc.Create(context.Background(), 1, req, "10.0.0.1")
			if err != nil {
				t.Fatalf("Create with priority_class=%q: %v", pc, err)
			}
		})
	}
}

func TestATMAdminService_Create_ValidDecimalAmountsAccepted(t *testing.T) {
	amount := "1234567890123456.78" // up to numeric(20,2)
	req := validCreateATMRequest()
	req.CapacityAmount = &amount
	repo := &fakeATMAdminRepo{}
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	got, err := svc.Create(context.Background(), 1, req, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.CapacityAmount == nil || *got.CapacityAmount != amount {
		t.Errorf("CapacityAmount = %v, want %s", got.CapacityAmount, amount)
	}
}

// --- Create: location reference -----------------------------------------

func TestATMAdminService_Create_InvalidLocationReference(t *testing.T) {
	repo := &fakeATMAdminRepo{
		locationExistsFunc: func(ctx context.Context, locationID int64) (bool, error) { return false, nil },
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	_, err := svc.Create(context.Background(), 1, validCreateATMRequest(), "10.0.0.1")
	if !errors.Is(err, ErrATMInvalidReference) {
		t.Fatalf("err = %v, want ErrATMInvalidReference", err)
	}
	if repo.createCalled {
		t.Error("repo.Create was called despite an invalid location reference")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite an invalid location reference")
	}
}

// --- Create: terminal_id conflict ---------------------------------------

func TestATMAdminService_Create_TerminalIDConflict(t *testing.T) {
	t.Run("pre-check finds an existing terminal_id", func(t *testing.T) {
		existingID := int64(42)
		repo := &fakeATMAdminRepo{
			findByTerminalIDFn: func(ctx context.Context, terminalID string) (*int64, error) { return &existingID, nil },
		}
		auditW := &fakeATMAuditWriter{}
		svc := NewATMAdminService(repo, auditW)

		_, err := svc.Create(context.Background(), 1, validCreateATMRequest(), "10.0.0.1")
		if !errors.Is(err, ErrATMTerminalIDConflict) {
			t.Fatalf("err = %v, want ErrATMTerminalIDConflict", err)
		}
		if repo.createCalled {
			t.Error("repo.Create was called despite a pre-check conflict")
		}
		if len(auditW.calls) != 0 {
			t.Error("audit.Write was called despite a conflict")
		}
	})

	t.Run("DB unique violation surfaces on a missed race", func(t *testing.T) {
		repo := &fakeATMAdminRepo{
			createFunc: func(ctx context.Context, arg db.CreateATMAdminParams) (db.CreateATMAdminRow, error) {
				return db.CreateATMAdminRow{}, fakeATMPgUniqueViolation()
			},
		}
		auditW := &fakeATMAuditWriter{}
		svc := NewATMAdminService(repo, auditW)

		_, err := svc.Create(context.Background(), 1, validCreateATMRequest(), "10.0.0.1")
		if !errors.Is(err, ErrATMTerminalIDConflict) {
			t.Fatalf("err = %v, want ErrATMTerminalIDConflict", err)
		}
		if len(auditW.calls) != 0 {
			t.Error("audit.Write was called despite a conflict")
		}
	})
}

// --- Create: success / audit --------------------------------------------

func TestATMAdminService_Create_Success_AuditsOnce(t *testing.T) {
	repo := &fakeATMAdminRepo{}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	got, err := svc.Create(context.Background(), 7, validCreateATMRequest(), "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.TerminalID != "TATM001" {
		t.Errorf("got.TerminalID = %q, want TATM001", got.TerminalID)
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	entry := auditW.calls[0]
	if entry.Action != "atm_created" || entry.EntityType != "atm" || entry.ActorID != 7 || entry.Before != nil {
		t.Errorf("audit entry = %+v, want action=atm_created entity_type=atm actor_id=7 before=nil", entry)
	}
}

func TestATMAdminService_Create_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeATMAdminRepo{}
	auditW := &fakeATMAuditWriter{err: errors.New("audit db down")}
	svc := NewATMAdminService(repo, auditW)

	_, err := svc.Create(context.Background(), 1, validCreateATMRequest(), "10.0.0.1")
	if err == nil {
		t.Fatal("Create: want an error when the audit write fails, got nil")
	}
	if !repo.createCalled {
		t.Error("repo.Create was not called -- the atm row itself should still have been inserted")
	}
}

// --- Update: not found / immutable terminal_id ---------------------------

func TestATMAdminService_Update_NotFound(t *testing.T) {
	tests := []struct {
		name    string
		getByID func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error)
	}{
		{"missing id", func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) { return nil, nil }},
		{"soft-disabled target", func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return disabledATM(id, "TATM001"), nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeATMAdminRepo{getByIDFunc: tt.getByID}
			auditW := &fakeATMAuditWriter{}
			svc := NewATMAdminService(repo, auditW)

			req := validCreateATMRequest()
			_, err := svc.Update(context.Background(), 1, 99, UpdateATMRequest{
				LocationID: req.LocationID, MachineType: req.MachineType, Brand: req.Brand, Model: req.Model,
				OperationHours: req.OperationHours, DeploymentType: req.DeploymentType,
			}, "10.0.0.1")
			if !errors.Is(err, ErrATMNotFound) {
				t.Fatalf("err = %v, want ErrATMNotFound", err)
			}
			if repo.updateCalled {
				t.Error("repo.Update was called despite a not-found target")
			}
			if len(auditW.calls) != 0 {
				t.Error("audit.Write was called despite a not-found target")
			}
		})
	}
}

func TestATMAdminService_Update_ImmutableTerminalIDGuard(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	newTerminalID := "DIFFERENT"
	_, err := svc.Update(context.Background(), 1, 1, UpdateATMRequest{
		TerminalID: &newTerminalID, LocationID: 10, MachineType: "ATM", Brand: "NCR",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	if !errors.Is(err, ErrATMTerminalIDImmutable) {
		t.Fatalf("err = %v, want ErrATMTerminalIDImmutable", err)
	}
	if repo.updateCalled {
		t.Error("repo.Update was called despite an immutable-terminal_id change attempt")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite a rejected update")
	}
}

func TestATMAdminService_Update_SameTerminalIDInPayloadIsAllowed(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
	}
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	sameTerminalID := "TATM001"
	_, err := svc.Update(context.Background(), 1, 1, UpdateATMRequest{
		TerminalID: &sameTerminalID, LocationID: 10, MachineType: "ATM", Brand: "Diebold",
		Model: "SelfServ", OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Update with unchanged terminal_id: %v", err)
	}
	if !repo.updateCalled {
		t.Error("repo.Update was not called")
	}
}

func TestATMAdminService_Update_Validation(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
	}
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	_, err := svc.Update(context.Background(), 1, 1, UpdateATMRequest{
		LocationID: 10, MachineType: "", Brand: "NCR", Model: "SelfServ",
		OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	var valErr *ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "machine_type" {
		t.Fatalf("err = %v, want *ValidationError{Field: machine_type}", err)
	}
}

func TestATMAdminService_Update_InvalidLocationReference(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
		locationExistsFunc: func(ctx context.Context, locationID int64) (bool, error) { return false, nil },
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateATMRequest{
		LocationID: 999, MachineType: "ATM", Brand: "NCR", Model: "SelfServ",
		OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	if !errors.Is(err, ErrATMInvalidReference) {
		t.Fatalf("err = %v, want ErrATMInvalidReference", err)
	}
	if repo.updateCalled {
		t.Error("repo.Update was called despite an invalid location reference")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite an invalid location reference")
	}
}

func TestATMAdminService_Update_Success_AuditsOnceWithBeforeAfter(t *testing.T) {
	before := activeATM(1, "TATM001")
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) { return before, nil },
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 7, 1, UpdateATMRequest{
		LocationID: 10, MachineType: "ATM", Brand: "Diebold", Model: "SelfServ",
		OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	entry := auditW.calls[0]
	if entry.Action != "atm_updated" || entry.Before == nil || entry.After == nil {
		t.Errorf("audit entry = %+v, want action=atm_updated with before and after set", entry)
	}
}

func TestATMAdminService_Update_RepoNoRowsMapsToNotFound(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
		updateFunc: func(ctx context.Context, arg db.UpdateATMAdminParams) (db.UpdateATMAdminRow, error) {
			return db.UpdateATMAdminRow{}, pgx.ErrNoRows
		},
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateATMRequest{
		LocationID: 10, MachineType: "ATM", Brand: "NCR", Model: "SelfServ",
		OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	if !errors.Is(err, ErrATMNotFound) {
		t.Fatalf("err = %v, want ErrATMNotFound", err)
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite the update affecting 0 rows")
	}
}

func TestATMAdminService_Update_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
	}
	auditW := &fakeATMAuditWriter{err: errors.New("audit db down")}
	svc := NewATMAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateATMRequest{
		LocationID: 10, MachineType: "ATM", Brand: "NCR", Model: "SelfServ",
		OperationHours: "24 Hours", DeploymentType: "Onsite",
	}, "10.0.0.1")
	if err == nil {
		t.Fatal("Update: want an error when the audit write fails, got nil")
	}
	if !repo.updateCalled {
		t.Error("repo.Update was not called -- the row itself should still have been updated")
	}
}

// --- Disable / Enable -----------------------------------------------------

func TestATMAdminService_Disable_NotFound(t *testing.T) {
	repo := &fakeATMAdminRepo{}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	err := svc.Disable(context.Background(), 1, 99, "10.0.0.1")
	if !errors.Is(err, ErrATMNotFound) {
		t.Fatalf("err = %v, want ErrATMNotFound", err)
	}
	if repo.disableCalled {
		t.Error("repo.Disable was called despite a not-found target")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite a not-found target")
	}
}

func TestATMAdminService_Disable_Success_AuditsOnce(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	if err := svc.Disable(context.Background(), 7, 1, "10.0.0.1"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if !repo.disableCalled {
		t.Error("repo.Disable was not called")
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	if auditW.calls[0].Action != "atm_deactivated" {
		t.Errorf("audit action = %q, want atm_deactivated", auditW.calls[0].Action)
	}
}

func TestATMAdminService_Disable_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return activeATM(id, "TATM001"), nil
		},
	}
	auditW := &fakeATMAuditWriter{err: errors.New("audit db down")}
	svc := NewATMAdminService(repo, auditW)

	err := svc.Disable(context.Background(), 1, 1, "10.0.0.1")
	if err == nil {
		t.Fatal("Disable: want an error when the audit write fails, got nil")
	}
	if !repo.disableCalled {
		t.Error("repo.Disable was not called -- the atm should still have been disabled")
	}
}

func TestATMAdminService_Enable_NotFound_NoAudit(t *testing.T) {
	repo := &fakeATMAdminRepo{}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	err := svc.Enable(context.Background(), 1, 99, "10.0.0.1")
	if !errors.Is(err, ErrATMNotFound) {
		t.Fatalf("err = %v, want ErrATMNotFound", err)
	}
	if repo.enableCalled {
		t.Error("repo.Enable was called despite a not-found target")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite a not-found target -- Enable must never audit a non-existent id")
	}
}

func TestATMAdminService_Enable_Success_AuditsOnce(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return disabledATM(id, "TATM001"), nil
		},
	}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	if err := svc.Enable(context.Background(), 7, 1, "10.0.0.1"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !repo.enableCalled {
		t.Error("repo.Enable was not called")
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	if auditW.calls[0].Action != "atm_reactivated" {
		t.Errorf("audit action = %q, want atm_reactivated", auditW.calls[0].Action)
	}
}

func TestATMAdminService_Enable_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeATMAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
			return disabledATM(id, "TATM001"), nil
		},
	}
	auditW := &fakeATMAuditWriter{err: errors.New("audit db down")}
	svc := NewATMAdminService(repo, auditW)

	err := svc.Enable(context.Background(), 1, 1, "10.0.0.1")
	if err == nil {
		t.Fatal("Enable: want an error when the audit write fails, got nil")
	}
	if !repo.enableCalled {
		t.Error("repo.Enable was not called -- the atm should still have been enabled")
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
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	got, err := svc.List(context.Background(), db.ListATMsAdminParams{Status: "active"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].LocationName == nil || *got[0].LocationName != loc {
		t.Errorf("List = %+v, want id=1 location_name=%s", got, loc)
	}
}

func TestATMAdminService_Count_PassesThroughToRepo(t *testing.T) {
	repo := &fakeATMAdminRepo{countFunc: func(ctx context.Context, arg db.CountATMsAdminParams) (int64, error) { return 42, nil }}
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	got, err := svc.Count(context.Background(), db.CountATMsAdminParams{Status: "all"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 42 {
		t.Errorf("Count = %d, want 42", got)
	}
}

func TestATMAdminService_Get_NotFound(t *testing.T) {
	repo := &fakeATMAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) { return nil, nil }}
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	_, err := svc.Get(context.Background(), 999)
	if !errors.Is(err, ErrATMNotFound) {
		t.Fatalf("err = %v, want ErrATMNotFound", err)
	}
}

func TestATMAdminService_Get_PassesThroughToRepo(t *testing.T) {
	repo := &fakeATMAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetATMAdminByIDRow, error) {
		return activeATM(id, "TATM001"), nil
	}}
	svc := NewATMAdminService(repo, &fakeATMAuditWriter{})

	got, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != 1 || got.TerminalID != "TATM001" {
		t.Errorf("Get = %+v, want id=1 terminal_id=TATM001", got)
	}
}

func TestATMAdminService_ListLocations_PassesThroughToRepo_NoAudit(t *testing.T) {
	repo := &fakeATMAdminRepo{listLocationsFunc: func(ctx context.Context) ([]db.ListLocationsForSelectRow, error) {
		return []db.ListLocationsForSelectRow{{ID: 10, Name: "Jakarta Pusat", CityOrRegency: "Jakarta Pusat", Province: "DKI Jakarta"}}, nil
	}}
	auditW := &fakeATMAuditWriter{}
	svc := NewATMAdminService(repo, auditW)

	got, err := svc.ListLocations(context.Background())
	if err != nil {
		t.Fatalf("ListLocations: %v", err)
	}
	if len(got) != 1 || got[0].ID != 10 || got[0].Name != "Jakarta Pusat" {
		t.Errorf("ListLocations = %+v, want one option id=10 name=Jakarta Pusat", got)
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called for a read-only ListLocations")
	}
}
