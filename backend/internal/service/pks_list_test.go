package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// Unit tests for PksListService and the renewal_days rules (spec vendor-pks-list task 5.3,
// Property 2 and Property 5). DB-backed properties live in vendor_pks_list_integration_test.go.

type fakePksListRepo struct {
	rows       []db.ListPksAdminRow
	total      int64
	pending    []db.ListPendingPksChangesRow
	err        error
	gotList    db.ListPksAdminParams
	gotCount   db.CountPksAdminParams
	gotPending db.ListPendingPksChangesParams
	calls      int
}

func (f *fakePksListRepo) List(_ context.Context, a db.ListPksAdminParams) ([]db.ListPksAdminRow, error) {
	f.calls++
	f.gotList = a
	return f.rows, f.err
}

func (f *fakePksListRepo) Count(_ context.Context, a db.CountPksAdminParams) (int64, error) {
	f.gotCount = a
	return f.total, f.err
}

func (f *fakePksListRepo) Pending(_ context.Context, a db.ListPendingPksChangesParams) ([]db.ListPendingPksChangesRow, error) {
	f.gotPending = a
	return f.pending, f.err
}

type fakeTypeFinder struct{ known map[string]bool }

func (f fakeTypeFinder) FindByCode(_ context.Context, code string) (*db.VendorPksType, error) {
	if f.known[code] {
		return &db.VendorPksType{Code: code}, nil
	}
	return nil, nil
}

type fakeVendorFinder struct{ known map[string]int64 }

func (f fakeVendorFinder) FindByCode(_ context.Context, code string) (*int64, error) {
	if id, ok := f.known[code]; ok {
		return &id, nil
	}
	return nil, nil
}

func newPksListSvc(repo *fakePksListRepo) *PksListService {
	s := NewPksListService(repo, fakeTypeFinder{known: map[string]bool{"FLM": true, "CIT": true}},
		fakeVendorFinder{known: map[string]int64{"ACM": 1}})
	s.now = func() time.Time { return time.Date(2026, 11, 19, 10, 0, 0, 0, time.FixedZone("WIB", 7*3600)) }
	return s
}

func pendingCreateRow(id, vendorID, typeID int64, vCode, tCode, payload string) db.ListPendingPksChangesRow {
	return db.ListPendingPksChangesRow{ID: id, Op: "create", VendorID: vendorID, VendorCode: vCode, VendorName: "Vendor " + vCode,
		PksTypeID: typeID, PksTypeCode: tCode, PksTypeName: tCode, Payload: json.RawMessage(payload)}
}

func TestPksListService_RoleGuard(t *testing.T) {
	for _, role := range []string{"ATM-USER", "BRANCH-ATM-SPV", "VENDOR-USER", "ACM-SPV", ""} {
		repo := &fakePksListRepo{}
		_, err := newPksListSvc(repo).List(context.Background(), role, PksListParams{})
		if !errors.Is(err, ErrMasterDataForbidden) || repo.calls != 0 {
			t.Errorf("role %q: err=%v calls=%d, want forbidden and no repo call", role, err, repo.calls)
		}
	}
	for _, role := range reports.PksViewerRoles {
		if _, err := newPksListSvc(&fakePksListRepo{}).List(context.Background(), role, PksListParams{}); err != nil {
			t.Errorf("role %q: %v", role, err)
		}
	}
}

func TestPksListService_FilterValidation(t *testing.T) {
	tests := []struct {
		name  string
		p     PksListParams
		field string
	}{
		{"unknown status", PksListParams{Status: "belum_diisi"}, "filter"},
		{"q too long", PksListParams{Q: strings.Repeat("a", 101)}, "filter"},
		{"unknown type", PksListParams{PksType: "NOPE"}, "pks_type"},
		{"unknown vendor", PksListParams{Vendor: "NOPE"}, "vendor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakePksListRepo{}
			_, err := newPksListSvc(repo).List(context.Background(), "ADMIN", tt.p)
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("err = %v, want ValidationError on %q", err, tt.field)
			}
			if repo.calls != 0 {
				t.Error("repo queried despite invalid filter")
			}
		})
	}
}

