package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeForecastRepo implements VendorRequestRepository. Only
// ListForecastForDate/CountForecastForDate carry real behavior for this
// test — BrowseForecast never calls the other methods, so they return zero
// values and exist only to satisfy the interface.
type fakeForecastRepo struct {
	listRows    []db.ListForecastForDateRow
	total       int64
	summaryRows []db.SummarizeForecastForDateRow
}

func (f *fakeForecastRepo) CreateVendorRequest(context.Context, db.CreateVendorRequestParams) (db.VendorRequest, error) {
	return db.VendorRequest{}, nil
}
func (f *fakeForecastRepo) InsertVendorRequestItem(context.Context, db.InsertVendorRequestItemParams) (db.VendorRequestItem, error) {
	return db.VendorRequestItem{}, nil
}
func (f *fakeForecastRepo) DeleteVendorRequestItems(context.Context, int64) error { return nil }
func (f *fakeForecastRepo) GetVendorRequest(context.Context, int64) (db.VendorRequest, error) {
	return db.VendorRequest{}, nil
}
func (f *fakeForecastRepo) GetVendorRequestForUpdate(context.Context, int64) (db.VendorRequest, error) {
	return db.VendorRequest{}, nil
}
func (f *fakeForecastRepo) ListVendorRequestItems(context.Context, int64) ([]db.VendorRequestItem, error) {
	return nil, nil
}
func (f *fakeForecastRepo) GetVendorRequestDetail(context.Context, int64) (db.GetVendorRequestDetailRow, error) {
	return db.GetVendorRequestDetailRow{}, nil
}
func (f *fakeForecastRepo) ListVendorRequests(context.Context, db.ListVendorRequestsParams) ([]db.ListVendorRequestsRow, error) {
	return nil, nil
}
func (f *fakeForecastRepo) CountVendorRequests(context.Context, db.CountVendorRequestsParams) (int64, error) {
	return 0, nil
}
func (f *fakeForecastRepo) NextRequestNumberSeq(context.Context, db.NextRequestNumberSeqParams) (int32, error) {
	return 0, nil
}
func (f *fakeForecastRepo) ListActiveVendors(context.Context) ([]db.ListActiveVendorsRow, error) {
	return nil, nil
}
func (f *fakeForecastRepo) ListDistinctVendorBranchRegions(context.Context) ([]*string, error) {
	return nil, nil
}
func (f *fakeForecastRepo) SoftCancelVendorRequest(context.Context, db.SoftCancelVendorRequestParams) (db.VendorRequest, error) {
	return db.VendorRequest{}, nil
}
func (f *fakeForecastRepo) ListForecastForDate(context.Context, db.ListForecastForDateParams) ([]db.ListForecastForDateRow, error) {
	return f.listRows, nil
}
func (f *fakeForecastRepo) CountForecastForDate(context.Context, db.CountForecastForDateParams) (int64, error) {
	return f.total, nil
}
func (f *fakeForecastRepo) SummarizeForecastForDate(context.Context, pgtype.Date) ([]db.SummarizeForecastForDateRow, error) {
	return f.summaryRows, nil
}
func (f *fakeForecastRepo) ForecastRowExists(context.Context, db.ForecastRowExistsParams) (db.ForecastRowExistsRow, error) {
	return db.ForecastRowExistsRow{}, nil
}
func (f *fakeForecastRepo) UpdateVendorRequestStatus(context.Context, db.UpdateVendorRequestStatusParams) (db.VendorRequest, error) {
	return db.VendorRequest{}, nil
}
func (f *fakeForecastRepo) ListActiveVendorRequestTickets(context.Context, string) ([]db.ListActiveVendorRequestTicketsRow, error) {
	return nil, nil
}
func (f *fakeForecastRepo) ListRequestVisitInfo(context.Context, int64) ([]db.ListRequestVisitInfoRow, error) {
	return nil, nil
}
func (f *fakeForecastRepo) ListAuditLogsByEntity(context.Context, db.ListAuditLogsByEntityParams) ([]db.AuditLog, error) {
	return nil, nil
}

