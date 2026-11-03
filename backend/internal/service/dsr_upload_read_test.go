package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// scopedDsrRepo owns upload 5 for vendor "Bijak" only, mimicking the SQL-side
// vendor scoping of GetDsrUploadByIDForVendor.
type scopedDsrRepo struct {
	fakeDsrRepository
	vendorErr error
	listArg   db.ListDsrUploadsByVendorParams
}

func (f *scopedDsrRepo) GetVendorByID(ctx context.Context, id int64) (db.GetVendorByIDRow, error) {
	if f.vendorErr != nil {
		return db.GetVendorByIDRow{}, f.vendorErr
	}
	return f.fakeDsrRepository.GetVendorByID(ctx, id)
}

func (f *scopedDsrRepo) GetDsrUploadByIDForVendor(_ context.Context, arg db.GetDsrUploadByIDForVendorParams) (db.GetDsrUploadByIDForVendorRow, error) {
	if arg.ID != 5 || arg.Vendor != "Bijak" {
		return db.GetDsrUploadByIDForVendorRow{}, pgx.ErrNoRows
	}
	rows, errs := int32(10), int32(3)
	return db.GetDsrUploadByIDForVendorRow{
		ID: 5, Filename: "dsr.xlsx", Vendor: "Bijak",
		ReportDate:  pgtype.Date{Time: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Valid: true},
		DailyStatus: "completed", DailyRowCount: &rows, DailyErrorCount: &errs,
		RencanaIsiStatus: "failed",
	}, nil
}

func (f *scopedDsrRepo) ListDsrDailyRowErrors(context.Context, int64) ([]db.ListDsrDailyRowErrorsRow, error) {
	return []db.ListDsrDailyRowErrorsRow{{RowNo: 4, LineLabel: "SALDO ?"}}, nil
}

func (f *scopedDsrRepo) ListDsrDailyRows(context.Context, int64) ([]db.ListDsrDailyRowsRow, error) {
	var total pgtype.Numeric
	_ = total.Scan("1500000")
	return []db.ListDsrDailyRowsRow{{RowNo: 1, LineLabel: "SALDO AKHIR", LineTotalIdr: total}}, nil
}

func (f *scopedDsrRepo) ListDsrRencanaIsiRows(context.Context, int64) ([]db.ListDsrRencanaIsiRowsRow, error) {
	var fill pgtype.Numeric
	_ = fill.Scan("2000000")
	return []db.ListDsrRencanaIsiRowsRow{{RowNo: 1, AtmTerminalID: "T001", Fill100kIdr: fill}}, nil
}

func (f *scopedDsrRepo) ListDsrUploadsByVendor(_ context.Context, arg db.ListDsrUploadsByVendorParams) ([]db.ListDsrUploadsByVendorRow, error) {
	f.listArg = arg
	return []db.ListDsrUploadsByVendorRow{{ID: 5, ReportDate: pgtype.Date{Time: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), Valid: true}, DailyStatus: "completed", RencanaIsiStatus: "failed"}}, nil
}

func (f *scopedDsrRepo) CountDsrUploadsByVendor(context.Context, db.CountDsrUploadsByVendorParams) (int64, error) {
	return 1, nil
}