func TestPksListService_DefaultsAndPaging(t *testing.T) {
	tests := []struct {
		name               string
		page, size         int
		wantPage, wantSize int
		wantOffset         int64
	}{
		{"defaults", 0, 0, 1, 20, 0},
		{"negative", -3, -1, 1, 20, 0},
		{"page 3 size 10", 3, 10, 3, 10, 20},
		{"size clamped to 100", 1, 500, 1, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakePksListRepo{}
			got, err := newPksListSvc(repo).List(context.Background(), "ATM-SPV", PksListParams{Page: tt.page, PageSize: tt.size})
			if err != nil {
				t.Fatal(err)
			}
			if got.Page != tt.wantPage || got.PageSize != tt.wantSize || repo.gotList.PageOffset != tt.wantOffset || repo.gotList.PageLimit != int64(tt.wantSize) {
				t.Errorf("page/size/offset/limit = %d/%d/%d/%d", got.Page, got.PageSize, repo.gotList.PageOffset, repo.gotList.PageLimit)
			}
			if repo.gotList.Status != "terkini" || repo.gotCount.Status != "terkini" {
				t.Errorf("status = %q/%q, want default terkini", repo.gotList.Status, repo.gotCount.Status)
			}
			if got.Items == nil {
				t.Error("Items is nil, want []")
			}
			wantToday := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
			if !repo.gotList.Today.Time.Equal(wantToday) || !repo.gotCount.Today.Time.Equal(wantToday) {
				t.Errorf("today = %v/%v, want WIB civil date %v", repo.gotList.Today.Time, repo.gotCount.Today.Time, wantToday)
			}
		})
	}
}

func TestPksListService_FilterReachesRepo(t *testing.T) {
	repo := &fakePksListRepo{}
	_, err := newPksListSvc(repo).List(context.Background(), "ADMIN", PksListParams{Status: "expired", PksType: " cit ", Vendor: "acm", Q: " wod "})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotList.Status != "expired" || repo.gotList.PksType == nil || *repo.gotList.PksType != "CIT" ||
		repo.gotList.Vendor == nil || *repo.gotList.Vendor != "ACM" || repo.gotList.Q == nil || *repo.gotList.Q != "wod" {
		t.Errorf("list params = %+v", repo.gotList)
	}
	if repo.gotPending.PksType == nil || *repo.gotPending.PksType != "CIT" || repo.gotPending.Vendor == nil || *repo.gotPending.Vendor != "ACM" {
		t.Errorf("pending params = %+v", repo.gotPending)
	}
}

// Property 5: pending changes are marked on the row of the same (vendor, type) and pending
// creates are merged as "menunggu_persetujuan" rows (page 1, terkini/semua only), counted in total.
func TestPksListService_PendingMerge(t *testing.T) {
	pl := `{"pks_name":"PKS Baru","effective_date":"2027-01-01","expiry_date":"2027-12-31","wod_number":"WOD-9","renewal_days":120}`
	row := db.ListPksAdminRow{ID: 5, VendorID: 1, PksTypeID: 10, VendorCode: "ACM", VendorName: "Acme", PksTypeCode: "FLM", PksTypeName: "FLM",
		WodNumber: "WOD-1", PksName: "PKS Lama", RenewalDays: 270, RemainingDays: 40, Status: "need_renewal",
		EffectiveDate: pgtype.Date{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		ExpiryDate:    pgtype.Date{Time: time.Date(2026, 12, 29, 0, 0, 0, 0, time.UTC), Valid: true}}
	pending := []db.ListPendingPksChangesRow{
		pendingCreateRow(77, 1, 10, "ACM", "FLM", pl), // same (vendor, type) as the row
		{ID: 78, Op: "disable", VendorID: 2, PksTypeID: 10, VendorCode: "OTH", PksTypeCode: "FLM", Payload: json.RawMessage(`{}`)},
	}

	tests := []struct {
		name       string
		status     string
		page       int
		wantItems  int
		wantTotal  int64
		wantCreate bool
	}{
		{"terkini page 1 merges the create", "terkini", 1, 2, 11, true},
		{"semua page 1 merges the create", "semua", 1, 2, 11, true},
		{"terkini page 2 has no create but total keeps it", "terkini", 2, 1, 11, false},
		{"expired never shows pending creates", "expired", 1, 1, 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakePksListRepo{rows: []db.ListPksAdminRow{row}, total: 10, pending: pending}
			got, err := newPksListSvc(repo).List(context.Background(), "ADMIN", PksListParams{Status: tt.status, Page: tt.page})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Items) != tt.wantItems || got.Total != tt.wantTotal {
				t.Fatalf("items/total = %d/%d, want %d/%d", len(got.Items), got.Total, tt.wantItems, tt.wantTotal)
			}
			var dbItem, createItem *PksListItem
			for i := range got.Items {
				if got.Items[i].ID == nil {
					createItem = &got.Items[i]
				} else {
					dbItem = &got.Items[i]
				}
			}
			if dbItem == nil || dbItem.Pending == nil || dbItem.Pending.ChangeID != 77 || dbItem.Pending.Op != "create" {
				t.Errorf("db row pending mark = %+v, want change 77/create", dbItem)
			}
			if tt.wantCreate {
				// today 2026-11-19 → 2027-12-31 is 407 days away.
				if createItem == nil || createItem.Status != pksStatusPending || createItem.RenewalDays != 120 ||
					createItem.WodNumber != "WOD-9" || createItem.RemainingDays != 407 {
					t.Errorf("pending create item = %+v", createItem)
				}
			} else if createItem != nil {
				t.Errorf("unexpected pending create item %+v", createItem)
			}
		})
	}
}

