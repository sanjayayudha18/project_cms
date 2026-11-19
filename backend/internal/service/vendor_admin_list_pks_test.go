package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Unit tests for the vendor-list PKS surface (spec vendor-pks-type task 5.2, Req 4.1-4.4, 4.6).
// The DB-backed properties (P6/P7) live in vendor_admin_list_pks_integration_test.go.

func TestVendorAdminService_ResolvePksFilterType(t *testing.T) {
	boom := errors.New("db down")
	tests := []struct {
		name    string
		repo    func(context.Context, string) (*db.VendorPksType, error)
		code    string
		wantID  int64
		wantErr error
		wantReq string // code the repo must have been asked for
	}{
		{"active type, trimmed and upper-cased", func(_ context.Context, c string) (*db.VendorPksType, error) {
			return &db.VendorPksType{ID: 7, Code: c, IsActive: true}, nil
		}, " cit ", 7, nil, "CIT"},
		{"unknown type", func(context.Context, string) (*db.VendorPksType, error) { return nil, nil }, "NOPE", 0, ErrPksFilterType, "NOPE"},
		{"inactive type", func(context.Context, string) (*db.VendorPksType, error) {
			return &db.VendorPksType{ID: 3, Code: "OLD", IsActive: false}, nil
		}, "OLD", 0, ErrPksFilterType, "OLD"},
		{"repo error is not a filter error", func(context.Context, string) (*db.VendorPksType, error) { return nil, boom }, "FLM", 0, boom, "FLM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var asked string
			repo := &fakeVendorAdminRepo{pksTypeByCodeFunc: func(ctx context.Context, c string) (*db.VendorPksType, error) {
				asked = c
				return tt.repo(ctx, c)
			}}
			id, err := NewVendorAdminService(repo, nil).ResolvePksFilterType(context.Background(), tt.code)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if id != tt.wantID || asked != tt.wantReq {
				t.Errorf("id/asked = %d/%q, want %d/%q", id, asked, tt.wantID, tt.wantReq)
			}
		})
	}
}

func TestVendorAdminService_PksSummaries(t *testing.T) {
	wib := time.FixedZone("WIB", 7*3600)
	var got db.ListVendorPksSummariesParams
	calls := 0
	repo := &fakeVendorAdminRepo{pksSummariesFunc: func(_ context.Context, arg db.ListVendorPksSummariesParams) ([]db.ListVendorPksSummariesRow, error) {
		calls++
		got = arg
		return []db.ListVendorPksSummariesRow{
			{VendorID: 1, Code: "CIT", Name: "Cash in Transit", PksStatus: "need_renewal", RemainingDays: 40},
			{VendorID: 1, Code: "FLM", Name: "FLM", PksStatus: "belum_diisi", RemainingDays: 0},
			{VendorID: 2, Code: "CIT", Name: "Cash in Transit", PksStatus: "expired", RemainingDays: -5},
		}, nil
	}}
	svc := NewVendorAdminService(repo, nil, WithPksRenewalMonths(6))
	svc.now = func() time.Time { return time.Date(2026, 11, 18, 23, 30, 0, 0, wib) }

	out, err := svc.PksSummaries(context.Background(), []int64{1, 2, 3})
	if err != nil {
		t.Fatalf("PksSummaries: %v", err)
	}
	if calls != 1 {
		t.Errorf("repo calls = %d, want 1 (one query per page)", calls)
	}
	if got.RenewalMonths != 6 || !got.Today.Valid || got.Today.Time.Format("2006-01-02") != "2026-11-18" || len(got.VendorIds) != 3 {
		t.Errorf("params = %+v, want renewal 6, today 2026-11-18 (WIB), 3 ids", got)
	}

	v1 := out[1]
	if len(v1) != 2 || v1[0].PksType != "CIT" || v1[1].PksType != "FLM" {
		t.Fatalf("vendor 1 entries = %+v, want CIT then FLM (query order kept)", v1)
	}
	if v1[0].PksTypeName != "Cash in Transit" || v1[0].PksStatus != "need_renewal" || v1[0].RemainingDays == nil || *v1[0].RemainingDays != 40 {
		t.Errorf("vendor 1 CIT = %+v", v1[0])
	}
	if v1[1].PksStatus != "belum_diisi" || v1[1].RemainingDays != nil {
		t.Errorf("vendor 1 FLM = %+v, want belum_diisi with nil remaining_days", v1[1])
	}
	if e := out[2]; len(e) != 1 || e[0].RemainingDays == nil || *e[0].RemainingDays != -5 {
		t.Errorf("vendor 2 = %+v, want expired with remaining_days -5", e)
	}
	if _, ok := out[3]; ok {
		t.Errorf("vendor 3 (INTERNAL / no rows) must have no entry, got %+v", out[3])
	}
}

func TestVendorAdminService_PksSummaries_EmptyAndError(t *testing.T) {
	calls := 0
	repo := &fakeVendorAdminRepo{pksSummariesFunc: func(context.Context, db.ListVendorPksSummariesParams) ([]db.ListVendorPksSummariesRow, error) {
		calls++
		return nil, errors.New("boom")
	}}
	svc := NewVendorAdminService(repo, nil)

	out, err := svc.PksSummaries(context.Background(), nil)
	if err != nil || len(out) != 0 || calls != 0 {
		t.Errorf("empty page: out=%v err=%v calls=%d, want empty map, no query", out, err, calls)
	}
	if _, err := svc.PksSummaries(context.Background(), []int64{1}); err == nil {
		t.Error("repo error must propagate")
	}
}

func TestVendorAdminService_ListCount_FillTodayAndRenewal(t *testing.T) {
	typeID := int64(5)
	var listArg db.ListVendorsAdminParams
	var countArg db.CountVendorsAdminParams
	repo := &fakeVendorAdminRepo{
		listFunc: func(_ context.Context, a db.ListVendorsAdminParams) ([]db.ListVendorsAdminRow, error) {
			listArg = a
			return nil, nil
		},
		countFunc: func(_ context.Context, a db.CountVendorsAdminParams) (int64, error) {
			countArg = a
			return 0, nil
		},
	}
	svc := NewVendorAdminService(repo, nil, WithPksRenewalMonths(9))
	svc.now = func() time.Time { return time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC) }
	st := "aktif"

	if _, err := svc.List(context.Background(), db.ListVendorsAdminParams{PksTypeID: &typeID, PksStatus: &st, RenewalMonths: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Count(context.Background(), db.CountVendorsAdminParams{PksTypeID: &typeID, PksStatus: &st}); err != nil {
		t.Fatal(err)
	}
	if listArg.RenewalMonths != 9 || countArg.RenewalMonths != 9 || !listArg.Today.Valid || !countArg.Today.Valid {
		t.Errorf("service must override Today/RenewalMonths: list=%+v count=%+v", listArg, countArg)
	}
	if listArg.PksTypeID == nil || *listArg.PksTypeID != typeID || countArg.PksTypeID == nil || *countArg.PksTypeID != typeID {
		t.Error("PksTypeID must reach the repo unchanged for both list and count")
	}
}
