package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ListCashposParams holds validated request parameters for listing itm_cashpos rows.
// Handler applies defaults (page=1, page_size=25, sort_by=cashpos_date, sort_order=desc)
// before calling ListCashpos.
type ListCashposParams struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	Search    string `json:"search"`
	DateFrom  string `json:"date_from"`
	DateTo    string `json:"date_to"`
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"`
}

var allowedCashposSortBy = []string{
	"cashpos_date", "terminal_id", "machine_type", "branch_code", "created_at", "id",
}

func (p ListCashposParams) validate() error {
	if p.Page < 1 {
		return &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		return &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	if len(p.Search) > 100 {
		return &ValidationError{Field: "search", Message: "maksimal 100 karakter"}
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
	if p.SortBy != "" && !slices.Contains(allowedCashposSortBy, p.SortBy) {
		return &ValidationError{
			Field:   "sort_by",
			Message: "harus salah satu dari: cashpos_date, terminal_id, machine_type, branch_code, created_at, id",
		}
	}
	if p.SortOrder != "" && !slices.Contains([]string{"asc", "desc"}, p.SortOrder) {
		return &ValidationError{Field: "sort_order", Message: "harus salah satu dari: asc, desc"}
	}
	return nil
}

// CashposRow is one itm_cashpos record with monetary fields as exact decimal strings.
type CashposRow struct {
	ID               int64
	FileID           int64
	CashposDate      time.Time
	TerminalID       string
	MachineType      string
	TellerID         string
	BranchCode       string
	StartingCash10k  string
	CashIn10k        string
	CashOut10k       string
	CashPosition10k  string
	StartingCash20k  string
	CashIn20k        string
	CashOut20k       string
	CashPosition20k  string
	StartingCash50k  string
	CashIn50k        string
	CashOut50k       string
	CashPosition50k  string
	StartingCash100k string
	CashIn100k       string
	CashOut100k      string
	CashPosition100k string
	PositionSource   string
	CreatedAt        time.Time
}

// ListCashposResult is the paginated cashpos list result.
type ListCashposResult struct {
	Data     []CashposRow
	Total    int64
	Page     int
	PageSize int
}

// AtmPortalCashposRepository is the cashpos-specific repository surface.
type AtmPortalCashposRepository interface {
	ListItmCashpos(ctx context.Context, arg db.ListItmCashposParams) ([]db.ItmCashpo, error)
	CountItmCashpos(ctx context.Context, arg db.CountItmCashposParams) (int64, error)
}

// ListCashpos validates params, lists and counts itm_cashpos rows, and maps
// numerics to exact decimal strings (no float conversion).
func (s *AtmPortalService) ListCashpos(ctx context.Context, params ListCashposParams) (*ListCashposResult, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	repo, ok := s.repo.(AtmPortalCashposRepository)
	if !ok {
		return nil, fmt.Errorf("listing cashpos: repository does not support cashpos queries")
	}

	listArg := db.ListItmCashposParams{
		Search:    params.Search,
		DateFrom:  params.DateFrom,
		DateTo:    params.DateTo,
		SortBy:    params.SortBy,
		SortOrder: params.SortOrder,
		Page:      int32(params.Page),
		PageSize:  int32(params.PageSize),
	}
	rows, err := repo.ListItmCashpos(ctx, listArg)
	if err != nil {
		return nil, fmt.Errorf("listing cashpos: %w", err)
	}

	total, err := repo.CountItmCashpos(ctx, db.CountItmCashposParams{
		Search:   params.Search,
		DateFrom: params.DateFrom,
		DateTo:   params.DateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("counting cashpos: %w", err)
	}

	data := make([]CashposRow, len(rows))
	for i, row := range rows {
		mapped, err := rowToCashposRow(row)
		if err != nil {
			return nil, fmt.Errorf("converting cashpos row id=%d: %w", row.ID, err)
		}
		data[i] = mapped
	}

	return &ListCashposResult{
		Data:     data,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

func rowToCashposRow(row db.ItmCashpo) (CashposRow, error) {
	out := CashposRow{
		ID:             row.ID,
		FileID:         row.FileID,
		TerminalID:     row.TerminalID,
		MachineType:    row.MachineType,
		TellerID:       row.TellerID,
		BranchCode:     row.BranchCode,
		PositionSource: row.PositionSource,
	}

	if !row.CashposDate.Valid {
		return CashposRow{}, fmt.Errorf("cashpos_date is null")
	}
	out.CashposDate = row.CashposDate.Time

	if !row.CreatedAt.Valid {
		return CashposRow{}, fmt.Errorf("created_at is null")
	}
	out.CreatedAt = row.CreatedAt.Time

	type amountField struct {
		name string
		n    pgtype.Numeric
		set  func(string)
	}
	fields := []amountField{
		{"starting_cash_10k", row.StartingCash10k, func(v string) { out.StartingCash10k = v }},
		{"cash_in_10k", row.CashIn10k, func(v string) { out.CashIn10k = v }},
		{"cash_out_10k", row.CashOut10k, func(v string) { out.CashOut10k = v }},
		{"cash_position_10k", row.CashPosition10k, func(v string) { out.CashPosition10k = v }},
		{"starting_cash_20k", row.StartingCash20k, func(v string) { out.StartingCash20k = v }},
		{"cash_in_20k", row.CashIn20k, func(v string) { out.CashIn20k = v }},
		{"cash_out_20k", row.CashOut20k, func(v string) { out.CashOut20k = v }},
		{"cash_position_20k", row.CashPosition20k, func(v string) { out.CashPosition20k = v }},
		{"starting_cash_50k", row.StartingCash50k, func(v string) { out.StartingCash50k = v }},
		{"cash_in_50k", row.CashIn50k, func(v string) { out.CashIn50k = v }},
		{"cash_out_50k", row.CashOut50k, func(v string) { out.CashOut50k = v }},
		{"cash_position_50k", row.CashPosition50k, func(v string) { out.CashPosition50k = v }},
		{"starting_cash_100k", row.StartingCash100k, func(v string) { out.StartingCash100k = v }},
		{"cash_in_100k", row.CashIn100k, func(v string) { out.CashIn100k = v }},
		{"cash_out_100k", row.CashOut100k, func(v string) { out.CashOut100k = v }},
		{"cash_position_100k", row.CashPosition100k, func(v string) { out.CashPosition100k = v }},
	}
	for _, f := range fields {
		s, err := numericToDecimalString(f.n)
		if err != nil {
			return CashposRow{}, fmt.Errorf("%s: %w", f.name, err)
		}
		f.set(s)
	}
	return out, nil
}

// numericToDecimalString converts pgtype.Numeric to an exact decimal string
// without float64. SQL NULL maps to "0.00" (NOT NULL money columns default 0).
func numericToDecimalString(n pgtype.Numeric) (string, error) {
	if !n.Valid {
		return "0.00", nil
	}
	if n.NaN {
		return "", fmt.Errorf("numeric is NaN")
	}
	if n.InfinityModifier != pgtype.Finite {
		return "", fmt.Errorf("numeric is infinite")
	}
	if n.Int == nil {
		return "0.00", nil
	}

	// value = Int * 10^Exp
	intStr := n.Int.String()
	neg := false
	if strings.HasPrefix(intStr, "-") {
		neg = true
		intStr = strings.TrimPrefix(intStr, "-")
	}
	if intStr == "" || intStr == "0" {
		if neg {
			// -0
		}
		// Fall through with "0" and apply exp for scale.
		if intStr == "" {
			intStr = "0"
		}
	}

	exp := int(n.Exp)
	var body string
	switch {
	case exp >= 0:
		body = intStr + strings.Repeat("0", exp) + ".00"
	default:
		scale := -exp
		// Pad left with zeros so we have at least `scale` fractional digits.
		for len(intStr) <= scale {
			intStr = "0" + intStr
		}
		// Now len(intStr) > scale: split integer / fractional parts.
		// After padding, if original was smaller than scale, leading zero is integer part.
		split := len(intStr) - scale
		intPart := intStr[:split]
		fracPart := intStr[split:]
		// Strip leading zeros from int part but keep a single 0.
		intPart = strings.TrimLeft(intPart, "0")
		if intPart == "" {
			intPart = "0"
		}
		// For money numeric(20,2), normalize fractional to at least 2 digits
		// without dropping extra precision if present.
		if len(fracPart) < 2 {
			fracPart = fracPart + strings.Repeat("0", 2-len(fracPart))
		}
		body = intPart + "." + fracPart
	}

	if neg && body != "0.00" && body != "0" {
		return "-" + body, nil
	}
	return body, nil
}
