package service

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// cashposFakeRepo implements AtmPortalRepository + AtmPortalCashposRepository.
type cashposFakeRepo struct {
	listRows  []db.ItmCashpo
	listErr   error
	count     int64
	countErr  error
	lastList  *db.ListItmCashposParams
	lastCount *db.CountItmCashposParams
}

func (f *cashposFakeRepo) ListATMsWithCashPos(context.Context, db.ListATMsWithCashPosParams) ([]db.ListATMsWithCashPosRow, error) {
	return nil, errors.New("not implemented")
}
func (f *cashposFakeRepo) CountATMsWithCashPos(context.Context, db.CountATMsWithCashPosParams) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *cashposFakeRepo) GetATMSummary(context.Context) (db.GetATMSummaryRow, error) {
	return db.GetATMSummaryRow{}, errors.New("not implemented")
}
func (f *cashposFakeRepo) GetLastUpdated(context.Context) (pgtype.Timestamp, error) {
	return pgtype.Timestamp{}, errors.New("not implemented")
}
func (f *cashposFakeRepo) ListItmCashpos(_ context.Context, arg db.ListItmCashposParams) ([]db.ItmCashpo, error) {
	cp := arg
	f.lastList = &cp
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listRows, nil
}
func (f *cashposFakeRepo) CountItmCashpos(_ context.Context, arg db.CountItmCashposParams) (int64, error) {
	cp := arg
	f.lastCount = &cp
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.count, nil
}

func mustNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("scan numeric %q: %v", s, err)
	}
	return n
}

func sampleCashposRow(t *testing.T) db.ItmCashpo {
	t.Helper()
	date := pgtype.Date{Time: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), Valid: true}
	created := pgtype.Timestamptz{Time: time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC), Valid: true}
	return db.ItmCashpo{
		ID:               42,
		FileID:           7,
		CashposDate:      date,
		TerminalID:       "ATM001",
		MachineType:      "ATM100K",
		TellerID:         "T001",
		BranchCode:       "BR01",
		StartingCash10k:  mustNumeric(t, "1000000.00"),
		CashIn10k:        mustNumeric(t, "50000.50"),
		CashOut10k:       mustNumeric(t, "25000.25"),
		CashPosition10k:  mustNumeric(t, "1025000.25"),
		StartingCash20k:  mustNumeric(t, "2000000.00"),
		CashIn20k:        mustNumeric(t, "0.00"),
		CashOut20k:       mustNumeric(t, "0.00"),
		CashPosition20k:  mustNumeric(t, "2000000.00"),
		StartingCash50k:  mustNumeric(t, "5000000.00"),
		CashIn50k:        mustNumeric(t, "100.00"),
		CashOut50k:       mustNumeric(t, "50.00"),
		CashPosition50k:  mustNumeric(t, "5050.00"),
		StartingCash100k: mustNumeric(t, "9999999999999999.99"),
		CashIn100k:       mustNumeric(t, "1.01"),
		CashOut100k:      mustNumeric(t, "2.02"),
		CashPosition100k: mustNumeric(t, "3.03"),
		PositionSource:   "CURRENT",
		CreatedAt:        created,
	}
}

func TestListCashpos_Validation(t *testing.T) {
	svc := NewAtmPortalService(&cashposFakeRepo{})
	ctx := context.Background()

	cases := []struct {
		name    string
		params  ListCashposParams
		wantFld string
	}{
		{"page zero", ListCashposParams{Page: 0, PageSize: 25, SortBy: "cashpos_date", SortOrder: "desc"}, "page"},
		{"page_size high", ListCashposParams{Page: 1, PageSize: 101, SortBy: "cashpos_date", SortOrder: "desc"}, "page_size"},
		{"page_size zero", ListCashposParams{Page: 1, PageSize: 0, SortBy: "cashpos_date", SortOrder: "desc"}, "page_size"},
		{"search too long", ListCashposParams{Page: 1, PageSize: 25, Search: strings.Repeat("x", 101), SortBy: "cashpos_date", SortOrder: "desc"}, "search"},
		{"bad date_from", ListCashposParams{Page: 1, PageSize: 25, DateFrom: "20-08-2026", SortBy: "cashpos_date", SortOrder: "desc"}, "date_from"},
		{"bad date_to", ListCashposParams{Page: 1, PageSize: 25, DateTo: "not-a-date", SortBy: "cashpos_date", SortOrder: "desc"}, "date_to"},
		{"reversed range", ListCashposParams{Page: 1, PageSize: 25, DateFrom: "2026-08-21", DateTo: "2026-08-01", SortBy: "cashpos_date", SortOrder: "desc"}, "date_from"},
		{"bad sort_by", ListCashposParams{Page: 1, PageSize: 25, SortBy: "refund_total", SortOrder: "desc"}, "sort_by"},
		{"bad sort_order", ListCashposParams{Page: 1, PageSize: 25, SortBy: "cashpos_date", SortOrder: "up"}, "sort_order"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ListCashpos(ctx, tc.params)
			if err == nil {
				t.Fatal("expected validation error")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %T: %v", err, err)
			}
			if ve.Field != tc.wantFld {
				t.Errorf("field = %q, want %q", ve.Field, tc.wantFld)
			}
		})
	}
}

