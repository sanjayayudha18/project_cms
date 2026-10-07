package service

import (
	"fmt"
	"regexp"
	"testing"

	"pgregory.net/rapid"
)

// Feature: replenishment-request-enhancements, Property 13: Format Request_Number
//
// Pure, no DB: exercises the same two pieces createWithRetryingNumber
// composes into a request_number (vendor_request_actions.go) -- the
// REP-<prefix>-<date>-<seq> literal format and fallbackVendorPrefix's
// always-3-char guarantee -- without touching Postgres.

var requestNumberFormatRe = regexp.MustCompile(`^REP-[A-Z0-9]{3}-\d{8}-\d{3}$`)

func TestProperty13_RequestNumberFormat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		code := rapid.StringMatching(`[a-zA-Z0-9 _-]{0,20}`).Draw(rt, "vendorCode")
		year := rapid.IntRange(2000, 2099).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		seq := rapid.IntRange(1, 999).Draw(rt, "seq")

		prefix := fallbackVendorPrefix(code)
		if len(prefix) != 3 {
			rt.Fatalf("fallbackVendorPrefix(%q) = %q, want exactly 3 chars", code, prefix)
		}

		dateSeg := fmt.Sprintf("%04d%02d%02d", year, month, day)
		requestNumber := fmt.Sprintf("REP-%s-%s-%03d", prefix, dateSeg, seq)

		if !requestNumberFormatRe.MatchString(requestNumber) {
			rt.Fatalf("assembled request_number %q does not match REP-<3>-<8>-<3> format", requestNumber)
		}
	})
}
