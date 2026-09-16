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

type fakeVendorAdminRepo struct {
	listFunc             func(ctx context.Context, arg db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error)
	countFunc            func(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error)
	getByIDFunc          func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error)
	findByCodeFunc       func(ctx context.Context, code string) (*int64, error)
	createFunc           func(ctx context.Context, arg db.CreateVendorAdminParams) (db.CreateVendorAdminRow, error)
	updateFunc           func(ctx context.Context, arg db.UpdateVendorAdminParams) (db.UpdateVendorAdminRow, error)
	disableFunc          func(ctx context.Context, id int64) error
	enableFunc           func(ctx context.Context, id int64) error
	countActiveUsersFunc func(ctx context.Context, vendorID int64) (int64, error)

	createCalled  bool
	updateCalled  bool
	disableCalled bool
	enableCalled  bool
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

func (f *fakeVendorAdminRepo) Create(ctx context.Context, arg db.CreateVendorAdminParams) (db.CreateVendorAdminRow, error) {
	f.createCalled = true
	if f.createFunc != nil {
		return f.createFunc(ctx, arg)
	}
	return db.CreateVendorAdminRow{ID: 1, Code: arg.Code, Name: arg.Name, ContactEmail: arg.ContactEmail, ContactPhone: arg.ContactPhone, HqAddress: arg.HqAddress, IsActive: true}, nil
}

func (f *fakeVendorAdminRepo) Update(ctx context.Context, arg db.UpdateVendorAdminParams) (db.UpdateVendorAdminRow, error) {
	f.updateCalled = true
	if f.updateFunc != nil {
		return f.updateFunc(ctx, arg)
	}
	return db.UpdateVendorAdminRow{ID: arg.ID, Name: arg.Name, ContactEmail: arg.ContactEmail, ContactPhone: arg.ContactPhone, HqAddress: arg.HqAddress, IsActive: true}, nil
}

func (f *fakeVendorAdminRepo) Disable(ctx context.Context, id int64) error {
	f.disableCalled = true
	if f.disableFunc != nil {
		return f.disableFunc(ctx, id)
	}
	return nil
}

func (f *fakeVendorAdminRepo) Enable(ctx context.Context, id int64) error {
	f.enableCalled = true
	if f.enableFunc != nil {
		return f.enableFunc(ctx, id)
	}
	return nil
}

func (f *fakeVendorAdminRepo) CountActiveUsers(ctx context.Context, vendorID int64) (int64, error) {
	if f.countActiveUsersFunc != nil {
		return f.countActiveUsersFunc(ctx, vendorID)
	}
	return 0, nil
}

type fakeVendorAuditWriter struct {
	calls []audit.Entry
	err   error
}

func (f *fakeVendorAuditWriter) Write(ctx context.Context, entry audit.Entry) error {
	f.calls = append(f.calls, entry)
	return f.err
}

func activeVendor(id int64, code string) *db.GetVendorAdminByIDRow {
	return &db.GetVendorAdminByIDRow{ID: id, Code: code, Name: "Vendor " + code, IsActive: true}
}

func disabledVendor(id int64, code string) *db.GetVendorAdminByIDRow {
	return &db.GetVendorAdminByIDRow{ID: id, Code: code, Name: "Vendor " + code, IsActive: false, DeletedAt: pgtype.Timestamptz{Valid: true}}
}

