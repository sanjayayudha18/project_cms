package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// dmaaFakeRepo implements DmaaForecastRepository.
type dmaaFakeRepo struct {
	listRows  []db.DmaaAtmForecast
	listErr   error
	count     int64
	countErr  error
	lastList  *db.ListDmaaForecastParams
	lastCount *db.CountDmaaForecastParams
}

func (f *dmaaFakeRepo) ListDmaaForecast(_ context.Context, arg db.ListDmaaForecastParams) ([]db.DmaaAtmForecast, error) {
	cp := arg
	f.lastList = &cp
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listRows, nil
}

func (f *dmaaFakeRepo) CountDmaaForecast(_ context.Context, arg db.CountDmaaForecastParams) (int64, error) {
	cp := arg
	f.lastCount = &cp
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.count, nil
}

func sampleDmaaRow() db.DmaaAtmForecast {
	return db.DmaaAtmForecast{
		TerminalID:      "ATM001",
		DmaaFileID:      42,
		PeriodePred:     pgtype.Date{Time: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), Valid: true},
		Denom:           100000,
		AmountReplenish: 500000000,
		AmountRefund:    0,
		CreatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 8, 25, 10, 30, 0, 0, time.UTC), Valid: true},
	}
}

func validDmaaParams() ListDmaaForecastParams {
	return ListDmaaForecastParams{
		Page:       1,
		PageSize:   25,
		DateFrom:   "",
		DateTo:     "",
		TerminalID: "",
		SortBy:     "periode_pred",
		SortOrder:  "desc",
	}
}

func TestListDmaaForecast_ValidParamsPassThrough(t *testing.T) {
	repo := &dmaaFakeRepo{listRows: []db.DmaaAtmForecast{sampleDmaaRow()}, count: 1}
	svc := NewDmaaForecastService(repo)

	result, err := svc.ListDmaaForecast(context.Background(), validDmaaParams())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Data))
	}
	if result.Data[0].AmountReplenish != 500000000 {
		t.Errorf("AmountReplenish = %d, want 500000000", result.Data[0].AmountReplenish)
	}
	if repo.lastList == nil || repo.lastList.SortBy != "periode_pred" || repo.lastList.SortOrder != "desc" {
		t.Errorf("list params not passed through: %+v", repo.lastList)
	}
}

func TestListDmaaForecast_PaginationMetadata(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		pageSize  int
		wantPages int
	}{
		{"exact division", 100, 25, 4},
		{"remainder rounds up", 101, 25, 5},
		{"zero rows", 0, 25, 0},
		{"single row", 1, 25, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &dmaaFakeRepo{count: tt.total}
			svc := NewDmaaForecastService(repo)
			params := validDmaaParams()
			params.PageSize = tt.pageSize

			result, err := svc.ListDmaaForecast(context.Background(), params)
			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}
			if result.TotalPages != tt.wantPages {
				t.Errorf("TotalPages = %d, want %d", result.TotalPages, tt.wantPages)
			}
		})
	}
}

func TestListDmaaForecast_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(p *ListDmaaForecastParams)
		field  string
	}{
		{"page zero", func(p *ListDmaaForecastParams) { p.Page = 0 }, "page"},
		{"page negative", func(p *ListDmaaForecastParams) { p.Page = -1 }, "page"},
		{"page_size zero", func(p *ListDmaaForecastParams) { p.PageSize = 0 }, "page_size"},
		{"page_size over 100", func(p *ListDmaaForecastParams) { p.PageSize = 101 }, "page_size"},
		{"date_from invalid", func(p *ListDmaaForecastParams) { p.DateFrom = "2026/08/01" }, "date_from"},
		{"date_to invalid", func(p *ListDmaaForecastParams) { p.DateTo = "not-a-date" }, "date_to"},
		{"date_from after date_to", func(p *ListDmaaForecastParams) {
			p.DateFrom = "2026-08-10"
			p.DateTo = "2026-08-01"
		}, "date_from"},
		{"sort_by unknown", func(p *ListDmaaForecastParams) { p.SortBy = "nope" }, "sort_by"},
		{"sort_order invalid", func(p *ListDmaaForecastParams) { p.SortOrder = "up" }, "sort_order"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := validDmaaParams()
			tt.mutate(&params)

			_, err := NewDmaaForecastService(&dmaaFakeRepo{}).ListDmaaForecast(context.Background(), params)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if ve.Field != tt.field {
				t.Errorf("Field = %q, want %q", ve.Field, tt.field)
			}
		})
	}
}

func TestListDmaaForecast_AllSortColumnsAndValidDatesPass(t *testing.T) {
	for _, sortBy := range allowedDmaaForecastSortBy {
		params := validDmaaParams()
		params.SortBy = sortBy
		if err := params.validate(); err != nil {
			t.Errorf("sortBy %q should be allowed: %v", sortBy, err)
		}
	}
	params := validDmaaParams()
	params.DateFrom = "2026-08-01"
	params.DateTo = "2026-08-31"
	if err := params.validate(); err != nil {
		t.Errorf("valid date range should pass: %v", err)
	}
}

func TestListDmaaForecast_RepoErrorMapped(t *testing.T) {
	svc := NewDmaaForecastService(&dmaaFakeRepo{listErr: errors.New("connection refused")})
	if _, err := svc.ListDmaaForecast(context.Background(), validDmaaParams()); err == nil {
		t.Fatal("expected error when repo fails")
	}

	svc = NewDmaaForecastService(&dmaaFakeRepo{countErr: errors.New("connection refused")})
	if _, err := svc.ListDmaaForecast(context.Background(), validDmaaParams()); err == nil {
		t.Fatal("expected error when count fails")
	}
}

func TestListDmaaForecast_NullDatesRejected(t *testing.T) {
	badRow := sampleDmaaRow()
	badRow.PeriodePred = pgtype.Date{}
	svc := NewDmaaForecastService(&dmaaFakeRepo{listRows: []db.DmaaAtmForecast{badRow}, count: 1})

	if _, err := svc.ListDmaaForecast(context.Background(), validDmaaParams()); err == nil {
		t.Fatal("expected error when periode_pred is null")
	}
}
