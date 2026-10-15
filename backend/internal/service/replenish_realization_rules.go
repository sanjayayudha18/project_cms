package service

import (
	"fmt"
	"maps"
	"slices"
	"time"
)

// Laporan realisasi vendor (.claude/sdlc/replenishment-realisasi/spec.md FR2.2, K1): pure checks, no DB.
// The vendor's numbers never decide the official classification (ITM wins, Q6c); these checks only
// stop typos.

// realizationEarliestDays: a realisasi may be dated at most this many days before the schedule (K1).
const realizationEarliestDays = 7

// RealizationRowInput is one ATM's realisasi in a vendor report.
type RealizationRowInput struct {
	TerminalID   string
	Result       string     // "success" | "failed"
	RealizedDate *time.Time // date only; nil when failed
	Amounts      map[int32]int64
}

// TicketOrder is one active ticket of the request: schedule + order per denom (ListRequestTicketOrders).
type TicketOrder struct {
	TicketID      int64
	ScheduledDate time.Time
	Orders        map[int32]int64
}

// validateRealization: one row per ATM of the request (same rule as laporan selesai), then per-row
// date and amount checks. today is the WIB calendar date.
func validateRealization(tickets map[string]TicketOrder, rows []RealizationRowInput, today time.Time) error {
	results := make([]CompletionResultInput, len(rows))
	for i, r := range rows {
		results[i] = CompletionResultInput{TerminalID: r.TerminalID, Result: r.Result}
	}
	if err := validateCompletionResults(slices.Sorted(maps.Keys(tickets)), results); err != nil {
		return err
	}
	for _, r := range rows {
		if err := validateRealizationRow(tickets[r.TerminalID], r, today); err != nil {
			return err
		}
	}
	return nil
}

func validateRealizationRow(t TicketOrder, r RealizationRowInput, today time.Time) error {
	invalid := func(format string, args ...any) error {
		return &ValidationError{Field: "realizations", Message: fmt.Sprintf(format, args...)}
	}
	if r.Result == completionFailed {
		if r.RealizedDate != nil || len(r.Amounts) > 0 {
			return invalid("ATM %s gagal: tanggal dan nominal harus kosong", r.TerminalID)
		}
		return nil
	}
	if r.RealizedDate == nil {
		return invalid("tanggal realisasi ATM %s wajib diisi", r.TerminalID)
	}
	d := civilDate(*r.RealizedDate)
	if d.After(civilDate(today)) {
		return invalid("tanggal realisasi ATM %s tidak boleh setelah hari ini", r.TerminalID)
	}
	if d.Before(civilDate(t.ScheduledDate).AddDate(0, 0, -realizationEarliestDays)) {
		return invalid("tanggal realisasi ATM %s paling awal %d hari sebelum jadwal", r.TerminalID, realizationEarliestDays)
	}
	anyFilled := false
	for _, denom := range slices.Sorted(maps.Keys(r.Amounts)) {
		amount := r.Amounts[denom]
		if _, ok := t.Orders[denom]; !ok {
			return invalid("pecahan %d bukan bagian order ATM %s", denom, r.TerminalID)
		}
		if amount < 0 || amount%int64(denom) != 0 {
			return invalid("nominal pecahan %d ATM %s harus kelipatan %d dan tidak negatif", denom, r.TerminalID, denom)
		}
		anyFilled = anyFilled || amount > 0
	}
	if !anyFilled {
		return invalid("ATM %s berhasil: isi minimal satu nominal", r.TerminalID)
	}
	return nil
}

// civilDate drops the clock and zone so dates compare as calendar days.
func civilDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
