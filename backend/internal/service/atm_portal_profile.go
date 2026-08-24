package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// ErrATMNotFound is returned by GetATMProfile when the terminal ID does not
// match an active, non-deleted ATM. The handler maps this to 404.
var ErrATMNotFound = errors.New("atm not found")

// ATMProfileResult is the ATM Profile header: master data plus the same
// replenishment-status precedence used by ListATMs (design.md Property 4),
// computed here in Go rather than SQL because it draws from two separate
// single-row queries (GetATMByTerminalID + GetLatestReplenishForTerminal)
// instead of one joined list query.
type ATMProfileResult struct {
	TerminalID              string
	LocationName            string
	Address                 string
	MachineType             string
	Brand                   string
	Model                   string
	DeploymentType          string
	OperationHours          string
	CapacityAmount          *string // decimal string, nil = NULL
	LowThresholdAmount      *string // decimal string, nil = NULL
	CriticalThresholdAmount *string // decimal string, nil = NULL
	IsActive                bool
	ReplenishmentStatus     string
}

// ListATMReplenishParams holds validated request parameters for a single
// terminal's replenishment history. Handler applies defaults (page=1,
// page_size=25) before calling ListATMReplenish.
type ListATMReplenishParams struct {
	TerminalID string
	Page       int
	PageSize   int
	DateFrom   string
	DateTo     string
}

// validate checks pagination/date bounds. Unlike ListATMCashposParams,
// date_from > date_to is NOT rejected here — design.md's error matrix (Req
// 4.6) says the replenish endpoint returns an empty result set for that
// case, not a 400.
func (p ListATMReplenishParams) validate() error {
	if strings.TrimSpace(p.TerminalID) == "" {
		return &ValidationError{Field: "terminalId", Message: "wajib diisi"}
	}
	if p.Page < 1 {
		return &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		return &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	if err := validateDateBound("date_from", p.DateFrom); err != nil {
		return err
	}
	if err := validateDateBound("date_to", p.DateTo); err != nil {
		return err
	}
	return nil
}

// ReplenishRecord is one itm_replenish row scoped to a single terminal, with
// monetary fields as exact decimal strings (no float).
type ReplenishRecord struct {
	ReplenishDate      string
	ReplenishTime      string
	TerminalID         string
	MachineType        string
	TellerID           string
	BranchCode         string
	Escrow             string
	RefundDenom10k     string
	RefundDenom20k     string
	RefundDenom50k     string
	RefundDenom100k    string
	RefundTotal        string
	ReplenishDenom10k  string
	ReplenishDenom20k  string
	ReplenishDenom50k  string
	ReplenishDenom100k string
	ReplenishTotal     string
}

// ListATMReplenishResult is the paginated replenish-history result.
type ListATMReplenishResult struct {
	Data     []ReplenishRecord
	Total    int64
	Page     int
	PageSize int
}

// ListATMCashposParams holds validated request parameters for a single
// terminal's cash-position history.
type ListATMCashposParams struct {
	TerminalID string
	Page       int
	PageSize   int
	DateFrom   string
	DateTo     string
}

// validate checks pagination/date bounds. Unlike ListATMReplenishParams,
// date_from > date_to IS rejected here as a 400 (design.md Req 5.8).
func (p ListATMCashposParams) validate() error {
	if strings.TrimSpace(p.TerminalID) == "" {
		return &ValidationError{Field: "terminalId", Message: "wajib diisi"}
	}
	if p.Page < 1 {
		return &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		return &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
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
	return nil
}

// ListATMCashposResult is the paginated cash-position-history result.
// Reuses CashposRow (defined in atm_portal_cashpos.go) instead of a new
// "CashposRecord" type — design.md's per-terminal CashposRecord shape is
// field-for-field identical to the existing global-list CashposRow, and
// ListCashposByTerminal scans into the same db.ItmCashpo row, so the
// existing rowToCashposRow converter is reused as-is.
type ListATMCashposResult struct {
	Data     []CashposRow
	Total    int64
	Page     int
	PageSize int
}

// AtmPortalProfileRepository is the ATM Profile repository surface (6 new
// sqlc-generated methods). *db.Queries satisfies this automatically.
type AtmPortalProfileRepository interface {
	GetATMByTerminalID(ctx context.Context, terminalID string) (db.GetATMByTerminalIDRow, error)
	GetLatestReplenishForTerminal(ctx context.Context, terminalID string) (pgtype.Numeric, error)
	ListReplenishByTerminal(ctx context.Context, arg db.ListReplenishByTerminalParams) ([]db.ListReplenishByTerminalRow, error)
	CountReplenishByTerminal(ctx context.Context, arg db.CountReplenishByTerminalParams) (int64, error)
	ListCashposByTerminal(ctx context.Context, arg db.ListCashposByTerminalParams) ([]db.ItmCashpo, error)
	CountCashposByTerminal(ctx context.Context, arg db.CountCashposByTerminalParams) (int64, error)
}

// GetATMProfile validates the terminal ID, fetches ATM master data and the
// latest replenish record from the read replica, computes the
// replenishment status precedence (design.md Property 4), and assembles
// the profile header result.
func (s *AtmPortalService) GetATMProfile(ctx context.Context, terminalID string) (*ATMProfileResult, error) {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return nil, &ValidationError{Field: "terminalId", Message: "wajib diisi"}
	}

	repo, ok := s.repo.(AtmPortalProfileRepository)
	if !ok {
		return nil, fmt.Errorf("getting atm profile: repository does not support profile queries")
	}

	atmRow, err := repo.GetATMByTerminalID(ctx, terminalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrATMNotFound
		}
		return nil, fmt.Errorf("getting atm: %w", err)
	}

	refundTotal, hasReplenish, err := latestRefundTotal(ctx, repo, terminalID)
	if err != nil {
		return nil, fmt.Errorf("getting latest replenish: %w", err)
	}

	lowThreshold, err := numericToFloat64Ptr(atmRow.LowThresholdAmount)
	if err != nil {
		return nil, fmt.Errorf("low_threshold_amount: %w", err)
	}
	criticalThreshold, err := numericToFloat64Ptr(atmRow.CriticalThresholdAmount)
	if err != nil {
		return nil, fmt.Errorf("critical_threshold_amount: %w", err)
	}
	status := computeReplenishmentStatus(lowThreshold, criticalThreshold, hasReplenish, refundTotal)

	capacityAmount, err := numericToDecimalStringPtr(atmRow.CapacityAmount)
	if err != nil {
		return nil, fmt.Errorf("capacity_amount: %w", err)
	}
	lowThresholdStr, err := numericToDecimalStringPtr(atmRow.LowThresholdAmount)
	if err != nil {
		return nil, fmt.Errorf("low_threshold_amount: %w", err)
	}
	criticalThresholdStr, err := numericToDecimalStringPtr(atmRow.CriticalThresholdAmount)
	if err != nil {
		return nil, fmt.Errorf("critical_threshold_amount: %w", err)
	}

	return &ATMProfileResult{
		TerminalID:              atmRow.TerminalID,
		LocationName:            atmRow.LocationName,
		Address:                 atmRow.Address,
		MachineType:             atmRow.MachineType,
		Brand:                   atmRow.Brand,
		Model:                   atmRow.Model,
		DeploymentType:          atmRow.DeploymentType,
		OperationHours:          atmRow.OperationHours,
		CapacityAmount:          capacityAmount,
		LowThresholdAmount:      lowThresholdStr,
		CriticalThresholdAmount: criticalThresholdStr,
		IsActive:                atmRow.IsActive,
		ReplenishmentStatus:     status,
	}, nil
}