// fakePgUniqueViolation builds an error isUniqueViolation() recognizes,
// simulating a race the pre-check missed.
func fakePgUniqueViolation() error {
	return &pgconn.PgError{Code: pgUniqueViolation}
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
			repo := &fakeVendorAdminRepo{}
			auditW := &fakeVendorAuditWriter{}
			svc := NewVendorAdminService(repo, auditW)

			_, err := svc.Create(context.Background(), 1, tt.req, "10.0.0.1")
			var valErr *ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("Create(%+v) err = %v, want *ValidationError", tt.req, err)
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

func TestVendorAdminService_Create_CodeConflict(t *testing.T) {
	t.Run("pre-check finds an existing code", func(t *testing.T) {
		existingID := int64(42)
		repo := &fakeVendorAdminRepo{
			findByCodeFunc: func(ctx context.Context, code string) (*int64, error) { return &existingID, nil },
		}
		auditW := &fakeVendorAuditWriter{}
		svc := NewVendorAdminService(repo, auditW)

		_, err := svc.Create(context.Background(), 1, CreateVendorRequest{Code: "ACM", Name: "Acme"}, "10.0.0.1")
		if !errors.Is(err, ErrVendorCodeConflict) {
			t.Fatalf("err = %v, want ErrVendorCodeConflict", err)
		}
		if repo.createCalled {
			t.Error("repo.Create was called despite a pre-check conflict")
		}
		if len(auditW.calls) != 0 {
			t.Error("audit.Write was called despite a conflict")
		}
	})

	t.Run("DB unique violation surfaces on a missed race", func(t *testing.T) {
		repo := &fakeVendorAdminRepo{
			createFunc: func(ctx context.Context, arg db.CreateVendorAdminParams) (db.CreateVendorAdminRow, error) {
				return db.CreateVendorAdminRow{}, fakePgUniqueViolation()
			},
		}
		auditW := &fakeVendorAuditWriter{}
		svc := NewVendorAdminService(repo, auditW)

		_, err := svc.Create(context.Background(), 1, CreateVendorRequest{Code: "ACM", Name: "Acme"}, "10.0.0.1")
		if !errors.Is(err, ErrVendorCodeConflict) {
			t.Fatalf("err = %v, want ErrVendorCodeConflict", err)
		}
		if len(auditW.calls) != 0 {
			t.Error("audit.Write was called despite a conflict")
		}
	})
}

func TestVendorAdminService_Create_Success_AuditsOnce(t *testing.T) {
	repo := &fakeVendorAdminRepo{}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	got, err := svc.Create(context.Background(), 7, CreateVendorRequest{Code: "ACM", Name: "Acme", ContactEmail: "ops@acme.test"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Code != "ACM" {
		t.Errorf("got.Code = %q, want ACM", got.Code)
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	entry := auditW.calls[0]
	if entry.Action != "vendor_created" || entry.EntityType != "vendor" || entry.ActorID != 7 || entry.Before != nil {
		t.Errorf("audit entry = %+v, want action=vendor_created entity_type=vendor actor_id=7 before=nil", entry)
	}
}

func TestVendorAdminService_Create_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeVendorAdminRepo{}
	auditW := &fakeVendorAuditWriter{err: errors.New("audit db down")}
	svc := NewVendorAdminService(repo, auditW)

	_, err := svc.Create(context.Background(), 1, CreateVendorRequest{Code: "ACM", Name: "Acme"}, "10.0.0.1")
	if err == nil {
		t.Fatal("Create: want an error when the audit write fails, got nil")
	}
	if !repo.createCalled {
		t.Error("repo.Create was not called -- the vendor row itself should still have been inserted")
	}
}

// --- Update ---------------------------------------------------------------

func TestVendorAdminService_Update_NotFound(t *testing.T) {
	tests := []struct {
		name    string
		getByID func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error)
	}{
		{"missing id", func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) { return nil, nil }},
		{"soft-disabled target", func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return disabledVendor(id, "ACM"), nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeVendorAdminRepo{getByIDFunc: tt.getByID}
			auditW := &fakeVendorAuditWriter{}
			svc := NewVendorAdminService(repo, auditW)

			_, err := svc.Update(context.Background(), 1, 99, UpdateVendorRequest{Name: "New Name"}, "10.0.0.1")
			if !errors.Is(err, ErrVendorNotFound) {
				t.Fatalf("err = %v, want ErrVendorNotFound", err)
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

func TestVendorAdminService_Update_ImmutableCodeGuard(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return activeVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	newCode := "DIFFERENT"
	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Code: &newCode, Name: "Acme"}, "10.0.0.1")
	if !errors.Is(err, ErrVendorCodeImmutable) {
		t.Fatalf("err = %v, want ErrVendorCodeImmutable", err)
	}
	if repo.updateCalled {
		t.Error("repo.Update was called despite an immutable-code change attempt")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite a rejected update")
	}
}

func TestVendorAdminService_Update_SameCodeInPayloadIsAllowed(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return activeVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	sameCode := "ACM"
	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Code: &sameCode, Name: "Acme Renamed"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Update with unchanged code: %v", err)
	}
	if !repo.updateCalled {
		t.Error("repo.Update was not called")
	}
}

func TestVendorAdminService_Update_Validation(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return activeVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

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

func TestVendorAdminService_Update_Success_AuditsOnceWithBeforeAfter(t *testing.T) {
	before := activeVendor(1, "ACM")
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) { return before, nil },
	}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 7, 1, UpdateVendorRequest{Name: "Acme Renamed"}, "10.0.0.1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	entry := auditW.calls[0]
	if entry.Action != "vendor_updated" || entry.Before == nil || entry.After == nil {
		t.Errorf("audit entry = %+v, want action=vendor_updated with before and after set", entry)
	}
}

func TestVendorAdminService_Update_RepoNoRowsMapsToNotFound(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return activeVendor(id, "ACM"), nil
		},
		updateFunc: func(ctx context.Context, arg db.UpdateVendorAdminParams) (db.UpdateVendorAdminRow, error) {
			return db.UpdateVendorAdminRow{}, pgx.ErrNoRows
		},
	}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Name: "Acme"}, "10.0.0.1")
	if !errors.Is(err, ErrVendorNotFound) {
		t.Fatalf("err = %v, want ErrVendorNotFound", err)
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite the update affecting 0 rows")
	}
}

func TestVendorAdminService_Update_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return activeVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{err: errors.New("audit db down")}
	svc := NewVendorAdminService(repo, auditW)

	_, err := svc.Update(context.Background(), 1, 1, UpdateVendorRequest{Name: "Acme"}, "10.0.0.1")
	if err == nil {
		t.Fatal("Update: want an error when the audit write fails, got nil")
	}
	if !repo.updateCalled {
		t.Error("repo.Update was not called -- the row itself should still have been updated")
	}
}

// --- Disable / Enable -------------------------------------------------

