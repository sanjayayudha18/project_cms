package service

import (
	"context"
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
	listRows []db.ListForecastForDateRow
	total    int64
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
func (f *fakeForecastRepo) MaxRequestNumberSeqForDate(context.Context, pgtype.Date) (int32, error) {
	return 0, nil
}
func (f *fakeForecastRepo) ListForecastForDate(context.Context, db.ListForecastForDateParams) ([]db.ListForecastForDateRow, error) {
	return f.listRows, nil
}
func (f *fakeForecastRepo) CountForecastForDate(context.Context, db.CountForecastForDateParams) (int64, error) {
	return f.total, nil
}
func (f *fakeForecastRepo) ForecastRowExists(context.Context, db.ForecastRowExistsParams) (db.ForecastRowExistsRow, error) {
	return db.ForecastRowExistsRow{}, nil
}
func (f *fakeForecastRepo) UpdateVendorRequestStatus(context.Context, db.UpdateVendorRequestStatusParams) (db.VendorRequest, error) {
	return db.VendorRequest{}, nil
}
func (f *fakeForecastRepo) ListAuditLogsByEntity(context.Context, string, int64) ([]db.AuditLog, error) {
	return nil, nil
}

// TestBrowseForecast_MapsNewContextFields covers Task 2: the four new
// LEFT JOIN columns (*string, nullable) must map to ForecastRow as their
// value when present, and as "" when the db row has them nil (Req 3.1, 3.3).
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
			},
		},
		total: 2,
	}
	svc := &VendorRequestService{read: fake}

	result, err := svc.BrowseForecast(context.Background(), BrowseForecastParams{
		ForecastDate: "2027-01-15",
		Page:         1,
		PageSize:     10,
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

	withoutVendor := result.Data[1]
	if withoutVendor.LokasiATM != "Another Location" || withoutVendor.Brand != "Wincor" {
		t.Errorf("lokasi/brand should still populate without a vendor package: %+v", withoutVendor)
	}
	if withoutVendor.FLMVendor != "" || withoutVendor.FLMVendorRegion != "" {
		t.Errorf("nil FLM columns should map to empty string, got FLMVendor=%q FLMVendorRegion=%q",
			withoutVendor.FLMVendor, withoutVendor.FLMVendorRegion)
	}
}
