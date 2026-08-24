package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func f64(v float64) *float64 { return &v }

// TestComputeReplenishmentStatus covers the strict precedence order from
// design.md Property 4: unconfigured > no_data > critical > low > normal.
func TestComputeReplenishmentStatus(t *testing.T) {
	cases := []struct {
		name              string
		lowThreshold      *float64
		criticalThreshold *float64
		hasReplenish      bool
		refundTotal       *float64
		want              string
	}{
		{"no low threshold -> unconfigured", nil, f64(1000), true, f64(500), "unconfigured"},
		{"no replenish record -> no_data", f64(1000), f64(500), false, nil, "no_data"},
		{"at or below critical -> critical", f64(1000), f64(500), true, f64(500), "critical"},
		{"below critical -> critical", f64(1000), f64(500), true, f64(100), "critical"},
		{"between critical and low -> low", f64(1000), f64(500), true, f64(800), "low"},
		{"no critical configured, below low -> low", f64(1000), nil, true, f64(500), "low"},
		{"above low -> normal", f64(1000), f64(500), true, f64(1500), "normal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeReplenishmentStatus(tc.lowThreshold, tc.criticalThreshold, tc.hasReplenish, tc.refundTotal)
			if got != tc.want {
				t.Errorf("computeReplenishmentStatus(%v, %v, %v, %v) = %q, want %q",
					tc.lowThreshold, tc.criticalThreshold, tc.hasReplenish, tc.refundTotal, got, tc.want)
			}
		})
	}
}

// profileFakeRepo implements AtmPortalRepository + AtmPortalProfileRepository.
type profileFakeRepo struct {
	atmRow    db.GetATMByTerminalIDRow
	atmErr    error
	refund    pgtype.Numeric
	refundErr error
}

func (f *profileFakeRepo) ListATMsWithCashPos(context.Context, db.ListATMsWithCashPosParams) ([]db.ListATMsWithCashPosRow, error) {
	return nil, errors.New("not implemented")
}
func (f *profileFakeRepo) CountATMsWithCashPos(context.Context, db.CountATMsWithCashPosParams) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *profileFakeRepo) GetATMSummary(context.Context) (db.GetATMSummaryRow, error) {
	return db.GetATMSummaryRow{}, errors.New("not implemented")
}
func (f *profileFakeRepo) GetLastUpdated(context.Context) (pgtype.Timestamp, error) {
	return pgtype.Timestamp{}, errors.New("not implemented")
}
func (f *profileFakeRepo) GetATMByTerminalID(context.Context, string) (db.GetATMByTerminalIDRow, error) {
	return f.atmRow, f.atmErr
}
func (f *profileFakeRepo) GetLatestReplenishForTerminal(context.Context, string) (pgtype.Numeric, error) {
	return f.refund, f.refundErr
}
func (f *profileFakeRepo) ListReplenishByTerminal(context.Context, db.ListReplenishByTerminalParams) ([]db.ListReplenishByTerminalRow, error) {
	return nil, nil
}
func (f *profileFakeRepo) CountReplenishByTerminal(context.Context, db.CountReplenishByTerminalParams) (int64, error) {
	return 0, nil
}
func (f *profileFakeRepo) ListCashposByTerminal(context.Context, db.ListCashposByTerminalParams) ([]db.ItmCashpo, error) {
	return nil, nil
}
func (f *profileFakeRepo) CountCashposByTerminal(context.Context, db.CountCashposByTerminalParams) (int64, error) {
	return 0, nil
}

func TestGetATMProfile_NotFound(t *testing.T) {
	fake := &profileFakeRepo{atmErr: pgx.ErrNoRows}
	svc := NewAtmPortalService(fake)
	_, err := svc.GetATMProfile(context.Background(), "MISSING")
	if !errors.Is(err, ErrATMNotFound) {
		t.Fatalf("want ErrATMNotFound, got %v", err)
	}
}

func TestGetATMProfile_EmptyTerminalID(t *testing.T) {
	svc := NewAtmPortalService(&profileFakeRepo{})
	_, err := svc.GetATMProfile(context.Background(), "   ")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "terminalId" {
		t.Fatalf("want terminalId ValidationError, got %v", err)
	}
}

func TestGetATMProfile_NoDataStatus(t *testing.T) {
	fake := &profileFakeRepo{
		atmRow: db.GetATMByTerminalIDRow{
			TerminalID:         "ATM001",
			LocationName:       "KCP Sudirman",
			Address:            "Jl. Sudirman",
			MachineType:        "CRM",
			Brand:              "NCR",
			Model:              "SelfServ 84",
			DeploymentType:     "ONSITE",
			OperationHours:     "24_HOURS",
			LowThresholdAmount: mustNumeric(t, "50000000.00"),
			IsActive:           true,
		},
		refundErr: pgx.ErrNoRows,
	}
	svc := NewAtmPortalService(fake)
	result, err := svc.GetATMProfile(context.Background(), "ATM001")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result.ReplenishmentStatus != "no_data" {
		t.Errorf("status = %q, want no_data", result.ReplenishmentStatus)
	}
	if result.CriticalThresholdAmount != nil {
		t.Errorf("critical_threshold_amount = %v, want nil (SQL NULL)", *result.CriticalThresholdAmount)
	}
}

func TestListATMReplenish_ReversedDateRangeReturnsEmpty(t *testing.T) {
	svc := NewAtmPortalService(&profileFakeRepo{})
	result, err := svc.ListATMReplenish(context.Background(), ListATMReplenishParams{
		TerminalID: "ATM001", Page: 1, PageSize: 25,
		DateFrom: "2026-08-21", DateTo: "2026-08-01",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(result.Data) != 0 || result.Total != 0 {
		t.Errorf("want empty result for reversed range, got %+v", result)
	}
}

func TestListATMCashpos_ReversedDateRangeIsValidationError(t *testing.T) {
	svc := NewAtmPortalService(&profileFakeRepo{})
	_, err := svc.ListATMCashpos(context.Background(), ListATMCashposParams{
		TerminalID: "ATM001", Page: 1, PageSize: 25,
		DateFrom: "2026-08-21", DateTo: "2026-08-01",
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "date_from" {
		t.Fatalf("want date_from ValidationError, got %v", err)
	}
}