func TestDsrService_ValidateUpload(t *testing.T) {
	s := &DsrService{}
	xlsx := append([]byte{'P', 'K', 3, 4}, make([]byte, 8)...)
	xls := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 8)...)

	for _, tc := range []struct {
		name, filename string
		content        []byte
		want           string // "" = error
	}{
		{"xlsx", "dsr.xlsx", xlsx, "dsr.xlsx"},
		{"xls", "dsr.xls", xls, "dsr.xls"},
		{"path traversal stripped", "../../etc/dsr.xlsx", xlsx, "dsr.xlsx"},
		{"empty", "dsr.xlsx", nil, ""},
		{"too big", "dsr.xlsx", append(xlsx, make([]byte, DsrMaxUploadBytes)...), ""},
		{"not a workbook", "dsr.xlsx", []byte("hello world"), ""},
		{"dot-dot name", "..", xlsx, ""},
		{"no name", "", xlsx, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ValidateUpload(tc.content, tc.filename)
			if tc.want == "" {
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Field != "file" {
					t.Fatalf("got %q, %v, want file ValidationError", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestDsrService_ResolveVendor(t *testing.T) {
	repo := &scopedDsrRepo{}
	s := NewDsrService(repo, nil, nil, "", "", "")
	if code, name, err := s.ResolveVendor(context.Background(), 7); err != nil || code != "BIJAK" || name != "Bijak" {
		t.Fatalf("ResolveVendor = %q %q %v", code, name, err)
	}
	repo.vendorErr = pgx.ErrNoRows
	if _, _, err := s.ResolveVendor(context.Background(), 7); !errors.Is(err, ErrDsrVendorNotFound) {
		t.Fatalf("missing vendor err = %v, want ErrDsrVendorNotFound", err)
	}
	repo.vendorErr = errors.New("db down")
	if _, _, err := s.ResolveVendor(context.Background(), 7); err == nil || errors.Is(err, ErrDsrVendorNotFound) {
		t.Fatalf("db error = %v, want wrapped non-not-found error", err)
	}
}

func TestDsrService_CheckUploadRate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewDsrService(&scopedDsrRepo{}, rdb, nil, "", "", "")
	ctx := context.Background()

	for i := 1; i <= dsrUploadRateLimit; i++ {
		if err := s.CheckUploadRate(ctx, 7); err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
	}
	if err := s.CheckUploadRate(ctx, 7); !errors.Is(err, ErrDsrRateLimited) {
		t.Fatalf("upload %d err = %v, want ErrDsrRateLimited", dsrUploadRateLimit+1, err)
	}
	if err := s.CheckUploadRate(ctx, 8); err != nil {
		t.Fatalf("other vendor has its own counter: %v", err)
	}
	for _, key := range mr.Keys() {
		if ttl := mr.TTL(key); ttl <= 0 || ttl > time.Hour {
			t.Errorf("key %s TTL = %v, want (0, 1h]", key, ttl)
		}
	}
	mr.Close()
	if err := s.CheckUploadRate(ctx, 7); err == nil || errors.Is(err, ErrDsrRateLimited) {
		t.Fatalf("redis down err = %v, want a plain error (handler -> 500)", err)
	}
}

func TestDsrService_SheetDetailIsVendorScoped(t *testing.T) {
	s := NewDsrService(&scopedDsrRepo{}, nil, nil, "", "", "")
	ctx := context.Background()

	daily, err := s.GetSaldoFile(ctx, "Bijak", 5)
	if err != nil {
		t.Fatal(err)
	}
	if daily.FileID != 5 || daily.Status != "completed" || daily.RowCount != 10 || daily.ErrorCount != 3 || daily.SuccessCount != 7 ||
		daily.ReportDate == nil || len(daily.Errors) != 1 || daily.Errors[0].Label != "SALDO ?" {
		t.Errorf("daily detail = %+v", daily)
	}
	if len(daily.DailyRows) != 1 || daily.DailyRows[0].LineTotalIdr == nil || *daily.DailyRows[0].LineTotalIdr != "1500000.00" || daily.DailyRows[0].Denom100k != nil {
		t.Errorf("daily rows = %+v (NULL denom must stay nil)", daily.DailyRows)
	}

	rencana, err := s.GetRencanaIsiFile(ctx, "Bijak", 5)
	if err != nil {
		t.Fatal(err)
	}
	if rencana.Status != "failed" || rencana.RowCount != 0 || len(rencana.RencanaIsiRows) != 1 ||
		rencana.RencanaIsiRows[0].Fill100kIdr == nil || *rencana.RencanaIsiRows[0].Fill100kIdr != "2000000.00" {
		t.Errorf("rencana detail = %+v", rencana)
	}

	for _, get := range []func(context.Context, string, int64) (*DsrSheetDetail, error){s.GetSaldoFile, s.GetRencanaIsiFile} {
		if _, err := get(ctx, "Other Vendor", 5); !errors.Is(err, ErrDsrFileNotFound) {
			t.Errorf("other vendor's file err = %v, want ErrDsrFileNotFound", err)
		}
		if _, err := get(ctx, "Bijak", 6); !errors.Is(err, ErrDsrFileNotFound) {
			t.Errorf("missing file err = %v, want ErrDsrFileNotFound", err)
		}
	}
}

func TestDsrService_ListUploads(t *testing.T) {
	repo := &scopedDsrRepo{}
	s := NewDsrService(repo, nil, nil, "", "", "")
	ctx := context.Background()

	res, err := s.ListUploads(ctx, DsrListUploadsParams{VendorName: "Bijak", Page: 2, PageSize: 10, DateFrom: "2026-10-01", DateTo: "2026-10-09"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Data) != 1 || res.Data[0].Daily.FileID != 5 || res.Data[0].RencanaIsi.Status != "failed" {
		t.Errorf("list = %+v", res)
	}
	if repo.listArg.Vendor != "Bijak" || repo.listArg.Page != 2 || repo.listArg.PageSize != 10 {
		t.Errorf("list arg = %+v (must stay scoped to the caller's vendor)", repo.listArg)
	}

	for _, tc := range []struct {
		p     DsrListUploadsParams
		field string
	}{
		{DsrListUploadsParams{Page: 0, PageSize: 10}, "page"},
		{DsrListUploadsParams{Page: 1, PageSize: 0}, "page_size"},
		{DsrListUploadsParams{Page: 1, PageSize: 101}, "page_size"},
		{DsrListUploadsParams{Page: 1, PageSize: 10, DateFrom: "09-10-2026"}, "date_from"},
		{DsrListUploadsParams{Page: 1, PageSize: 10, DateTo: "kemarin"}, "date_to"},
		{DsrListUploadsParams{Page: 1, PageSize: 10, DateFrom: "2026-10-09", DateTo: "2026-10-01"}, "date_from"},
	} {
		var ve *ValidationError
		if _, err := s.ListUploads(ctx, tc.p); !errors.As(err, &ve) || ve.Field != tc.field {
			t.Errorf("%+v: err = %v, want %s ValidationError", tc.p, err, tc.field)
		} else if strings.TrimSpace(ve.Message) == "" {
			t.Errorf("%s: empty message", tc.field)
		}
	}
}
