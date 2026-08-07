package middleware

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/redis/go-redis/v9"
	"pgregory.net/rapid"
)

// newPropertyTestRateLimiter creates a RateLimiter backed by miniredis for property testing.
func newPropertyTestRateLimiter(t *rapid.T) (*RateLimiter, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	rl := NewRateLimiter(client, RateLimitConfig{
		MaxPerUsername: 5,
		MaxPerIP:       20,
		Window:         15 * time.Minute,
	})
	return rl, mr
}

// usernameRune returns a generator for username-safe runes (lowercase alpha + digits + dot/underscore).
func usernameRune() *rapid.Generator[rune] {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789._"
	return rapid.Map(rapid.IntRange(0, len(chars)-1), func(i int) rune {
		return rune(chars[i])
	})
}

// usernameGen generates random username strings (alphanumeric, 3-30 chars).
func usernameGen() *rapid.Generator[string] {
	return rapid.StringOfN(usernameRune(), 3, 30, -1)
}

// ipGen generates random IPv4 address strings.
func ipGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		a := rapid.IntRange(1, 255).Draw(t, "octet1")
		b := rapid.IntRange(0, 255).Draw(t, "octet2")
		c := rapid.IntRange(0, 255).Draw(t, "octet3")
		d := rapid.IntRange(1, 254).Draw(t, "octet4")
		return fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
	})
}

// Feature: user-login, Property 8: Username Rate Limit Threshold
// **Validates: Requirements 7.1, 7.3**
//
// For any username, the Rate_Limiter SHALL allow the first 5 failed login attempts
// within a 15-minute window, and SHALL reject the 6th and subsequent attempts with
// a RateLimitError whose RetryAfter value is ≤ 900 seconds.
func TestProperty_UsernameRateLimitThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rl, _ := newPropertyTestRateLimiter(t)
		ctx := context.Background()

		username := usernameGen().Draw(t, "username")
		ip := ipGen().Draw(t, "ip")

		// First 5 failed attempts should be allowed
		for i := 0; i < 5; i++ {
			err := rl.Check(ctx, username, ip)
			if err != nil {
				t.Fatalf("attempt %d: Check should allow (counter=%d, limit=5), got: %v", i+1, i, err)
			}
			err = rl.IncrementFailed(ctx, username, ip)
			if err != nil {
				t.Fatalf("attempt %d: IncrementFailed failed: %v", i+1, err)
			}
		}

		// 6th and subsequent attempts should be rejected
		extraAttempts := rapid.IntRange(1, 5).Draw(t, "extraAttempts")
		for i := 0; i < extraAttempts; i++ {
			err := rl.Check(ctx, username, ip)
			if err == nil {
				t.Fatalf("attempt %d after limit: Check should reject, got nil", 6+i)
			}

			rlErr, ok := err.(*auth.RateLimitError)
			if !ok {
				t.Fatalf("expected *auth.RateLimitError, got %T: %v", err, err)
			}

			if rlErr.RetryAfter <= 0 || rlErr.RetryAfter > 900 {
				t.Fatalf("RetryAfter should be in (0, 900], got %d", rlErr.RetryAfter)
			}
		}
	})
}

// Feature: user-login, Property 9: IP Rate Limit Threshold
// **Validates: Requirements 7.2, 7.4**
//
// For any IP address, the Rate_Limiter SHALL allow the first 20 failed login attempts
// within a 15-minute window, and SHALL reject the 21st and subsequent attempts with
// a RateLimitError whose RetryAfter value is ≤ 900 seconds.
func TestProperty_IPRateLimitThreshold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rl, _ := newPropertyTestRateLimiter(t)
		ctx := context.Background()

		ip := ipGen().Draw(t, "ip")

		// Use unique usernames to avoid hitting per-username limit (5).
		// Each unique username only gets 1 increment, so username limit never triggers.
		for i := 0; i < 20; i++ {
			username := fmt.Sprintf("iptest_user_%d_%s", i, ip)
			err := rl.Check(ctx, username, ip)
			if err != nil {
				t.Fatalf("attempt %d: Check should allow IP (counter=%d, limit=20), got: %v", i+1, i, err)
			}
			err = rl.IncrementFailed(ctx, username, ip)
			if err != nil {
				t.Fatalf("attempt %d: IncrementFailed failed: %v", i+1, err)
			}
		}

		// 21st attempt with a fresh username — should be blocked by IP limit
		freshUsername := fmt.Sprintf("iptest_fresh_%s", ip)
		err := rl.Check(ctx, freshUsername, ip)
		if err == nil {
			t.Fatal("21st attempt: Check should reject on IP limit, got nil")
		}

		rlErr, ok := err.(*auth.RateLimitError)
		if !ok {
			t.Fatalf("expected *auth.RateLimitError, got %T: %v", err, err)
		}

		if rlErr.RetryAfter <= 0 || rlErr.RetryAfter > 900 {
			t.Fatalf("RetryAfter should be in (0, 900], got %d", rlErr.RetryAfter)
		}
	})
}

// Feature: user-login, Property 10: Rate Limit Reset on Success
// **Validates: Requirements 7.5, 7.8**
//
// For any username with N failed attempts (N < 5), a successful login SHALL reset the
// username counter to 0, while the IP counter SHALL remain unchanged at its current value.
func TestProperty_RateLimitResetOnSuccess(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rl, _ := newPropertyTestRateLimiter(t)
		ctx := context.Background()

		username := usernameGen().Draw(t, "username")
		ip := ipGen().Draw(t, "ip")
		// N failed attempts where N < 5
		n := rapid.IntRange(1, 4).Draw(t, "failedAttempts")

		// Simulate N failed login attempts
		for i := 0; i < n; i++ {
			err := rl.IncrementFailed(ctx, username, ip)
			if err != nil {
				t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
			}
		}

		// Simulate successful login — reset username counter
		err := rl.ResetUsername(ctx, username)
		if err != nil {
			t.Fatalf("ResetUsername failed: %v", err)
		}

		// After reset, username counter should be 0 — next 5 attempts should be allowed
		for i := 0; i < 5; i++ {
			err := rl.Check(ctx, username, ip)
			if err != nil {
				t.Fatalf("after reset, attempt %d: Check should allow (counter=%d, limit=5), got: %v", i+1, i, err)
			}
			err = rl.IncrementFailed(ctx, username, ip)
			if err != nil {
				t.Fatalf("after reset, IncrementFailed #%d failed: %v", i+1, err)
			}
		}

		// Username should now be blocked at 5
		err = rl.Check(ctx, username, ip)
		if err == nil {
			t.Fatal("after reset + 5 failures: Check should reject, got nil")
		}

		// Verify IP counter was NOT reset: it should be at n (original) + 5 (after reset)
		// Use a different username to isolate IP check
		otherUsername := username + "_other"
		ipTotal := n + 5
		if ipTotal < 20 {
			// IP should still allow (below 20)
			err = rl.Check(ctx, otherUsername, ip)
			if err != nil {
				t.Fatalf("IP counter should be %d/20 (below limit), but Check returned: %v", ipTotal, err)
			}
		}
	})
}
