package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ListDmaaForecastParams holds validated request parameters for listing
// dmaa_atm_forecast rows. The HTTP handler applies API-contract defaults
// (page=1, page_size=25, sort_by=periode_pred, sort_order=desc) before
// calling ListDmaaForecast, matching the ATM Portal convention.
type ListDmaaForecastParams struct {
	Page       int    `json:"page" validate:"gte=1"`
	PageSize   int    `json:"page_size" validate:"gte=1,lte=100"`
	DateFrom   string `json:"date_from"`
	DateTo     string `json:"date_to"`
	TerminalID string `json:"terminal_id"`
	SortBy     string `json:"sort_by" validate:"omitempty,oneof=terminal_id dmaa_file_id periode_pred denom amount_replenish amount_refund created_at"`
	SortOrder  string `json:"sort_order" validate:"omitempty,oneof=asc desc"`
}

var allowedDmaaForecastSortBy = []string{
	"terminal_id", "dmaa_file_id", "periode_pred", "denom", "amount_replenish", "amount_refund", "created_at",
}

// validate checks ListDmaaForecastParams, returning a *ValidationError for
// the first failing field.
func (p ListDmaaForecastParams) validate() error {
	if p.Page < 1 {
		return &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		return &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	if len(p.TerminalID) > 100 {
		return &ValidationError{Field: "terminal_id", Message: "maksimal 100 karakter"}
	}
	if err := validateDateBound("date_from", p.DateFrom); err != nil {
		return err
	}
	if err := validateDateBound("date_to", p.DateTo); err != nil {
		return err
	}
	if p.DateFrom != "" && p.DateTo != "" && p.DateFrom > p.DateTo {
		return &ValidationError{Field: "date_from", Message: "tidak boleh lebih besar dari date_to"}
	}
	if p.SortBy != "" && !slices.Contains(allowedDmaaForecastSortBy, p.SortBy) {
		return &ValidationError{
			Field:   "sort_by",
			Message: "harus salah satu dari: terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund, created_at",
		}
	}
	if p.SortOrder != "" && !slices.Contains([]string{"asc", "desc"}, p.SortOrder) {
		return &ValidationError{Field: "sort_order", Message: "harus salah satu dari: asc, desc"}
	}
	return nil
}

// DmaaForecastRow is one dmaa_atm_forecast record. Amounts are bigint in
// IDR minor-major units (whole rupiah), never float.
type DmaaForecastRow struct {
	TerminalID      string
	DmaaFileID      int64
	PeriodePred     time.Time
	Denom           int32
	AmountReplenish int64
	AmountRefund    int64
	CreatedAt       time.Time
}

// ListDmaaForecastResult is the paginated forecast list result.
type ListDmaaForecastResult struct {
	Data       []DmaaForecastRow
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// DmaaForecastRepository is the small, service-defined interface the
// sqlc-generated *db.Queries satisfies. Defined where it's used, per Go
// convention, so the service can be tested against a fake.
type DmaaForecastRepository interface {
	ListDmaaForecast(ctx context.Context, arg db.ListDmaaForecastParams) ([]db.DmaaAtmForecast, error)
	CountDmaaForecast(ctx context.Context, arg db.CountDmaaForecastParams) (int64, error)
}

// DmaaForecastServicer is the interface DmaaForecastHandler depends on.
type DmaaForecastServicer interface {
	ListDmaaForecast(ctx context.Context, params ListDmaaForecastParams) (*ListDmaaForecastResult, error)
}

// DmaaForecastService implements DmaaForecastServicer.
type DmaaForecastService struct {
	repo DmaaForecastRepository
}

// NewDmaaForecastService creates a new DmaaForecastService wrapping the
// given repository (instantiate with the read replica pool).
func NewDmaaForecastService(repo DmaaForecastRepository) *DmaaForecastService {
	return &DmaaForecastService{repo: repo}
}

// ListDmaaForecast validates params, lists and counts dmaa_atm_forecast
// rows, and assembles the paginated result with TotalPages =
// ceil(TotalRows / PageSize).
func (s *DmaaForecastService) ListDmaaForecast(ctx context.Context, params ListDmaaForecastParams) (*ListDmaaForecastResult, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	rows, err := s.repo.ListDmaaForecast(ctx, db.ListDmaaForecastParams{
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
		TerminalID: params.TerminalID,
		SortBy:     params.SortBy,
		SortOrder:  params.SortOrder,
		Page:       int32(params.Page),
		PageSize:   int32(params.PageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("listing dmaa forecast: %w", err)
	}

	total, err := s.repo.CountDmaaForecast(ctx, db.CountDmaaForecastParams{
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
		TerminalID: params.TerminalID,
	})
	if err != nil {
		return nil, fmt.Errorf("counting dmaa forecast: %w", err)
	}

	data := make([]DmaaForecastRow, len(rows))
	for i, row := range rows {
		if !row.PeriodePred.Valid {
			return nil, fmt.Errorf("converting dmaa forecast row terminal_id=%s: periode_pred is null", row.TerminalID)
		}
		if !row.CreatedAt.Valid {
			return nil, fmt.Errorf("converting dmaa forecast row terminal_id=%s: created_at is null", row.TerminalID)
		}
		data[i] = DmaaForecastRow{
			TerminalID:      row.TerminalID,
			DmaaFileID:      row.DmaaFileID,
			PeriodePred:     row.PeriodePred.Time,
			Denom:           row.Denom,
			AmountReplenish: row.AmountReplenish,
			AmountRefund:    row.AmountRefund,
			CreatedAt:       row.CreatedAt.Time,
		}
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}

	return &ListDmaaForecastResult{
		Data:       data,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}
