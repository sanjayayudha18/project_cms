package reports

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// wantVE asserts err is a *ValidationError with exactly field+msg (or nil when field == "").
func wantVE(t *testing.T, err error, field, msg string) {
	t.Helper()
	if field == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != field || ve.Message != msg {
		t.Errorf("got {%q, %q}, want {%q, %q}", ve.Field, ve.Message, field, msg)
	}
	if want := field + ": " + msg; ve.Error() != want {
		t.Errorf("Error() = %q, want %q", ve.Error(), want)
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// Property 3: parameter validation equals the legacy handler/service rules.
func TestParseDsrLateMonth(t *testing.T) {
	cases := []struct{ name, raw, field, msg string }{
		{"missing", "", "month", "month wajib diisi (YYYY-MM)"},
		{"bad format", "2026-13", "month", "format month YYYY-MM"},
		{"day given", "2026-10-01", "month", "format month YYYY-MM"},
		{"ok", "2026-10", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseDsrLateMonth(tc.raw)
			wantVE(t, err, tc.field, tc.msg)
			if tc.field == "" && !m.Equal(day(2026, 10, 1)) {
				t.Errorf("month = %v, want 2026-10-01", m)
			}
		})
	}
}

func TestValidateDsrLateMonth(t *testing.T) {
	const msg = "bulan tidak boleh setelah bulan berjalan"
	oct := day(2026, 10, 1)
	cases := []struct {
		name         string
		month, now   time.Time
		field, wantM string
	}{
		{"zero month", time.Time{}, oct, "month", "month wajib diisi (YYYY-MM)"},
		{"current month", oct, time.Date(2026, 10, 15, 12, 0, 0, 0, WIB), "", ""},
		{"past month", day(2026, 9, 1), oct, "", ""},
		{"future month", day(2026, 11, 1), time.Date(2026, 10, 31, 12, 0, 0, 0, WIB), "month", msg},
		// 2026-10-31 18:00 UTC = 2026-11-01 01:00 WIB: November is already "current" in WIB.
		{"month change: UTC still Oct, WIB Nov", day(2026, 11, 1), time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC), "", ""},
		{"month change: Dec not yet", day(2026, 12, 1), time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC), "month", msg},
		// 2026-10-31 16:59:59 UTC = 23:59:59 WIB: still October.
		{"just before WIB month change", day(2026, 11, 1), time.Date(2026, 10, 31, 16, 59, 59, 0, time.UTC), "month", msg},
		{"just at WIB month change", day(2026, 11, 1), time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC), "", ""},
		{"year boundary", day(2027, 1, 1), time.Date(2026, 12, 31, 17, 0, 0, 0, time.UTC), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wantVE(t, ValidateDsrLateMonth(tc.month, tc.now), tc.field, tc.wantM) })
	}
}

func TestValidateReplenishmentRange(t *testing.T) {
	from := day(2026, 10, 1)
	cases := []struct {
		name         string
		from, to     time.Time
		field, wantM string
	}{
		{"from missing", time.Time{}, from, "date_from", "date_from dan date_to wajib diisi"},
		{"to missing", from, time.Time{}, "date_from", "date_from dan date_to wajib diisi"},
		{"both missing", time.Time{}, time.Time{}, "date_from", "date_from dan date_to wajib diisi"},
		{"to before from", from, day(2026, 9, 30), "date_to", "date_to tidak boleh sebelum date_from"},
		{"same day", from, from, "", ""},
		{"31 days inclusive ok", from, day(2026, 10, 31), "", ""},
		{"32 days rejected", from, day(2026, 11, 1), "date_to", "rentang maksimal 31 hari"},
		{"time of day ignored (to 23:59 on day 31)", from.Add(23 * time.Hour), time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC), "", ""},
		{"time of day ignored (32nd day)", from, time.Date(2026, 11, 1, 0, 0, 1, 0, time.UTC), "date_to", "rentang maksimal 31 hari"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wantVE(t, ValidateReplenishmentRange(tc.from, tc.to), tc.field, tc.wantM) })
	}
}

// Reference predicates written straight from the legacy rules, independent of the production code.
func refMonthField(month, now time.Time) string {
	if month.IsZero() {
		return "month"
	}
	n := now.In(WIB)
	if month.Year()*12+int(month.Month()) > n.Year()*12+int(n.Month()) {
		return "month"
	}
	return ""
}

func refRangeField(from, to time.Time) string {
	if from.IsZero() || to.IsZero() {
		return "date_from"
	}
	if to.Before(from) {
		return "date_to"
	}
	fd := day(from.Year(), from.Month(), from.Day()).Unix() / 86400
	td := day(to.Year(), to.Month(), to.Day()).Unix() / 86400
	if td-fd+1 > 31 {
		return "date_to"
	}
	return ""
}

func fieldOf(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Field
	}
	if err != nil {
		return "NOT_VALIDATION_ERROR"
	}
	return ""
}

func FuzzValidateDsrLateMonth(f *testing.F) {
	f.Add(int64(1792000000), int64(1792000000), false)
	f.Add(int64(1793491200), int64(1793469600), false) // Nov 1 vs 2026-10-31 18:00 UTC
	f.Add(int64(0), int64(0), true)
	f.Fuzz(func(t *testing.T, monthSec, nowSec int64, zero bool) {
		// Keep within years ~1970..2200 so Date arithmetic never overflows.
		const span = 230 * 365 * 24 * 3600
		ms, ns := monthSec%span, nowSec%span
		if ms < 0 {
			ms = -ms
		}
		if ns < 0 {
			ns = -ns
		}
		mt := time.Unix(ms, 0).UTC()
		month := day(mt.Year(), mt.Month(), 1)
		if zero {
			month = time.Time{}
		}
		now := time.Unix(ns, 0)
		if got, want := fieldOf(ValidateDsrLateMonth(month, now)), refMonthField(month, now); got != want {
			t.Fatalf("month=%v now=%v: field %q, want %q", month, now, got, want)
		}
	})
}

