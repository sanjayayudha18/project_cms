package auth

import (
	"testing"
	"time"
)

func TestIsLocked(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		lockedUntil *time.Time
		want        bool
	}{
		{name: "nil locked_until: not locked", lockedUntil: nil, want: false},
		{name: "locked_until in the future: locked", lockedUntil: ptrTime(now.Add(1 * time.Minute)), want: true},
		{name: "locked_until exactly now: auto-unlocked (boundary exclusive)", lockedUntil: ptrTime(now), want: false},
		{name: "locked_until 31 minutes in the past: auto-unlocked", lockedUntil: ptrTime(now.Add(-31 * time.Minute)), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLocked(tc.lockedUntil, now)
			if got != tc.want {
				t.Errorf("IsLocked() = %v, want %v", got, tc.want)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