// TestBrowseForecast_MapsNewContextFields covers Task 2: the four new
// LEFT JOIN columns (*string, nullable) must map to ForecastRow as their
// value when present, and as "" when the db row has them nil (Req 3.1, 3.3).
// replenishment-request-enhancements Task 3 (Req 5.2, 5.3, 5.5, 5.10,
// 5.11): priority_class/paket follow the same *string->"" rule; escrow is a
// decimal string (never float) when present and nil — not "0.00" — when the
// terminal has no itm_replenish row.
func TestBrowseForecast_MapsNewContextFields(t *testing.T) {
	vendorName := "TAG"
	region := "TAG Jawa Barat"
	fake := &fakeForecastRepo{
		listRows: []db.ListForecastForDateRow{
			{
				TerminalID:      "1234",
				PeriodePred:     pgtype.Date{Time: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true},
				Denom:           50000,
				AmountReplenish: 100000000,
				AmountRefund:    0,
				DmaaFileID:      1,
				LokasiAtm:       stringPtrOrNil("Test Location"),
				Brand:           stringPtrOrNil("Hyosung"),
				FlmVendor:       &vendorName,
				FlmVendorRegion: &region,
				PriorityClass:   stringPtrOrNil("PRIORITY 1"),
				Paket:           stringPtrOrNil("PAKET 3"),
				Escrow:          mustNumeric(t, "1500000.00"),
			},
			{
				// No active vendor package (Req 3.3): FLM columns nil, brand/lokasi
				// still populated since those come from a separate LEFT JOIN.
				TerminalID:      "5678",
				PeriodePred:     pgtype.Date{Time: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true},
				Denom:           100000,
				AmountReplenish: 200000000,
				AmountRefund:    0,
				DmaaFileID:      1,
				LokasiAtm:       stringPtrOrNil("Another Location"),
				Brand:           stringPtrOrNil("Wincor"),
				FlmVendor:       nil,
				FlmVendorRegion: nil,
				PriorityClass:   nil,
				Paket:           nil,
				Escrow:          pgtype.Numeric{}, // invalid = SQL NULL
			},
		},
		total: 2,
	}
	svc := &VendorRequestService{read: fake}

	result, err := svc.BrowseForecast(context.Background(), BrowseForecastParams{
		ForecastDate:    "2027-01-15",
		FLMVendor:       "TAG",
		FLMVendorRegion: "TAG Jawa Barat",
		Page:            1,
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("BrowseForecast: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Data))
	}

	withVendor := result.Data[0]
	if withVendor.LokasiATM != "Test Location" || withVendor.Brand != "Hyosung" ||
		withVendor.FLMVendor != "TAG" || withVendor.FLMVendorRegion != "TAG Jawa Barat" {
		t.Errorf("row with vendor mapped incorrectly: %+v", withVendor)
	}
	if withVendor.PriorityClass != "PRIORITY 1" || withVendor.Paket != "PAKET 3" {
		t.Errorf("priority_class/paket mapped incorrectly: %q / %q",
			withVendor.PriorityClass, withVendor.Paket)
	}
	if withVendor.Escrow == nil || *withVendor.Escrow != "1500000.00" {
		t.Errorf("escrow = %v, want decimal string \"1500000.00\"", withVendor.Escrow)
	}

	withoutVendor := result.Data[1]
	if withoutVendor.LokasiATM != "Another Location" || withoutVendor.Brand != "Wincor" {
		t.Errorf("lokasi/brand should still populate without a vendor package: %+v", withoutVendor)
	}
	if withoutVendor.FLMVendor != "" || withoutVendor.FLMVendorRegion != "" {
		t.Errorf("nil FLM columns should map to empty string, got FLMVendor=%q FLMVendorRegion=%q",
			withoutVendor.FLMVendor, withoutVendor.FLMVendorRegion)
	}
	if withoutVendor.PriorityClass != "" || withoutVendor.Paket != "" {
		t.Errorf("nil priority_class/paket should map to empty string, got %q / %q",
			withoutVendor.PriorityClass, withoutVendor.Paket)
	}
	if withoutVendor.Escrow != nil {
		t.Errorf("NULL escrow should map to nil, not \"0.00\": %v", *withoutVendor.Escrow)
	}
}

// TestBrowseForecast_ValidatesCIT2Filters covers Task 4.1 (Req 1.14): all
// three filters are length-bound to <=255 chars. forecast-browser-summary FR2
// relaxed Req 1.4: empty vendor/region is now valid ("Semua"), and
// unassigned cannot be combined with a vendor/region filter.
func TestBrowseForecast_ValidatesCIT2Filters(t *testing.T) {
	base := BrowseForecastParams{
		ForecastDate:    "2027-01-15",
		FLMVendor:       "TAG",
		FLMVendorRegion: "TAG Jawa Barat",
		Page:            1,
		PageSize:        10,
	}
	longValue := strings.Repeat("a", 256)

	tests := []struct {
		name      string
		mutate    func(p BrowseForecastParams) BrowseForecastParams
		wantField string
	}{
		{"unassigned with flm_vendor rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.Unassigned = true
			p.FLMVendorRegion = ""
			return p
		}, "unassigned"},
		{"unassigned with flm_vendor_region rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.Unassigned = true
			p.FLMVendor = ""
			return p
		}, "unassigned"},
		{"no_region with flm_vendor_region rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.NoRegion = true
			return p
		}, "no_region"},
		{"unassigned with no_region rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.Unassigned, p.NoRegion = true, true
			p.FLMVendor, p.FLMVendorRegion = "", ""
			return p
		}, "unassigned"},
		{"brand over 255 chars rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.Brand = longValue
			return p
		}, "brand"},
		{"flm_vendor over 255 chars rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.FLMVendor = longValue
			return p
		}, "flm_vendor"},
		{"flm_vendor_region over 255 chars rejected", func(p BrowseForecastParams) BrowseForecastParams {
			p.FLMVendorRegion = longValue
			return p
		}, "flm_vendor_region"},
	}

	svc := &VendorRequestService{read: &fakeForecastRepo{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.BrowseForecast(context.Background(), tt.mutate(base))
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("BrowseForecast() error = %v, want *ValidationError", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}

	// Optional Brand with all required filters present must pass validation
	// (Req 1.9: empty Brand = no filter).
	if _, err := svc.BrowseForecast(context.Background(), base); err != nil {
		t.Errorf("BrowseForecast() with valid required filters = %v, want nil", err)
	}

	// forecast-browser-summary FR2: no vendor/region ("Semua") and
	// unassigned alone are both valid.
	all := base
	all.FLMVendor, all.FLMVendorRegion = "", ""
	if _, err := svc.BrowseForecast(context.Background(), all); err != nil {
		t.Errorf("BrowseForecast() without vendor/region = %v, want nil", err)
	}
	all.Unassigned = true
	if _, err := svc.BrowseForecast(context.Background(), all); err != nil {
		t.Errorf("BrowseForecast() unassigned only = %v, want nil", err)
	}
	// review R1: vendor + no_region (region left empty) is valid.
	noRegion := base
	noRegion.FLMVendorRegion, noRegion.NoRegion = "", true
	if _, err := svc.BrowseForecast(context.Background(), noRegion); err != nil {
		t.Errorf("BrowseForecast() vendor + no_region = %v, want nil", err)
	}
}

func TestBrowseForecast_MapsIsRequested(t *testing.T) {
	fake := &fakeForecastRepo{
		listRows: []db.ListForecastForDateRow{{TerminalID: "1", IsRequested: true}, {TerminalID: "2"}},
		total:    2,
	}
	svc := &VendorRequestService{read: fake}
	result, err := svc.BrowseForecast(context.Background(), BrowseForecastParams{ForecastDate: "2027-01-15", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("BrowseForecast: %v", err)
	}
	if !result.Data[0].IsRequested || result.Data[1].IsRequested {
		t.Errorf("IsRequested = %v/%v, want true/false", result.Data[0].IsRequested, result.Data[1].IsRequested)
	}
}

// TestForecastSummary_TotalsAreSumOfGroups covers forecast-browser-summary
// FR1: unrequested = atm - requested per group, totals = Σ groups, and the
// "" vendor group feeds unassigned_atm_count.
func TestForecastSummary_TotalsAreSumOfGroups(t *testing.T) {
	fake := &fakeForecastRepo{summaryRows: []db.SummarizeForecastForDateRow{
		{FlmVendor: "PT ABC", FlmVendorRegion: "Jabodetabek", AtmCount: 84, RequestedAtmCount: 40,
			AmountReplenish: 12_400_000_000, UnrequestedAmountReplenish: 6_100_000_000},
		{FlmVendor: "PT XYZ", FlmVendorRegion: "Jawa Barat", AtmCount: 51, RequestedAtmCount: 51,
			AmountReplenish: 7_100_000_000},
		{FlmVendor: "", FlmVendorRegion: "", AtmCount: 6,
			AmountReplenish: 810_000_000, UnrequestedAmountReplenish: 810_000_000},
	}}
	svc := &VendorRequestService{read: fake}

	got, err := svc.ForecastSummary(context.Background(), "2027-01-15")
	if err != nil {
		t.Fatalf("ForecastSummary: %v", err)
	}
	want := ForecastSummaryTotals{
		ATMCount: 141, RequestedATMCount: 91, UnrequestedATMCount: 50, UnassignedATMCount: 6,
		AmountReplenish: 20_310_000_000, UnrequestedAmountReplenish: 6_910_000_000,
	}
	if got.Totals != want {
		t.Errorf("Totals = %+v, want %+v", got.Totals, want)
	}
	if got.Groups[0].UnrequestedATMCount != 44 || got.Groups[1].UnrequestedATMCount != 0 {
		t.Errorf("UnrequestedATMCount = %d/%d, want 44/0", got.Groups[0].UnrequestedATMCount, got.Groups[1].UnrequestedATMCount)
	}
	if got.ForecastDate.Format("2006-01-02") != "2027-01-15" {
		t.Errorf("ForecastDate = %v", got.ForecastDate)
	}
}

func TestForecastSummary_EmptyDateReturnsEmptyGroups(t *testing.T) {
	svc := &VendorRequestService{read: &fakeForecastRepo{}}
	got, err := svc.ForecastSummary(context.Background(), "2027-01-15")
	if err != nil {
		t.Fatalf("ForecastSummary: %v", err)
	}
	if got.Groups == nil || len(got.Groups) != 0 || got.Totals != (ForecastSummaryTotals{}) {
		t.Errorf("want non-nil empty groups and zero totals, got %+v", got)
	}
}

func TestForecastSummary_ValidatesDate(t *testing.T) {
	svc := &VendorRequestService{read: &fakeForecastRepo{}}
	for _, date := range []string{"", "15-01-2027", "2027-13-40"} {
		_, err := svc.ForecastSummary(context.Background(), date)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != "forecast_date" {
			t.Errorf("ForecastSummary(%q) error = %v, want forecast_date ValidationError", date, err)
		}
	}
}