func TestPendingCreateItems_QFilterDefaultsAndBadPayload(t *testing.T) {
	today := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	pending := []db.ListPendingPksChangesRow{
		pendingCreateRow(1, 1, 1, "ACM", "FLM", `{"pks_name":"Alpha","wod_number":"W-1","effective_date":"2026-01-01","expiry_date":"2026-12-19"}`),
		pendingCreateRow(2, 1, 2, "ACM", "CIT", `{"pks_name":"Beta","wod_number":"X-2","effective_date":"2026-01-01","expiry_date":"2026-12-19"}`),
		pendingCreateRow(3, 1, 3, "ACM", "OFF", `not json`),
		{ID: 4, Op: "enable", PksTypeID: 1, Payload: json.RawMessage(`{}`)},
	}
	got := pendingCreateItems(pending, "", today)
	if len(got) != 2 {
		t.Fatalf("items = %d, want 2 (bad payload and non-create skipped)", len(got))
	}
	if got[0].RenewalDays != reports.RenewalDaysDefault || got[0].RemainingDays != 30 {
		t.Errorf("default renewal/remaining = %d/%d, want %d/30", got[0].RenewalDays, got[0].RemainingDays, reports.RenewalDaysDefault)
	}
	if f := pendingCreateItems(pending, "BETA", today); len(f) != 1 || f[0].PksName != "Beta" {
		t.Errorf("q=BETA → %+v", f)
	}
	if f := pendingCreateItems(pending, "w-1", today); len(f) != 1 || f[0].WodNumber != "W-1" {
		t.Errorf("q=w-1 (WOD) → %+v", f)
	}
}

// Property 2: renewal_days is valid iff 30 <= days <= 3666 and days <= term (expiry - effective).
func TestValidateRenewalDays_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		eff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, rapid.IntRange(0, 4000).Draw(rt, "eff"))
		term := rapid.IntRange(0, 5000).Draw(rt, "term")
		days := rapid.Int32Range(-10, 4000).Draw(rt, "days")
		if rapid.Bool().Draw(rt, "nearBoundary") {
			days = rapid.SampledFrom([]int32{0, 29, 30, 31, 3665, 3666, 3667, int32(term) - 1, int32(term), int32(term) + 1}).Draw(rt, "edge")
		}
		err := validateRenewalDays(days, eff, eff.AddDate(0, 0, term))
		want := days >= reports.RenewalDaysMin && days <= reports.RenewalDaysMax && int(days) <= term
		if (err == nil) != want {
			rt.Fatalf("days=%d term=%d: err=%v, want valid=%v", days, term, err, want)
		}
		var ve *ValidationError
		if err != nil && (!errors.As(err, &ve) || ve.Field != "renewal_days") {
			rt.Fatalf("err = %v, want ValidationError on renewal_days", err)
		}
	})
}

// UpdateReminder refuses every role outside ADMIN/ADMIN_PARAM before opening a transaction
// (the nil pool would panic otherwise).
func TestUpdateReminder_RoleGuard(t *testing.T) {
	svc := NewVendorPksAdminService(nil, nil, nil)
	for _, role := range []string{"ATM-SPV", "ATM-USER", "BRANCH-ATM-SPV", "VENDOR-USER", "APPACCESS", ""} {
		if _, err := svc.UpdateReminder(context.Background(), 1, role, 5, 100, "127.0.0.1"); !errors.Is(err, ErrMasterDataForbidden) {
			t.Errorf("role %q: err = %v, want ErrMasterDataForbidden", role, err)
		}
	}
}
