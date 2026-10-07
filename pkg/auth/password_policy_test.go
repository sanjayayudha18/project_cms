package auth

import (
	"testing"
	"time"
)

func TestPasswordExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		ageDays      int
		wantExpired  bool
		wantDaysLeft int
	}{
		{name: "exactly 90 days: not yet expired", ageDays: 90, wantExpired: false, wantDaysLeft: 0},
		{name: "91 days: expired", ageDays: 91, wantExpired: true, wantDaysLeft: -1},
		{name: "83 days: in 7-day warning window", ageDays: 83, wantExpired: false, wantDaysLeft: 7},
		{name: "82 days: outside warning window", ageDays: 82, wantExpired: false, wantDaysLeft: 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changedAt := now.Add(-time.Duration(tc.ageDays) * 24 * time.Hour)

			expired, daysLeft := PasswordExpiry(&changedAt, now)

			if expired != tc.wantExpired {
				t.Errorf("expired = %v, want %v", expired, tc.wantExpired)
			}
			if daysLeft != tc.wantDaysLeft {
				t.Errorf("daysLeft = %d, want %d", daysLeft, tc.wantDaysLeft)
			}
		})
	}
}

func TestPasswordExpiry_NilChangedAt_NotEvaluated(t *testing.T) {
	// A row that predates this column (or an LDAP account the caller mistakenly
	// evaluates anyway) must never be treated as expired.
	expired, daysLeft := PasswordExpiry(nil, time.Now())

	if expired {
		t.Error("expired = true, want false for nil changedAt")
	}
	if daysLeft != PasswordMaxAgeDays {
		t.Errorf("daysLeft = %d, want %d for nil changedAt", daysLeft, PasswordMaxAgeDays)
	}
}

func TestInPasswordWarningWindow(t *testing.T) {
	tests := []struct {
		daysLeft int
		want     bool
	}{
		{daysLeft: 8, want: false},  // 82 days old — outside window
		{daysLeft: 7, want: true},   // 83 days old — warning starts
		{daysLeft: 0, want: true},   // exactly at expiry boundary, still "warn"
		{daysLeft: -1, want: false}, // already expired — not a warning, a rejection
	}

	for _, tc := range tests {
		got := InPasswordWarningWindow(tc.daysLeft)
		if got != tc.want {
			t.Errorf("InPasswordWarningWindow(%d) = %v, want %v", tc.daysLeft, got, tc.want)
		}
	}
}
