package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Unit tests for VendorPksAdminService submit paths (spec vendor-pks, task 7.4).

type fakePksRepo struct {
	vendor    *db.GetVendorPksVendorRow
	active    *db.VendorPk
	byID      *db.VendorPk
	pending   *db.GetPendingVendorPksChangeRow
	pendingEr error
}

func (f *fakePksRepo) Vendor(context.Context, int64) (*db.GetVendorPksVendorRow, error) {
	return f.vendor, nil
}
func (f *fakePksRepo) Active(context.Context, int64) (*db.VendorPk, error) { return f.active, nil }
func (f *fakePksRepo) GetByID(context.Context, int64) (*db.VendorPk, error) {
	return f.byID, nil
}
func (f *fakePksRepo) ListByVendor(context.Context, int64) ([]db.ListVendorPksByVendorRow, error) {
	return nil, nil
}
func (f *fakePksRepo) Pending(context.Context, int64) (*db.GetPendingVendorPksChangeRow, error) {
	return f.pending, f.pendingEr
}
func (f *fakePksRepo) Status(context.Context, db.GetVendorPksStatusParams) (string, error) {
	return "", nil
}

type fakePksSubmitter struct {
	calls []SubmitRequest
}

func (f *fakePksSubmitter) Submit(_ context.Context, _ int64, req SubmitRequest, _ string) (db.MasterDataChangeRequest, error) {
	f.calls = append(f.calls, req)
	return db.MasterDataChangeRequest{ID: 1}, nil
}

func okVendor() *db.GetVendorPksVendorRow {
	return &db.GetVendorPksVendorRow{ID: 7, Kind: "FLM_VENDOR"}
}

func newPksSvc(repo *fakePksRepo) (*VendorPksAdminService, *fakePksSubmitter) {
	sub := &fakePksSubmitter{}
	return NewVendorPksAdminService(repo, sub, 9), sub
}

func TestVendorPksSubmitNew_Validation(t *testing.T) {
	valid := SubmitVendorPksRequest{PksName: "PKS 2026", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01"}
	with := func(f func(*SubmitVendorPksRequest)) SubmitVendorPksRequest { r := valid; f(&r); return r }

	tests := []struct {
		name      string
		req       SubmitVendorPksRequest
		wantField string // "" = accepted
	}{
		{"valid", valid, ""},
		{"empty name", with(func(r *SubmitVendorPksRequest) { r.PksName = "   " }), "pks_name"},
		{"name 500 ok", with(func(r *SubmitVendorPksRequest) { r.PksName = strings.Repeat("a", 500) }), ""},
		{"name 501", with(func(r *SubmitVendorPksRequest) { r.PksName = strings.Repeat("a", 501) }), "pks_name"},
		{"name 501 runes", with(func(r *SubmitVendorPksRequest) { r.PksName = strings.Repeat("é", 501) }), "pks_name"},
		{"effective empty", with(func(r *SubmitVendorPksRequest) { r.EffectiveDate = "" }), "effective_date"},
		{"effective bad format", with(func(r *SubmitVendorPksRequest) { r.EffectiveDate = "01-01-2026" }), "effective_date"},
		{"expiry bad format", with(func(r *SubmitVendorPksRequest) { r.ExpiryDate = "2027/01/01" }), "expiry_date"},
		{"expiry impossible date", with(func(r *SubmitVendorPksRequest) { r.ExpiryDate = "2027-02-30" }), "expiry_date"},
		{"expiry before effective", with(func(r *SubmitVendorPksRequest) { r.ExpiryDate = "2025-12-31" }), "expiry_date"},
		{"expiry equals effective", with(func(r *SubmitVendorPksRequest) { r.ExpiryDate = "2026-01-01" }), ""},
		{"past dates accepted", with(func(r *SubmitVendorPksRequest) { r.EffectiveDate, r.ExpiryDate = "2000-01-01", "2001-01-01" }), ""},
		{"future dates accepted", with(func(r *SubmitVendorPksRequest) { r.EffectiveDate, r.ExpiryDate = "2090-01-01", "2095-01-01" }), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sub := newPksSvc(&fakePksRepo{vendor: okVendor()})
			_, err := svc.SubmitNew(context.Background(), 1, 7, tt.req, "127.0.0.1")
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(sub.calls) != 1 {
					t.Fatalf("Submit calls = %d, want 1", len(sub.calls))
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("err = %v, want ValidationError on %q", err, tt.wantField)
			}
			if len(sub.calls) != 0 {
				t.Fatalf("Submit must not be called on validation error")
			}
		})
	}
}

