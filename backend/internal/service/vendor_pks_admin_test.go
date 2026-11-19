package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Unit tests for VendorPksAdminService submit paths (spec vendor-pks task 7.4,
// per type: vendor-pks-type task 4.4).

const (
	testTypeFLM = int64(1)
	testTypeCIT = int64(2)
)

type fakePksRepo struct {
	vendor    *db.GetVendorPksVendorRow
	active    map[int64]*db.GetActiveVendorPksRow // by pks type id
	byID      *db.GetVendorPksByIDRow
	pending   []db.GetPendingVendorPksChangesRow
	types     map[string]*db.VendorPksType // by code
	wodTaken  bool
	pendingEr error
}

func (f *fakePksRepo) Vendor(context.Context, int64) (*db.GetVendorPksVendorRow, error) {
	return f.vendor, nil
}
func (f *fakePksRepo) Active(_ context.Context, _, typeID int64) (*db.GetActiveVendorPksRow, error) {
	return f.active[typeID], nil
}
func (f *fakePksRepo) GetByID(context.Context, int64) (*db.GetVendorPksByIDRow, error) {
	return f.byID, nil
}
func (f *fakePksRepo) ListByVendor(context.Context, int64) ([]db.ListVendorPksByVendorRow, error) {
	return nil, nil
}
func (f *fakePksRepo) Pending(context.Context, int64) ([]db.GetPendingVendorPksChangesRow, error) {
	return f.pending, f.pendingEr
}
func (f *fakePksRepo) PksType(_ context.Context, id int64) (*db.VendorPksType, error) {
	for _, t := range f.types {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}
func (f *fakePksRepo) PksTypeByCode(_ context.Context, code string) (*db.VendorPksType, error) {
	return f.types[code], nil
}
func (f *fakePksRepo) WodTaken(context.Context, string) (bool, error) { return f.wodTaken, nil }
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

func testPksTypes() map[string]*db.VendorPksType {
	return map[string]*db.VendorPksType{
		"FLM": {ID: testTypeFLM, Code: "FLM", Name: "FLM", IsActive: true},
		"CIT": {ID: testTypeCIT, Code: "CIT", Name: "CIT", IsActive: true},
		"OFF": {ID: 3, Code: "OFF", Name: "Off", IsActive: false},
	}
}

// newPksRepo is a repo with an FLM vendor and the three test types; tests tweak it.
func newPksRepo() *fakePksRepo {
	return &fakePksRepo{vendor: okVendor(), types: testPksTypes()}
}

func newPksSvc(repo *fakePksRepo) (*VendorPksAdminService, *fakePksSubmitter) {
	sub := &fakePksSubmitter{}
	return NewVendorPksAdminService(repo, sub, 9), sub
}

func pendingFor(id, vendorID, typeID int64) db.GetPendingVendorPksChangesRow {
	p, _ := json.Marshal(map[string]int64{"vendor_id": vendorID, "pks_type_id": typeID})
	return db.GetPendingVendorPksChangesRow{ID: id, Op: "create", Payload: p}
}

func validPksReq() SubmitVendorPksRequest {
	return SubmitVendorPksRequest{PksType: "FLM", WodNumber: "WOD-001", PksName: "PKS 2026", EffectiveDate: "2026-01-01", ExpiryDate: "2027-01-01"}
}

func TestVendorPksSubmitNew_Validation(t *testing.T) {
	valid := validPksReq()
	with := func(f func(*SubmitVendorPksRequest)) SubmitVendorPksRequest { r := valid; f(&r); return r }

	tests := []struct {
		name      string
		req       SubmitVendorPksRequest
		wantField string // "" = accepted
	}{
		{"valid", valid, ""},
		{"type missing", with(func(r *SubmitVendorPksRequest) { r.PksType = "  " }), "pks_type"},
		{"type unknown", with(func(r *SubmitVendorPksRequest) { r.PksType = "NOPE" }), "pks_type"},
		{"type inactive", with(func(r *SubmitVendorPksRequest) { r.PksType = "OFF" }), "pks_type"},
		{"wod empty", with(func(r *SubmitVendorPksRequest) { r.WodNumber = "   " }), "wod_number"},
		{"wod 100 ok", with(func(r *SubmitVendorPksRequest) { r.WodNumber = strings.Repeat("a", 100) }), ""},
		{"wod 101", with(func(r *SubmitVendorPksRequest) { r.WodNumber = strings.Repeat("a", 101) }), "wod_number"},
		{"wod 101 runes", with(func(r *SubmitVendorPksRequest) { r.WodNumber = strings.Repeat("é", 101) }), "wod_number"},
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
			svc, sub := newPksSvc(newPksRepo())
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
		name  string
		setup func(*fakePksRepo)
		check func(error) bool
	}{
		{"missing vendor 404", func(r *fakePksRepo) { r.vendor = nil }, func(e error) bool { return errors.Is(e, ErrVendorNotFound) }},
		{"internal vendor 422", func(r *fakePksRepo) { r.vendor = &db.GetVendorPksVendorRow{ID: 7, Kind: "INTERNAL"} },
			func(e error) bool { return errors.Is(e, ErrVendorPksInternalVendor) }},
		{"inactive vendor 409", func(r *fakePksRepo) {
			r.vendor = &db.GetVendorPksVendorRow{ID: 7, Kind: "FLM_VENDOR", DeletedAt: pgtype.Timestamptz{Valid: true}}
		}, func(e error) bool { return errors.Is(e, ErrVendorPksVendorInactive) }},
		{"wod taken 409", func(r *fakePksRepo) { r.wodTaken = true }, func(e error) bool { return errors.Is(e, ErrVendorPksWodTaken) }},
		{"pending same type 409", func(r *fakePksRepo) { r.pending = []db.GetPendingVendorPksChangesRow{pendingFor(3, 7, testTypeFLM)} },
			func(e error) bool { return errors.Is(e, ErrMasterDataChangePending) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newPksRepo()
			tt.setup(repo)
			svc, sub := newPksSvc(repo)
			_, err := svc.SubmitNew(context.Background(), 1, 7, validPksReq(), "")
			if !tt.check(err) {
				t.Fatalf("unexpected err: %v", err)
			}
			if len(sub.calls) != 0 {
				t.Fatal("Submit must not be called")
			}
		})
	}
}

// Property 8 (unit side): a pending change blocks only its own (vendor, type).
func TestVendorPksSubmitNew_PendingPerType(t *testing.T) {
	repo := newPksRepo()
	repo.pending = []db.GetPendingVendorPksChangesRow{pendingFor(3, 7, testTypeFLM)}
	svc, sub := newPksSvc(repo)

	cit := validPksReq()
	cit.PksType = "CIT"
	if _, err := svc.SubmitNew(context.Background(), 1, 7, cit, ""); err != nil {
		t.Fatalf("pending FLM must not block CIT: %v", err)
	}
	if len(sub.calls) != 1 || sub.calls[0].Payload.(VendorPksCreatePayload).PksTypeID != testTypeCIT {
		t.Fatalf("calls = %+v, want one CIT create", sub.calls)
	}
	if _, err := svc.SubmitNew(context.Background(), 1, 7, validPksReq(), ""); !errors.Is(err, ErrMasterDataChangePending) {
		t.Fatalf("FLM err = %v, want ErrMasterDataChangePending", err)
	}
}

func TestVendorPksSubmitNew_ReplacesPksID(t *testing.T) {
	req := validPksReq()
	req.PksName, req.WodNumber = " P ", " WOD-9 "

	t.Run("no active → nil, type id + trimmed fields in payload", func(t *testing.T) {
		svc, sub := newPksSvc(newPksRepo())
		if _, err := svc.SubmitNew(context.Background(), 1, 7, req, ""); err != nil {
			t.Fatal(err)
		}
		got := sub.calls[0]
		p := got.Payload.(VendorPksCreatePayload)
		if got.EntityType != "vendor_pks" || got.Op != "create" || got.EntityID != nil {
			t.Fatalf("bad request: %+v", got)
		}
		if p.ReplacesPksID != nil || p.PksName != "P" || p.WodNumber != "WOD-9" || p.VendorID != 7 || p.PksTypeID != testTypeFLM {
			t.Fatalf("bad payload: %+v", p)
		}
	})

	t.Run("active of the same type → its id (even when terminated)", func(t *testing.T) {
		repo := newPksRepo()
		repo.active = map[int64]*db.GetActiveVendorPksRow{
			testTypeFLM: {ID: 42, VendorID: 7, PksTypeID: testTypeFLM, TerminatedAt: pgtype.Timestamptz{Valid: true}},
		}
		svc, sub := newPksSvc(repo)
		if _, err := svc.SubmitNew(context.Background(), 1, 7, req, ""); err != nil {
			t.Fatal(err)
		}
		p := sub.calls[0].Payload.(VendorPksCreatePayload)
		if p.ReplacesPksID == nil || *p.ReplacesPksID != 42 {
			t.Fatalf("ReplacesPksID = %v, want 42", p.ReplacesPksID)
		}
	})

	t.Run("active of another type is not replaced", func(t *testing.T) {
		repo := newPksRepo()
		repo.active = map[int64]*db.GetActiveVendorPksRow{testTypeCIT: {ID: 99, VendorID: 7, PksTypeID: testTypeCIT}}
		svc, sub := newPksSvc(repo)
		if _, err := svc.SubmitNew(context.Background(), 1, 7, req, ""); err != nil {
			t.Fatal(err)
		}
		if p := sub.calls[0].Payload.(VendorPksCreatePayload); p.ReplacesPksID != nil {
			t.Fatalf("ReplacesPksID = %v, want nil (CIT PKS is a different type)", *p.ReplacesPksID)
		}
	})
}

func TestVendorPksSubmitReason(t *testing.T) {
	terminated := pgtype.Timestamptz{Valid: true}
	row := func(mod func(*db.GetVendorPksByIDRow)) *db.GetVendorPksByIDRow {
		r := &db.GetVendorPksByIDRow{ID: 5, VendorID: 7, PksTypeID: testTypeCIT, PksName: "PKS A"}
		if mod != nil {
			mod(r)
		}
		return r
	}
	activeRow := row(nil)
	termRow := row(func(r *db.GetVendorPksByIDRow) { r.TerminatedAt = terminated })

	tests := []struct {
		name      string
		terminate bool
		pks       *db.GetVendorPksByIDRow
		reason    string
		pending   []db.GetPendingVendorPksChangesRow
		wantErr   error // nil = ok
		wantField string
		wantOp    string
	}{
		{"terminate ok", true, activeRow, " salah input ", nil, nil, "", "disable"},
		{"unterminate ok", false, termRow, "batal", nil, nil, "", "enable"},
		{"pks not found", true, nil, "x", nil, ErrVendorPksNotFound, "", ""},
		{"pks other vendor", true, row(func(r *db.GetVendorPksByIDRow) { r.VendorID = 99 }), "x", nil, ErrVendorPksNotFound, "", ""},
		{"pks replaced", true, row(func(r *db.GetVendorPksByIDRow) { r.ReplacedAt = terminated }), "x", nil, ErrVendorPksNotActive, "", ""},
		{"unterminate replaced", false, row(func(r *db.GetVendorPksByIDRow) { r.ReplacedAt, r.TerminatedAt = terminated, terminated }), "x", nil, ErrVendorPksNotActive, "", ""},
		{"terminate twice", true, termRow, "x", nil, ErrVendorPksAlreadyTerminated, "", ""},
		{"unterminate not terminated", false, activeRow, "x", nil, ErrVendorPksNotTerminated, "", ""},
		{"empty reason", true, activeRow, "   ", nil, nil, "reason", ""},
		{"reason 501", true, activeRow, strings.Repeat("a", 501), nil, nil, "reason", ""},
		{"reason 500 ok", true, activeRow, strings.Repeat("a", 500), nil, nil, "", "disable"},
		{"pending same type", true, activeRow, "x", []db.GetPendingVendorPksChangesRow{pendingFor(1, 7, testTypeCIT)}, ErrMasterDataChangePending, "", ""},
		{"pending other type ok", true, activeRow, "x", []db.GetPendingVendorPksChangesRow{pendingFor(1, 7, testTypeFLM)}, nil, "", "disable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newPksRepo()
			repo.byID, repo.pending = tt.pks, tt.pending
			svc, sub := newPksSvc(repo)
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
				// before = snapshot of the PKS row (same shape as the applier's CurrentState).
				if b, ok := c.Before.(db.GetVendorPksByIDRow); !ok || b.ID != 5 || b.PksName != "PKS A" {
					t.Fatalf("Before = %#v, want snapshot of the row", c.Before)
				}
				p := c.Payload.(VendorPksReasonPayload)
				if p.VendorID != 7 || p.PksTypeID != testTypeCIT || p.Reason != strings.TrimSpace(tt.reason) {
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
			repo := newPksRepo()
			repo.vendor, repo.byID = v, &db.GetVendorPksByIDRow{ID: 5, VendorID: 7}
			svc, sub := newPksSvc(repo)
			if _, err := svc.SubmitTerminate(context.Background(), 1, 7, 5, "x", ""); err == nil {
				t.Fatal("want error")
			}
			if len(sub.calls) != 0 {
				t.Fatal("Submit must not be called")
			}
		})
	}
}
