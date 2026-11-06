//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property tests for vendor_pks_status() and vendor_pks_validity_months()
// (migration 022; spec vendor-pks Property 1 + 2). The functions are pure, so one
// rolled-back tx serves every iteration. Skips when DATABASE_URL is empty.

// addMonthsClamped is the Go reference for "date + N months, clamped to month end".
func addMonthsClamped(d time.Time, n int) time.Time {
	total := d.Year()*12 + int(d.Month()) - 1 + n
	y, m := total/12, time.Month(total%12+1)
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := d.Day()
	if day > last {
		day = last
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func ymd(t time.Time) string { return t.Format("2006-01-02") }

// refStatus is the Req 6.2–6.6 reference model.
func refStatus(kind string, expiry *time.Time, terminated bool, today time.Time, months int) *string {
	s := func(v string) *string { return &v }
	switch {
	case kind == "INTERNAL":
		return nil
	case expiry == nil:
		return s("belum_diisi")
	case terminated:
		return s("terminated")
	case expiry.Before(today):
		return s("expired")
	case expiry.Before(addMonthsClamped(today, months)):
		return s("need_renewal")
	}
	return s("aktif")
}

// refValidityMonths: largest n >= 0 with addMonthsClamped(eff, n) <= exp + 1 day.
func refValidityMonths(eff, exp time.Time) int {
	end := exp.AddDate(0, 0, 1)
	n := 0
	for !addMonthsClamped(eff, n+1).After(end) {
		n++
	}
	return n
}

var (
	pksMinDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	pksMaxDate = time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC)
	pksDays    = int(pksMaxDate.Sub(pksMinDate).Hours() / 24)
)

func drawDate(t *rapid.T, label string) time.Time {
	return pksMinDate.AddDate(0, 0, rapid.IntRange(0, pksDays).Draw(t, label))
}

func TestVendorPksStatusFixedCases(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	var got string
	// Expiry day itself is still valid: not "expired".
	if err := tx.QueryRow(ctx,
		`SELECT vendor_pks_status('FLM_VENDOR', '2026-11-05'::date, NULL, '2026-11-05'::date, 9)`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == "expired" {
		t.Fatalf("expiry day must not be expired, got %q", got)
	}
	// Day after expiry is expired.
	if err := tx.QueryRow(ctx,
		`SELECT vendor_pks_status('FLM_VENDOR', '2026-11-04'::date, NULL, '2026-11-05'::date, 9)`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "expired" {
		t.Fatalf("got %q, want expired", got)
	}

	var months int
	if err := tx.QueryRow(ctx,
		`SELECT vendor_pks_validity_months('2026-01-01'::date, '2027-12-31'::date)`).Scan(&months); err != nil {
		t.Fatal(err)
	}
	if months != 24 {
		t.Fatalf("2026-01-01 → 2027-12-31 = %d months, want 24", months)
	}

	if d := addMonthsClamped(time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), 9); ymd(d) != "2027-02-28" {
		t.Fatalf("reference model: 2026-05-31 + 9 months = %s, want 2027-02-28", ymd(d))
	}
}

// Property 1: Aturan status PKS.
func TestVendorPksStatusProperty(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.SampledFrom([]string{"FLM_VENDOR", "INTERNAL"}).Draw(rt, "kind")
		today := drawDate(rt, "today")
		months := rapid.IntRange(1, 24).Draw(rt, "renewal_months")
		hasPks := rapid.Bool().Draw(rt, "has_pks")
		terminated := rapid.Bool().Draw(rt, "terminated")

		// Bias expiry toward the today / today+N-months boundaries.
		var expiry time.Time
		switch rapid.IntRange(0, 2).Draw(rt, "expiry_mode") {
		case 0:
			expiry = drawDate(rt, "expiry")
		case 1:
			expiry = today.AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, "d_today"))
		default:
			expiry = addMonthsClamped(today, months).AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, "d_renew"))
		}

		var expArg *string
		var expRef *time.Time
		if hasPks {
			s := ymd(expiry)
			expArg, expRef = &s, &expiry
		}
		var termArg *time.Time
		if terminated {
			ts := time.Now().UTC()
			termArg = &ts
		}

		var got *string
		err := tx.QueryRow(ctx,
			`SELECT vendor_pks_status($1, $2::date, $3::timestamptz, $4::date, $5::int)`,
			kind, expArg, termArg, ymd(today), months).Scan(&got)
		if err != nil {
			rt.Fatalf("query: %v", err)
		}
		want := refStatus(kind, expRef, terminated, today, months)
		if (got == nil) != (want == nil) || (got != nil && *got != *want) {
			rt.Fatalf("kind=%s expiry=%v terminated=%v today=%s months=%d: got %v, want %v",
				kind, expArg, terminated, ymd(today), months, deref(got), deref(want))
		}
	})
}

// Property 2: Masa berlaku dalam bulan penuh.
func TestVendorPksValidityMonthsProperty(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		eff := drawDate(rt, "effective")
		var exp time.Time
		if rapid.Bool().Draw(rt, "near_month_end") {
			// expiry = (eff + n months) - 1 day ± 1: exercises month-end clamping.
			n := rapid.IntRange(1, 24).Draw(rt, "n")
			exp = addMonthsClamped(eff, n).AddDate(0, 0, rapid.IntRange(-2, 1).Draw(rt, "off"))
			if exp.Before(eff) {
				exp = eff
			}
		} else {
			exp = eff.AddDate(0, 0, rapid.IntRange(0, 1500).Draw(rt, "span"))
		}

		var got int
		if err := tx.QueryRow(ctx,
			`SELECT vendor_pks_validity_months($1::date, $2::date)`, ymd(eff), ymd(exp)).Scan(&got); err != nil {
			rt.Fatalf("query: %v", err)
		}
		if want := refValidityMonths(eff, exp); got != want {
			rt.Fatalf("eff=%s exp=%s: got %d, want %d", ymd(eff), ymd(exp), got, want)
		}
	})
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