func TestVendorPksSubmitNew_VendorChecks(t *testing.T) {
	tests := []struct {
		name    string
		vendor  *db.GetVendorPksVendorRow
		pending *db.GetPendingVendorPksChangeRow
		check   func(error) bool
	}{
		{"missing vendor 404", nil, nil, func(e error) bool { return errors.Is(e, ErrVendorNotFound) }},
		{"internal vendor 422", &db.GetVendorPksVendorRow{ID: 7, Kind: "INTERNAL"}, nil,
			func(e error) bool { return errors.Is(e, ErrVendorPksInternalVendor) }},
		{"inactive vendor 409", &db.GetVendorPksVendorRow{ID: 7, Kind: "FLM_VENDOR", DeletedAt: pgtype.Timestamptz{Valid: true}}, nil,
			func(e error) bool { return errors.Is(e, ErrVendorPksVendorInactive) }},
		{"pending change 409", okVendor(), &db.GetPendingVendorPksChangeRow{ID: 3},
			func(e error) bool { return errors.Is(e, ErrMasterDataChangePending) }},
	}
	req := SubmitVendorPksRequest{PksName: "P", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sub := newPksSvc(&fakePksRepo{vendor: tt.vendor, pending: tt.pending})
			_, err := svc.SubmitNew(context.Background(), 1, 7, req, "")
			if !tt.check(err) {
				t.Fatalf("unexpected err: %v", err)
			}
			if len(sub.calls) != 0 {
				t.Fatal("Submit must not be called")
			}
		})
	}
}