func TestListCashpos_MapsAllFieldsAndDecimalStrings(t *testing.T) {
	row := sampleCashposRow(t)
	fake := &cashposFakeRepo{listRows: []db.ItmCashpo{row}, count: 1}
	svc := NewAtmPortalService(fake)
	ctx := context.Background()

	result, err := svc.ListCashpos(ctx, ListCashposParams{
		Page: 1, PageSize: 25, SortBy: "cashpos_date", SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("ListCashpos: %v", err)
	}
	if result.Total != 1 || result.Page != 1 || result.PageSize != 25 {
		t.Fatalf("meta total/page/size = %d/%d/%d", result.Total, result.Page, result.PageSize)
	}
	if len(result.Data) != 1 {
		t.Fatalf("len(data)=%d", len(result.Data))
	}
	got := result.Data[0]
	if got.ID != 42 || got.FileID != 7 {
		t.Errorf("id/file_id = %d/%d", got.ID, got.FileID)
	}
	if got.TerminalID != "ATM001" || got.MachineType != "ATM100K" || got.TellerID != "T001" || got.BranchCode != "BR01" {
		t.Errorf("identity fields mismatch: %+v", got)
	}
	if got.PositionSource != "CURRENT" {
		t.Errorf("position_source = %q", got.PositionSource)
	}
	if got.CashposDate.Format("2006-01-02") != "2026-08-20" {
		t.Errorf("cashpos_date = %v", got.CashposDate)
	}
	if !got.CreatedAt.Equal(time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("created_at = %v", got.CreatedAt)
	}
	// Exact decimal strings — large value must not lose precision via float.
	if got.StartingCash100k != "9999999999999999.99" {
		t.Errorf("starting_cash_100k = %q, want exact large decimal", got.StartingCash100k)
	}
	if got.CashIn10k != "50000.50" {
		t.Errorf("cash_in_10k = %q", got.CashIn10k)
	}
	if got.CashPosition10k != "1025000.25" {
		t.Errorf("cash_position_10k = %q", got.CashPosition10k)
	}
	if fake.lastList == nil || fake.lastList.Page != 1 || fake.lastList.PageSize != 25 {
		t.Errorf("list params not forwarded: %+v", fake.lastList)
	}
}

func TestListCashpos_Empty(t *testing.T) {
	fake := &cashposFakeRepo{listRows: nil, count: 0}
	svc := NewAtmPortalService(fake)
	result, err := svc.ListCashpos(context.Background(), ListCashposParams{
		Page: 1, PageSize: 25, SortBy: "terminal_id", SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result.Total != 0 || len(result.Data) != 0 {
		t.Fatalf("want empty, got total=%d len=%d", result.Total, len(result.Data))
	}
}

func TestListCashpos_RepoError(t *testing.T) {
	fake := &cashposFakeRepo{listErr: errors.New("db down")}
	svc := NewAtmPortalService(fake)
	_, err := svc.ListCashpos(context.Background(), ListCashposParams{
		Page: 1, PageSize: 10, SortBy: "id", SortOrder: "asc",
	})
	if err == nil || !strings.Contains(err.Error(), "listing cashpos") {
		t.Fatalf("want wrapped list error, got %v", err)
	}
}

func TestListCashpos_CountError(t *testing.T) {
	fake := &cashposFakeRepo{listRows: []db.ItmCashpo{}, countErr: errors.New("count fail")}
	svc := NewAtmPortalService(fake)
	_, err := svc.ListCashpos(context.Background(), ListCashposParams{
		Page: 1, PageSize: 10, SortBy: "id", SortOrder: "asc",
	})
	if err == nil || !strings.Contains(err.Error(), "counting cashpos") {
		t.Fatalf("want wrapped count error, got %v", err)
	}
}

func TestListCashpos_ForwardsFilters(t *testing.T) {
	fake := &cashposFakeRepo{listRows: nil, count: 0}
	svc := NewAtmPortalService(fake)
	_, err := svc.ListCashpos(context.Background(), ListCashposParams{
		Page: 2, PageSize: 50,
		Search: "ATM", DateFrom: "2026-01-01", DateTo: "2026-08-21",
		SortBy: "branch_code", SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fake.lastList.Search != "ATM" || fake.lastList.DateFrom != "2026-01-01" || fake.lastList.DateTo != "2026-08-21" {
		t.Errorf("filters: %+v", fake.lastList)
	}
	if fake.lastList.SortBy != "branch_code" || fake.lastList.SortOrder != "asc" || fake.lastList.Page != 2 {
		t.Errorf("sort/page: %+v", fake.lastList)
	}
	if fake.lastCount.Search != "ATM" {
		t.Errorf("count search: %+v", fake.lastCount)
	}
}

func TestNumericToDecimalString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0", "0.00"},
		{"0.00", "0.00"},
		{"1.50", "1.50"},
		{"1000000.00", "1000000.00"},
		{"9999999999999999.99", "9999999999999999.99"},
		{"-12.34", "-12.34"},
	}
	for _, tc := range cases {
		n := mustNumeric(t, tc.in)
		got, err := numericToDecimalString(n)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("numericToDecimalString(%q) = %q, want %q (int=%v exp=%d)", tc.in, got, tc.want, n.Int, n.Exp)
		}
	}

	// Explicit construction without Scan.
	var n pgtype.Numeric
	n.Int = big.NewInt(12345)
	n.Exp = -2
	n.Valid = true
	got, err := numericToDecimalString(n)
	if err != nil {
		t.Fatal(err)
	}
	if got != "123.45" {
		t.Errorf("got %q want 123.45", got)
	}
}