func FuzzValidateReplenishmentRange(f *testing.F) {
	f.Add(int64(1790000000), int64(1790000000), false, false)
	f.Add(int64(1790000000), int64(1790000000+30*86400), false, false)
	f.Add(int64(1790000000), int64(1790000000+31*86400), false, false)
	f.Add(int64(0), int64(0), true, false)
	f.Fuzz(func(t *testing.T, fromSec, toSec int64, zeroFrom, zeroTo bool) {
		const span = 230 * 365 * 24 * 3600
		fs, ts := fromSec%span, toSec%span
		if fs < 0 {
			fs = -fs
		}
		if ts < 0 {
			ts = -ts
		}
		from, to := time.Unix(fs, 0).UTC(), time.Unix(ts, 0).UTC()
		if zeroFrom {
			from = time.Time{}
		}
		if zeroTo {
			to = time.Time{}
		}
		if got, want := fieldOf(ValidateReplenishmentRange(from, to)), refRangeField(from, to); got != want {
			t.Fatalf("from=%v to=%v: field %q, want %q", from, to, got, want)
		}
	})
}

// Property 7: DSR start-date gate.
func TestParseDsrLateStartDate(t *testing.T) {
	if d, err := ParseDsrLateStartDate(""); d != nil || err != nil {
		t.Errorf("empty = (%v, %v), want (nil, nil)", d, err)
	}
	d, err := ParseDsrLateStartDate("2026-10-27")
	if err != nil || d == nil || !d.Equal(day(2026, 10, 27)) {
		t.Errorf("valid = (%v, %v), want 2026-10-27", d, err)
	}
	for _, raw := range []string{"27-10-2026", "2026-10", "2026-02-30", "abc"} {
		d, err := ParseDsrLateStartDate(raw)
		if d != nil || err == nil || !strings.Contains(err.Error(), "DSR_LATE_START_DATE") {
			t.Errorf("%q = (%v, %v), want nil + error naming DSR_LATE_START_DATE", raw, d, err)
		}
	}
}

// Remaining branches.
func TestUnknownKind(t *testing.T) {
	if h := Header("nope"); len(h) != 0 {
		t.Errorf("Header(unknown) = %v, want empty", h)
	}
	if got := FileName("nope", day(2026, 10, 1), day(2026, 10, 1), day(2026, 10, 2)); got != "" {
		t.Errorf("FileName(unknown) = %q, want empty", got)
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, "nope", nil); err == nil || buf.Len() != 0 {
		t.Errorf("WriteCSV(unknown) err=%v written=%d, want error and no output", err, buf.Len())
	}
}

func TestHeaderReturnsCopy(t *testing.T) {
	h := Header(ReplenishmentTrips)
	h[0] = "x"
	if Header(ReplenishmentTrips)[0] != "vendor_id" {
		t.Error("Header must return a copy")
	}
}

func TestNilCellsRenderEmpty(t *testing.T) {
	if got := DsrLateDailyCells(DsrLateDailyRow{Deadline: day(2026, 10, 1)}); got[5] != "" || got[7] != "" || got[9] != "0" {
		t.Errorf("nil region/received = %q", got)
	}
	if lines := AmountMismatchLines(AmountMismatchRow{Amounts: []Amount{{Denom: 50000}}}); len(lines) != 1 || lines[0][0] != "" || lines[0][10] != "" || lines[0][14] != "" {
		t.Errorf("nil ptrs = %q", lines)
	}
	if got := AmountMismatchLines(AmountMismatchRow{}); len(got) != 0 {
		t.Errorf("no amounts = %v, want none", got)
	}
	if got := TripCells(TripRow{TerminalID: "T"}); got[0] != "" || got[1] != "" || got[4] != "" || got[5] != "" {
		t.Errorf("nil trip = %q", got)
	}
	// realized date is used when scheduled is nil
	if got := dateCell(nil, ptr(day(2026, 10, 7))); got != "2026-10-07" {
		t.Errorf("dateCell fallback = %q", got)
	}
}

// failWriter fails once more than limit bytes were accepted.
type failWriter struct{ limit, n int }

func (w *failWriter) Write(p []byte) (int, error) {
	if w.n+len(p) > w.limit {
		return 0, errors.New("disk full")
	}
	w.n += len(p)
	return len(p), nil
}

func TestWriteCSVErrors(t *testing.T) {
	row := make([]string, len(Header(ReplenishmentTrips)))
	big := append([]string(nil), row...)
	big[1] = strings.Repeat("x", 10000)
	cases := []struct {
		name  string
		limit int
		rows  [][]string
	}{
		{"BOM write fails", 0, nil},
		{"header flush fails", 3, [][]string{row}},                // BOM ok, buffered header fails at Flush
		{"row write fails (buffer overflow)", 3, [][]string{big}}, // oversize row forces a write mid-Write
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := WriteCSV(&failWriter{limit: tc.limit}, ReplenishmentTrips, tc.rows); err == nil {
				t.Error("want error")
			}
		})
	}
}
