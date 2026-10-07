package service

import (
	"testing"
	"time"
)

// TestValidateReplenishDate covers Task 5.4 (Req 2.6, 2.7): required,
// not earlier than today in Asia/Jakarta.
func TestValidateReplenishDate(t *testing.T) {
	today := jakartaCalendarDate(0)
	tests := []struct {
		name    string
		date    time.Time
		wantErr bool
	}{
		{"missing (zero value) rejected", time.Time{}, true},
		{"yesterday rejected", today.AddDate(0, 0, -1), true},
		{"today accepted", today, false},
		{"tomorrow accepted", today.AddDate(0, 0, 1), false},
		{"far future accepted", today.AddDate(0, 0, 30), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReplenishDate(tt.date)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateReplenishDate(%v) error = %v, wantErr %v", tt.date, err, tt.wantErr)
			}
			if err != nil {
				if ve, ok := err.(*ValidationError); !ok || ve.Field != "replenish_date" {
					t.Errorf("expected *ValidationError{Field: replenish_date}, got %#v", err)
				}
			}
		})
	}
}

// TestValidateCategoryDateConsistency covers Task 5.4 (Req 3.3-3.5, 3.10,
// 3.11): planned=H+1, emergency=H+0, additional in {H+0,H+1,H+2}, and an
// invalid category is always rejected regardless of date.
func TestValidateCategoryDateConsistency(t *testing.T) {
	h0 := jakartaCalendarDate(0)
	h1 := jakartaCalendarDate(1)
	h2 := jakartaCalendarDate(2)
	h3 := jakartaCalendarDate(3)

	tests := []struct {
		name     string
		category string
		date     time.Time
		wantErr  bool
	}{
		{"planned at H+1 accepted", "planned", h1, false},
		{"planned at H+0 rejected", "planned", h0, true},
		{"planned at H+2 rejected", "planned", h2, true},
		{"emergency at H+0 accepted", "emergency", h0, false},
		{"emergency at H+1 rejected", "emergency", h1, true},
		{"additional at H+0 accepted", "additional", h0, false},
		{"additional at H+1 accepted", "additional", h1, false},
		{"additional at H+2 accepted", "additional", h2, false},
		{"additional at H+3 rejected", "additional", h3, true},
		{"empty category rejected", "", h1, true},
		{"unknown category rejected", "urgent", h1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCategoryDateConsistency(tt.category, tt.date)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCategoryDateConsistency(%q, %v) error = %v, wantErr %v", tt.category, tt.date, err, tt.wantErr)
			}
		})
	}
}

// TestAcceptItems covers Task 5.5 (Req 3.6, 3.7, 3.14): items with no
// dmaa_atm_forecast row are accepted, amount_refund is always 0, operator
// brand/lokasi_atm pass through, and shape violations are rejected.
// replenishment-request-enhancements Task 5.2 (Req 1.7, 1.8, 5.12): a
// Forecast-Browser Emergency/Additional item keeps its DMAA periode_pred
// instead of being re-anchored to replenish_date.
func TestAcceptItems(t *testing.T) {
	replenishDate := jakartaCalendarDate(0)

	t.Run("valid manual item accepted with amount_refund=0 and context fields passed through", func(t *testing.T) {
		items := []ItemInput{{
			TerminalID:      "T-MANUAL-1",
			Denom:           100000,
			AmountReplenish: 5000000,
			Brand:           "Wincor",
			LokasiATM:       "Plaza Test",
		}}
		resolved, err := acceptItems(items, replenishDate)
		if err != nil {
			t.Fatalf("acceptItems: %v", err)
		}
		if len(resolved) != 1 {
			t.Fatalf("expected 1 resolved item, got %d", len(resolved))
		}
		r := resolved[0]
		if r.amountRefund != 0 {
			t.Errorf("amountRefund = %d, want 0 (no DMAA row to copy from)", r.amountRefund)
		}
		if r.input.Brand != "Wincor" || r.input.LokasiATM != "Plaza Test" {
			t.Errorf("expected brand/lokasi_atm to pass through unchanged, got %+v", r.input)
		}
		if !r.input.PeriodePred.Equal(replenishDate) {
			t.Errorf("PeriodePred = %v, want anchored to replenish_date %v", r.input.PeriodePred, replenishDate)
		}
	})

	t.Run("forecast-browser item keeps its periode_pred (Req 1.7/1.8, 5.12)", func(t *testing.T) {
		dmaaPeriode := jakartaCalendarDate(1)
		items := []ItemInput{{
			TerminalID:      "T-DMAA-1",
			PeriodePred:     dmaaPeriode,
			Denom:           50000,
			AmountReplenish: 7500000,
		}}
		resolved, err := acceptItems(items, replenishDate)
		if err != nil {
			t.Fatalf("acceptItems: %v", err)
		}
		if !resolved[0].input.PeriodePred.Equal(dmaaPeriode) {
			t.Errorf("PeriodePred = %v, want the sent DMAA periode %v preserved", resolved[0].input.PeriodePred, dmaaPeriode)
		}
	})

	tests := []struct {
		name      string
		item      ItemInput
		wantField string
	}{
		{"empty terminal_id rejected", ItemInput{TerminalID: "", Denom: 1, AmountReplenish: 1}, "terminal_id"},
		{"terminal_id over 64 chars rejected", ItemInput{TerminalID: string(make([]byte, 65)), Denom: 1, AmountReplenish: 1}, "terminal_id"},
		{"zero denom rejected", ItemInput{TerminalID: "T1", Denom: 0, AmountReplenish: 1}, "denom"},
		{"negative denom rejected", ItemInput{TerminalID: "T1", Denom: -1, AmountReplenish: 1}, "denom"},
		{"zero amount_replenish rejected", ItemInput{TerminalID: "T1", Denom: 1, AmountReplenish: 0}, "amount_replenish"},
		{"negative amount_replenish rejected", ItemInput{TerminalID: "T1", Denom: 1, AmountReplenish: -1}, "amount_replenish"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := acceptItems([]ItemInput{tt.item}, replenishDate)
			ve, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("acceptItems() error = %v, want *ValidationError", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}
}
