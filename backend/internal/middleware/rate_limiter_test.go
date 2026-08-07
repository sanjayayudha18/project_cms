package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/redis/go-redis/v9"
)

// newTestRateLimiter creates a RateLimiter backed by miniredis for testing.
func newTestRateLimiter(t *testing.T) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	rl := NewRateLimiter(client, RateLimitConfig{
		MaxPerUsername: 5,
		MaxPerIP:       20,
		Window:         15 * time.Minute,
	})
	return rl, mr
}

func TestRateLimiter_AllowsUpToUsernameLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	username := "testuser"
	ip := "192.168.1.1"

	// First 5 attempts should be allowed
	for i := 0; i < 5; i++ {
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// After 5 increments, Check should block
	err := rl.Check(ctx, username, ip)
	if err == nil {
		t.Fatal("expected rate limit error after 5 failed attempts, got nil")
	}

	var rlErr *auth.RateLimitError
	if !isRateLimitError(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
}

func TestRateLimiter_AllowsBeforeUsernameLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	username := "testuser"
	ip := "192.168.1.1"

	// Increment 4 times (below limit of 5)
	for i := 0; i < 4; i++ {
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// Check should still allow (count=4, limit=5)
	err := rl.Check(ctx, username, ip)
	if err != nil {
		t.Fatalf("expected nil (below limit), got: %v", err)
	}
}

func TestRateLimiter_BlocksAt6thUsernameAttempt(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	username := "testuser"
	ip := "192.168.1.1"

	// Increment 5 times to hit the limit
	for i := 0; i < 5; i++ {
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// 6th attempt — Check should block
	err := rl.Check(ctx, username, ip)
	if err == nil {
		t.Fatal("expected rate limit error at 6th attempt, got nil")
	}

	var rlErr *auth.RateLimitError
	if !isRateLimitError(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
}

func TestRateLimiter_AllowsUpToIPLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	ip := "10.0.0.1"

	// Use different usernames so username limit (5) isn't hit
	for i := 0; i < 20; i++ {
		username := "user" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// IP counter is at 20, Check with a fresh username should block on IP
	err := rl.Check(ctx, "freshuser", ip)
	if err == nil {
		t.Fatal("expected rate limit error after 20 IP attempts, got nil")
	}

	var rlErr *auth.RateLimitError
	if !isRateLimitError(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
}

func TestRateLimiter_BlocksAt21stIPAttempt(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	ip := "10.0.0.2"

	// Use different usernames to avoid username limit
	for i := 0; i < 20; i++ {
		username := "ipuser" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// 21st — should be blocked by IP
	err := rl.Check(ctx, "newuser", ip)
	if err == nil {
		t.Fatal("expected rate limit error at 21st IP attempt, got nil")
	}

	var rlErr *auth.RateLimitError
	if !isRateLimitError(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
}

func TestRateLimiter_RetryAfterIsCorrect(t *testing.T) {
	rl, mr := newTestRateLimiter(t)
	ctx := context.Background()
	username := "retryuser"
	ip := "172.16.0.1"

	// Exhaust username limit
	for i := 0; i < 5; i++ {
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// Fast-forward time by 5 minutes to make TTL < 900
	mr.FastForward(5 * time.Minute)

	err := rl.Check(ctx, username, ip)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}

	var rlErr *auth.RateLimitError
	if !isRateLimitError(err, &rlErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}

	// Retry-After should be > 0 and ≤ 900 seconds
	if rlErr.RetryAfter <= 0 {
		t.Errorf("RetryAfter should be > 0, got %d", rlErr.RetryAfter)
	}
	if rlErr.RetryAfter > 900 {
		t.Errorf("RetryAfter should be ≤ 900, got %d", rlErr.RetryAfter)
	}
}

func TestRateLimiter_ResetUsernameClearsUsernameOnly(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	ctx := context.Background()
	username := "resetuser"
	ip := "192.168.2.1"

	// Increment 3 times for both username and IP
	for i := 0; i < 3; i++ {
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed #%d failed: %v", i+1, err)
		}
	}

	// Reset username counter
	if err := rl.ResetUsername(ctx, username); err != nil {
		t.Fatalf("ResetUsername failed: %v", err)
	}

	// Username counter should be reset — 5 more increments from fresh should be allowed
	for i := 0; i < 5; i++ {
		if err := rl.IncrementFailed(ctx, username, ip); err != nil {
			t.Fatalf("IncrementFailed after reset #%d failed: %v", i+1, err)
		}
	}

	// Username counter now at 5 again — should block
	err := rl.Check(ctx, username, ip)
	if err == nil {
		t.Fatal("expected rate limit after re-exhausting username limit")
	}

	// Verify that IP counter was NOT reset: it should be at 3 + 5 = 8
	// Use a different username to isolate IP check
	err = rl.Check(ctx, "otheruser", ip)
	if err != nil {
		t.Fatalf("expected IP counter to still be below limit (8/20), got: %v", err)
	}
}

func TestRateLimiter_RedisFailureReturns503(t *testing.T) {
	rl, mr := newTestRateLimiter(t)
	ctx := context.Background()

	// Close miniredis to simulate Redis failure
	mr.Close()

	// Check should return ErrServiceUnavailable
	err := rl.Check(ctx, "anyuser", "1.2.3.4")
	if err == nil {
		t.Fatal("expected error when Redis is down, got nil")
	}
	if err != auth.ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable, got: %v", err)
	}

	// IncrementFailed should also return ErrServiceUnavailable
	err = rl.IncrementFailed(ctx, "anyuser", "1.2.3.4")
	if err == nil {
		t.Fatal("expected error from IncrementFailed when Redis is down, got nil")
	}
	if err != auth.ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable from IncrementFailed, got: %v", err)
	}

	// ResetUsername should also return ErrServiceUnavailable
	err = rl.ResetUsername(ctx, "anyuser")
	if err == nil {
		t.Fatal("expected error from ResetUsername when Redis is down, got nil")
	}
	if err != auth.ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable from ResetUsername, got: %v", err)
	}
}

// isRateLimitError checks if err is a *auth.RateLimitError and assigns it to target.
func isRateLimitError(err error, target **auth.RateLimitError) bool {
	if e, ok := err.(*auth.RateLimitError); ok {
		*target = e
		return true
	}
	return false
}