func TestVendorAdminService_Disable_NotFound(t *testing.T) {
	repo := &fakeVendorAdminRepo{}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	_, err := svc.Disable(context.Background(), 1, 99, "10.0.0.1")
	if !errors.Is(err, ErrVendorNotFound) {
		t.Fatalf("err = %v, want ErrVendorNotFound", err)
	}
	if repo.disableCalled {
		t.Error("repo.Disable was called despite a not-found target")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite a not-found target")
	}
}

func TestVendorAdminService_Disable_Success_AuditsOnceAndSurfacesLinkedUsersWarning(t *testing.T) {
	tests := []struct {
		name         string
		linkedActive int64
	}{
		{"no linked active users", 0},
		{"linked active users still present", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeVendorAdminRepo{
				getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
					return activeVendor(id, "ACM"), nil
				},
				countActiveUsersFunc: func(ctx context.Context, vendorID int64) (int64, error) { return tt.linkedActive, nil },
			}
			auditW := &fakeVendorAuditWriter{}
			svc := NewVendorAdminService(repo, auditW)

			result, err := svc.Disable(context.Background(), 7, 1, "10.0.0.1")
			if err != nil {
				t.Fatalf("Disable: %v", err)
			}
			if !repo.disableCalled {
				t.Error("repo.Disable was not called")
			}
			if result.LinkedUsersWarning != tt.linkedActive {
				t.Errorf("LinkedUsersWarning = %d, want %d", result.LinkedUsersWarning, tt.linkedActive)
			}
			if len(auditW.calls) != 1 {
				t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
			}
			if auditW.calls[0].Action != "vendor_deactivated" {
				t.Errorf("audit action = %q, want vendor_deactivated", auditW.calls[0].Action)
			}
		})
	}
}

func TestVendorAdminService_Disable_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return activeVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{err: errors.New("audit db down")}
	svc := NewVendorAdminService(repo, auditW)

	_, err := svc.Disable(context.Background(), 1, 1, "10.0.0.1")
	if err == nil {
		t.Fatal("Disable: want an error when the audit write fails, got nil")
	}
	if !repo.disableCalled {
		t.Error("repo.Disable was not called -- the vendor should still have been disabled")
	}
}

func TestVendorAdminService_Enable_NotFound_NoAudit(t *testing.T) {
	repo := &fakeVendorAdminRepo{}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	err := svc.Enable(context.Background(), 1, 99, "10.0.0.1")
	if !errors.Is(err, ErrVendorNotFound) {
		t.Fatalf("err = %v, want ErrVendorNotFound", err)
	}
	if repo.enableCalled {
		t.Error("repo.Enable was called despite a not-found target")
	}
	if len(auditW.calls) != 0 {
		t.Error("audit.Write was called despite a not-found target -- Enable must never audit a non-existent id")
	}
}

func TestVendorAdminService_Enable_Success_AuditsOnce(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return disabledVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{}
	svc := NewVendorAdminService(repo, auditW)

	if err := svc.Enable(context.Background(), 7, 1, "10.0.0.1"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !repo.enableCalled {
		t.Error("repo.Enable was not called")
	}
	if len(auditW.calls) != 1 {
		t.Fatalf("audit.Write called %d times, want exactly 1", len(auditW.calls))
	}
	if auditW.calls[0].Action != "vendor_reactivated" {
		t.Errorf("audit action = %q, want vendor_reactivated", auditW.calls[0].Action)
	}
}

func TestVendorAdminService_Enable_AuditFailureSurfacesError(t *testing.T) {
	repo := &fakeVendorAdminRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
			return disabledVendor(id, "ACM"), nil
		},
	}
	auditW := &fakeVendorAuditWriter{err: errors.New("audit db down")}
	svc := NewVendorAdminService(repo, auditW)

	err := svc.Enable(context.Background(), 1, 1, "10.0.0.1")
	if err == nil {
		t.Fatal("Enable: want an error when the audit write fails, got nil")
	}
	if !repo.enableCalled {
		t.Error("repo.Enable was not called -- the vendor should still have been enabled")
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
	svc := NewVendorAdminService(repo, &fakeVendorAuditWriter{})

	got, err := svc.List(context.Background(), db.ListVendorsAdminParams{Status: "active"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("List = %+v, want %+v", got, want)
	}
}

func TestVendorAdminService_Count_PassesThroughToRepo(t *testing.T) {
	repo := &fakeVendorAdminRepo{countFunc: func(ctx context.Context, arg db.CountVendorsAdminParams) (int64, error) { return 42, nil }}
	svc := NewVendorAdminService(repo, &fakeVendorAuditWriter{})

	got, err := svc.Count(context.Background(), db.CountVendorsAdminParams{Status: "all"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 42 {
		t.Errorf("Count = %d, want 42", got)
	}
}

func TestVendorAdminService_Get_PassesThroughToRepo(t *testing.T) {
	repo := &fakeVendorAdminRepo{getByIDFunc: func(ctx context.Context, id int64) (*db.GetVendorAdminByIDRow, error) {
		return activeVendor(id, "ACM"), nil
	}}
	svc := NewVendorAdminService(repo, &fakeVendorAuditWriter{})

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
