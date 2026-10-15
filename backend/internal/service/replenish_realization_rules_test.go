package service

import (
	"errors"
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func dayPtr(s string) *time.Time {
	t := day(s)
	return &t
}

func TestValidateRealization(t *testing.T) {
	tickets := map[string]TicketOrder{
		"ATM1": {TicketID: 1, ScheduledDate: day("2026-10-14"), Orders: map[int32]int64{100000: 200_000_000, 50000: 50_000_000}},
		"ATM2": {TicketID: 2, ScheduledDate: day("2026-10-14"), Orders: map[int32]int64{100000: 100_000_000}},
	}
	today := day("2026-10-15")
	ok1 := RealizationRowInput{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-14"),
		Amounts: map[int32]int64{100000: 200_000_000, 50000: 0}}
	failed2 := RealizationRowInput{TerminalID: "ATM2", Result: completionFailed}

	tests := []struct {
		name    string
		rows    []RealizationRowInput
		wantErr bool
	}{
		{"success + failed valid", []RealizationRowInput{ok1, failed2}, false},
		{"amount differs from order is allowed", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-15"), Amounts: map[int32]int64{100000: 300_000_000}},
			failed2}, false},
		{"K1 earliest bound: schedule - 7 days", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-07"), Amounts: map[int32]int64{100000: 100000}},
			failed2}, false},
		{"K1 before schedule - 7 days", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-06"), Amounts: map[int32]int64{100000: 100000}},
			failed2}, true},
		{"K1 after today", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-16"), Amounts: map[int32]int64{100000: 100000}},
			failed2}, true},
		{"success without date", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, Amounts: map[int32]int64{100000: 100000}}, failed2}, true},
		{"success all zero", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-14"), Amounts: map[int32]int64{100000: 0}},
			failed2}, true},
		{"not a multiple of denom", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-14"), Amounts: map[int32]int64{50000: 75000}},
			failed2}, true},
		{"negative amount", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-14"), Amounts: map[int32]int64{100000: -100000}},
			failed2}, true},
		{"denom not in order", []RealizationRowInput{
			{TerminalID: "ATM1", Result: completionSuccess, RealizedDate: dayPtr("2026-10-14"), Amounts: map[int32]int64{20000: 20000}},
			failed2}, true},
		{"failed with date", []RealizationRowInput{ok1,
			{TerminalID: "ATM2", Result: completionFailed, RealizedDate: dayPtr("2026-10-14")}}, true},
		{"failed with amount", []RealizationRowInput{ok1,
			{TerminalID: "ATM2", Result: completionFailed, Amounts: map[int32]int64{100000: 100000}}}, true},
		{"missing ATM", []RealizationRowInput{ok1}, true},
		{"duplicate ATM", []RealizationRowInput{ok1, ok1, failed2}, true},
		{"unknown ATM", []RealizationRowInput{ok1, failed2, {TerminalID: "ATM9", Result: completionFailed}}, true},
		{"bad result", []RealizationRowInput{ok1, {TerminalID: "ATM2", Result: "done"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRealization(tickets, tt.rows, today)
			if tt.wantErr {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want ValidationError, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}
