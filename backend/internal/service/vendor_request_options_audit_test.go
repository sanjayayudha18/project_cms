package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// optionsAuditRepo overrides only the reads ListVendorOptions/AuditLog use.
type optionsAuditRepo struct {
	fakeForecastRepo
	vendors  []db.ListActiveVendorsRow
	regions  []*string
	getErr   error
	audit    []db.AuditLog
	auditArg db.ListAuditLogsByEntityParams
	err      error
}

func (f *optionsAuditRepo) ListActiveVendors(context.Context) ([]db.ListActiveVendorsRow, error) {
	return f.vendors, f.err
}
func (f *optionsAuditRepo) ListDistinctVendorBranchRegions(context.Context) ([]*string, error) {
	return f.regions, nil
}
func (f *optionsAuditRepo) GetVendorRequest(context.Context, int64) (db.VendorRequest, error) {
	return db.VendorRequest{}, f.getErr
}
func (f *optionsAuditRepo) ListAuditLogsByEntity(_ context.Context, p db.ListAuditLogsByEntityParams) ([]db.AuditLog, error) {
	f.auditArg = p
	return f.audit, f.err
}

func TestListVendorOptions_MapsVendorsAndSkipsNullRegions(t *testing.T) {
	jabar := "Jawa Barat"
	svc := &VendorRequestService{read: &optionsAuditRepo{
		vendors: []db.ListActiveVendorsRow{{ID: 1, Name: "TAG"}, {ID: 2, Name: "BRINKS"}},
		regions: []*string{&jabar, nil},
	}}
	got, err := svc.ListVendorOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Vendors) != 2 || got.Vendors[1] != (VendorOption{ID: 2, Name: "BRINKS"}) {
		t.Errorf("vendors = %+v", got.Vendors)
	}
	if len(got.Regions) != 1 || got.Regions[0] != jabar {
		t.Errorf("regions = %v, want [Jawa Barat]", got.Regions)
	}

	svc = &VendorRequestService{read: &optionsAuditRepo{err: errors.New("boom")}}
	if _, err := svc.ListVendorOptions(context.Background()); err == nil {
		t.Error("want error when vendor list fails")
	}
}

func TestAuditLog(t *testing.T) {
	ctx := context.Background()

	t.Run("missing request is ErrNotFound", func(t *testing.T) {
		svc := &VendorRequestService{read: &optionsAuditRepo{getErr: pgx.ErrNoRows}}
		if _, err := svc.AuditLog(ctx, 7); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("other lookup error is not ErrNotFound", func(t *testing.T) {
		svc := &VendorRequestService{read: &optionsAuditRepo{getErr: errors.New("db down")}}
		if _, err := svc.AuditLog(ctx, 7); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want wrapped db error", err)
		}
	})

	t.Run("maps state and metadata", func(t *testing.T) {
		at := time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC)
		repo := &optionsAuditRepo{audit: []db.AuditLog{{
			ID: 5, ActorID: 9, Action: "approve", EntityType: "vendor_request", EntityID: 7,
			Before:    []byte(`{"state":"submitted"}`),
			After:     []byte(`{"state":"approved","reason":"ok"}`),
			CreatedAt: pgtype.Timestamptz{Time: at, Valid: true},
		}}}
		got, err := (&VendorRequestService{read: repo}).AuditLog(ctx, 7)
		if err != nil {
			t.Fatal(err)
		}
		if repo.auditArg != (db.ListAuditLogsByEntityParams{EntityType: "vendor_request", EntityID: 7}) {
			t.Errorf("queried %+v", repo.auditArg)
		}
		e := got[0]
		if len(got) != 1 || e.PerformedBy != 9 || !e.PerformedAt.Equal(at) ||
			e.PreviousState == nil || *e.PreviousState != "submitted" ||
			e.NewState == nil || *e.NewState != "approved" ||
			len(e.Metadata) != 1 || e.Metadata["reason"] != "ok" {
			t.Errorf("entry = %+v", e)
		}
	})
}