func TestVendorPksSubmitNew_ReplacesPksID(t *testing.T) {
	req := SubmitVendorPksRequest{PksName: " P ", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01"}

	t.Run("no active → nil", func(t *testing.T) {
		svc, sub := newPksSvc(&fakePksRepo{vendor: okVendor()})
		if _, err := svc.SubmitNew(context.Background(), 1, 7, req, ""); err != nil {
			t.Fatal(err)
		}
		got := sub.calls[0]
		p := got.Payload.(VendorPksCreatePayload)
		if got.EntityType != "vendor_pks" || got.Op != "create" || got.EntityID != nil {
			t.Fatalf("bad request: %+v", got)
		}
		if p.ReplacesPksID != nil || p.PksName != "P" || p.VendorID != 7 {
			t.Fatalf("bad payload: %+v", p)
		}
	})

	t.Run("active → its id (even when terminated)", func(t *testing.T) {
		active := &db.VendorPk{ID: 42, VendorID: 7, TerminatedAt: pgtype.Timestamptz{Valid: true}}
		svc, sub := newPksSvc(&fakePksRepo{vendor: okVendor(), active: active})
		if _, err := svc.SubmitNew(context.Background(), 1, 7, req, ""); err != nil {
			t.Fatal(err)
		}
		p := sub.calls[0].Payload.(VendorPksCreatePayload)
		if p.ReplacesPksID == nil || *p.ReplacesPksID != 42 {
			t.Fatalf("ReplacesPksID = %v, want 42", p.ReplacesPksID)
		}
	})
}

func TestVendorPksSubmitReason(t *testing.T) {
	terminated := pgtype.Timestamptz{Valid: true}
	activeRow := &db.VendorPk{ID: 5, VendorID: 7, PksName: "PKS A"}
	termRow := &db.VendorPk{ID: 5, VendorID: 7, PksName: "PKS A", TerminatedAt: terminated}

	tests := []struct {
		name      string
		terminate bool
		pks       *db.VendorPk
		reason    string
		pending   *db.GetPendingVendorPksChangeRow
		wantErr   error // nil = ok
		wantField string
		wantOp    string
	}{
		{"terminate ok", true, activeRow, " salah input ", nil, nil, "", "disable"},
		{"unterminate ok", false, termRow, "batal", nil, nil, "", "enable"},
		{"pks not found", true, nil, "x", nil, ErrVendorPksNotFound, "", ""},
		{"pks other vendor", true, &db.VendorPk{ID: 5, VendorID: 99}, "x", nil, ErrVendorPksNotFound, "", ""},
		{"pks replaced", true, &db.VendorPk{ID: 5, VendorID: 7, ReplacedAt: terminated}, "x", nil, ErrVendorPksNotActive, "", ""},
		{"unterminate replaced", false, &db.VendorPk{ID: 5, VendorID: 7, ReplacedAt: terminated, TerminatedAt: terminated}, "x", nil, ErrVendorPksNotActive, "", ""},
		{"terminate twice", true, termRow, "x", nil, ErrVendorPksAlreadyTerminated, "", ""},
		{"unterminate not terminated", false, activeRow, "x", nil, ErrVendorPksNotTerminated, "", ""},
		{"empty reason", true, activeRow, "   ", nil, nil, "reason", ""},
		{"reason 501", true, activeRow, strings.Repeat("a", 501), nil, nil, "reason", ""},
		{"reason 500 ok", true, activeRow, strings.Repeat("a", 500), nil, nil, "", "disable"},
		{"pending", true, activeRow, "x", &db.GetPendingVendorPksChangeRow{ID: 1}, ErrMasterDataChangePending, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sub := newPksSvc(&fakePksRepo{vendor: okVendor(), byID: tt.pks, pending: tt.pending})
			var err error
			if tt.terminate {
				_, err = svc.SubmitTerminate(context.Background(), 1, 7, 5, tt.reason, "ip")
			} else {
				_, err = svc.SubmitUnterminate(context.Background(), 1, 7, 5, tt.reason, "ip")
			}

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.wantField != "":
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Field != tt.wantField {
					t.Fatalf("err = %v, want ValidationError on %q", err, tt.wantField)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(sub.calls) != 1 {
					t.Fatalf("Submit calls = %d, want 1", len(sub.calls))
				}
				c := sub.calls[0]
				if c.Op != tt.wantOp || c.EntityType != "vendor_pks" || c.EntityID == nil || *c.EntityID != 5 {
					t.Fatalf("bad request: %+v", c)
				}
				// before = snapshot of the PKS row.
				if b, ok := c.Before.(db.VendorPk); !ok || b.ID != 5 || b.PksName != "PKS A" {
					t.Fatalf("Before = %#v, want snapshot of the row", c.Before)
				}
				p := c.Payload.(VendorPksReasonPayload)
				if p.VendorID != 7 || p.Reason != strings.TrimSpace(tt.reason) {
					t.Fatalf("bad payload: %+v", p)
				}
				return
			}
			if len(sub.calls) != 0 {
				t.Fatal("Submit must not be called on error")
			}
		})
	}
}

func TestVendorPksSubmitReason_VendorChecks(t *testing.T) {
	for name, v := range map[string]*db.GetVendorPksVendorRow{
		"missing":  nil,
		"internal": {ID: 7, Kind: "INTERNAL"},
		"inactive": {ID: 7, Kind: "FLM_VENDOR", DeletedAt: pgtype.Timestamptz{Valid: true}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, sub := newPksSvc(&fakePksRepo{vendor: v, byID: &db.VendorPk{ID: 5, VendorID: 7}})
			if _, err := svc.SubmitTerminate(context.Background(), 1, 7, 5, "x", ""); err == nil {
				t.Fatal("want error")
			}
			if len(sub.calls) != 0 {
				t.Fatal("Submit must not be called")
			}
		})
	}
}