// latestRefundTotal fetches the latest replenish record's refund_total,
// returning hasReplenish=false (not an error) when the terminal has no
// replenish history yet.
func latestRefundTotal(ctx context.Context, repo AtmPortalProfileRepository, terminalID string) (*float64, bool, error) {
	n, err := repo.GetLatestReplenishForTerminal(ctx, terminalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	v, err := numericToFloat64Ptr(n)
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// computeReplenishmentStatus applies the strict precedence order from
// design.md Property 4: unconfigured > no_data > critical > low > normal.
func computeReplenishmentStatus(lowThreshold, criticalThreshold *float64, hasReplenish bool, refundTotal *float64) string {
	if lowThreshold == nil {
		return "unconfigured"
	}
	if !hasReplenish || refundTotal == nil {
		return "no_data"
	}
	if criticalThreshold != nil && *refundTotal <= *criticalThreshold {
		return "critical"
	}
	if *refundTotal <= *lowThreshold {
		return "low"
	}
	return "normal"
}

// ListATMReplenish validates params, lists and counts a single terminal's
// itm_replenish rows, and maps numerics to exact decimal strings.
func (s *AtmPortalService) ListATMReplenish(ctx context.Context, params ListATMReplenishParams) (*ListATMReplenishResult, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	repo, ok := s.repo.(AtmPortalProfileRepository)
	if !ok {
		return nil, fmt.Errorf("listing atm replenish: repository does not support profile queries")
	}

	// Req 4.6: date_from > date_to returns an empty result, not an error.
	if params.DateFrom != "" && params.DateTo != "" && params.DateFrom > params.DateTo {
		return &ListATMReplenishResult{Data: []ReplenishRecord{}, Page: params.Page, PageSize: params.PageSize}, nil
	}

	rows, err := repo.ListReplenishByTerminal(ctx, db.ListReplenishByTerminalParams{
		TerminalID: params.TerminalID,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
		Page:       int32(params.Page),
		PageSize:   int32(params.PageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("listing atm replenish: %w", err)
	}

	total, err := repo.CountReplenishByTerminal(ctx, db.CountReplenishByTerminalParams{
		TerminalID: params.TerminalID,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("counting atm replenish: %w", err)
	}

	data := make([]ReplenishRecord, len(rows))
	for i, row := range rows {
		rec, err := rowToReplenishRecord(row)
		if err != nil {
			return nil, fmt.Errorf("converting replenish row: %w", err)
		}
		data[i] = rec
	}

	return &ListATMReplenishResult{Data: data, Total: total, Page: params.Page, PageSize: params.PageSize}, nil
}

// ListATMCashpos validates params, lists and counts a single terminal's
// itm_cashpos rows, reusing rowToCashposRow (atm_portal_cashpos.go) since
// the row shape is identical to the global cashpos list.
func (s *AtmPortalService) ListATMCashpos(ctx context.Context, params ListATMCashposParams) (*ListATMCashposResult, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	repo, ok := s.repo.(AtmPortalProfileRepository)
	if !ok {
		return nil, fmt.Errorf("listing atm cashpos: repository does not support profile queries")
	}

	rows, err := repo.ListCashposByTerminal(ctx, db.ListCashposByTerminalParams{
		TerminalID: params.TerminalID,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
		Page:       int32(params.Page),
		PageSize:   int32(params.PageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("listing atm cashpos: %w", err)
	}

	total, err := repo.CountCashposByTerminal(ctx, db.CountCashposByTerminalParams{
		TerminalID: params.TerminalID,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("counting atm cashpos: %w", err)
	}

	data := make([]CashposRow, len(rows))
	for i, row := range rows {
		mapped, err := rowToCashposRow(row)
		if err != nil {
			return nil, fmt.Errorf("converting cashpos row id=%d: %w", row.ID, err)
		}
		data[i] = mapped
	}

	return &ListATMCashposResult{Data: data, Total: total, Page: params.Page, PageSize: params.PageSize}, nil
}

// rowToReplenishRecord converts a generated sqlc row into ReplenishRecord,
// formatting the date/time columns and mapping all 11 NOT NULL numeric
// columns to exact decimal strings via numericToDecimalString (existing
// helper in atm_portal_cashpos.go).
func rowToReplenishRecord(row db.ListReplenishByTerminalRow) (ReplenishRecord, error) {
	if !row.ReplenishDate.Valid {
		return ReplenishRecord{}, fmt.Errorf("replenish_date is null")
	}

	out := ReplenishRecord{
		ReplenishDate: row.ReplenishDate.Time.Format("2006-01-02"),
		ReplenishTime: pgTimeToHHMMSS(row.ReplenishTime),
		TerminalID:    row.TerminalID,
		MachineType:   row.MachineType,
		TellerID:      row.TellerID,
		BranchCode:    row.BranchCode,
	}

	type amountField struct {
		name string
		n    pgtype.Numeric
		set  func(string)
	}
	fields := []amountField{
		{"escrow", row.Escrow, func(v string) { out.Escrow = v }},
		{"refund_denom_10k", row.RefundDenom10k, func(v string) { out.RefundDenom10k = v }},
		{"refund_denom_20k", row.RefundDenom20k, func(v string) { out.RefundDenom20k = v }},
		{"refund_denom_50k", row.RefundDenom50k, func(v string) { out.RefundDenom50k = v }},
		{"refund_denom_100k", row.RefundDenom100k, func(v string) { out.RefundDenom100k = v }},
		{"refund_total", row.RefundTotal, func(v string) { out.RefundTotal = v }},
		{"replenish_denom_10k", row.ReplenishDenom10k, func(v string) { out.ReplenishDenom10k = v }},
		{"replenish_denom_20k", row.ReplenishDenom20k, func(v string) { out.ReplenishDenom20k = v }},
		{"replenish_denom_50k", row.ReplenishDenom50k, func(v string) { out.ReplenishDenom50k = v }},
		{"replenish_denom_100k", row.ReplenishDenom100k, func(v string) { out.ReplenishDenom100k = v }},
		{"replenish_total", row.ReplenishTotal, func(v string) { out.ReplenishTotal = v }},
	}
	for _, f := range fields {
		v, err := numericToDecimalString(f.n)
		if err != nil {
			return ReplenishRecord{}, fmt.Errorf("%s: %w", f.name, err)
		}
		f.set(v)
	}
	return out, nil
}

// pgTimeToHHMMSS formats a NOT NULL pgtype.Time as "HH:MM:SS", reusing
// pgTimeToStringPtr (atm_portal.go) rather than duplicating its
// microseconds-since-midnight formatting logic.
func pgTimeToHHMMSS(t pgtype.Time) string {
	if s := pgTimeToStringPtr(t); s != nil {
		return *s
	}
	return ""
}

// numericToDecimalStringPtr converts a nullable pgtype.Numeric into a
// *string, returning nil for SQL NULL — unlike numericToDecimalString
// (atm_portal_cashpos.go), which defaults NULL to "0.00" for NOT NULL money
// columns. This is for atms.capacity_amount/low_threshold_amount/
// critical_threshold_amount, which ARE nullable (design.md: "nil = NULL").
func numericToDecimalStringPtr(n pgtype.Numeric) (*string, error) {
	if !n.Valid {
		return nil, nil
	}
	s, err := numericToDecimalString(n)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
