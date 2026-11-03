package service

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func rat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad rat " + s)
	}
	return r
}

// TestComputeReplenishmentStatus covers the strict precedence order from
// design.md Property 4: unconfigured > no_data > critical > low > normal,
// with exact (non-float) money comparison at fractional boundaries (QC F7).
func TestComputeReplenishmentStatus(t *testing.T) {
	cases := []struct {
		name              string
		lowThreshold      *big.Rat
		criticalThreshold *big.Rat
		hasReplenish      bool
		refundTotal       *big.Rat
		want              string
	}{
		{"no low threshold -> unconfigured", nil, rat("1000"), true, rat("500"), "unconfigured"},
		{"no replenish record -> no_data", rat("1000"), rat("500"), false, nil, "no_data"},
		{"at or below critical -> critical", rat("1000"), rat("500"), true, rat("500"), "critical"},
		{"below critical -> critical", rat("1000"), rat("500"), true, rat("100"), "critical"},
		{"between critical and low -> low", rat("1000"), rat("500"), true, rat("800"), "low"},
		{"no critical configured, below low -> low", rat("1000"), nil, true, rat("500"), "low"},
		{"above low -> normal", rat("1000"), rat("500"), true, rat("1500"), "normal"},
		{"one cent above low -> normal", rat("1000000.00"), rat("500000"), true, rat("1000000.01"), "normal"},
		{"exactly low -> low", rat("1000000.00"), rat("500000"), true, rat("1000000"), "low"},
		{"exactly critical (fraction) -> critical", rat("1000000"), rat("500000.50"), true, rat("500000.5"), "critical"},
		{"beyond float precision -> normal", rat("9999999999999999.98"), nil, true, rat("9999999999999999.99"), "normal"},
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

// numericToRat keeps money exact: NULL → nil, any exponent, NaN rejected.
func TestNumericToRat(t *testing.T) {
	num := func(s string) pgtype.Numeric {
		var n pgtype.Numeric
		if err := n.Scan(s); err != nil {
			t.Fatalf("scan %q: %v", s, err)
		}
		return n
	}
	if r, err := numericToRat(pgtype.Numeric{}); r != nil || err != nil {
		t.Errorf("NULL = %v, %v; want nil, nil", r, err)
	}
	for in, want := range map[string]string{
		"1250000.50":          "2500001/2",
		"9999999999999999.99": "999999999999999999/100",
		"-0.01":               "-1/100",
	} {
		r, err := numericToRat(num(in))
		if err != nil || r.String() != want {
			t.Errorf("numericToRat(%s) = %v, %v; want %s", in, r, err, want)
		}
	}
	if r, err := numericToRat(pgtype.Numeric{Int: big.NewInt(5), Exp: 3, Valid: true}); err != nil || r.String() != "5000/1" {
		t.Errorf("positive exponent = %v, %v; want 5000/1", r, err)
	}
	if _, err := numericToRat(pgtype.Numeric{NaN: true, Valid: true}); err == nil {
		t.Error("NaN must be rejected")
	}
}

// T1 (QC F7): list ATM money is an exact decimal string; NULL stays nil.
func TestRowToAtmWithCashPos_MoneyAsDecimalString(t *testing.T) {
	num := func(s string) pgtype.Numeric {
		var n pgtype.Numeric
		if err := n.Scan(s); err != nil {
			t.Fatalf("scan %q: %v", s, err)
		}
		return n
	}
	got, err := rowToAtmWithCashPos(db.ListATMsWithCashPosRow{
		LowThresholdAmount: num("1000000"),
		RefundTotal:        num("1250000.50"),
		ReplenishTotal:     num("9999999999999999.99"),
		Escrow:             num("0"),
	})
	if err != nil {
		t.Fatal(err)
	}
	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	for name, c := range map[string]struct {
		got  *string
		want string
	}{
		"low":       {got.LowThresholdAmount, "1000000.00"},
		"critical":  {got.CriticalThresholdAmount, "<nil>"},
		"refund":    {got.RefundTotal, "1250000.50"},
		"replenish": {got.ReplenishTotal, "9999999999999999.99"},
		"escrow":    {got.Escrow, "0.00"},
	} {
		if str(c.got) != c.want {
			t.Errorf("%s = %s, want %s", name, str(c.got), c.want)
		}
	}
}
