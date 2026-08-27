package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig holds the configuration for login rate limiting.
type RateLimitConfig struct {
	MaxPerUsername int           // 5
	MaxPerIP       int           // 20
	Window         time.Duration // 15 minutes
}

// RateLimiter enforces login rate limits using Redis counters.
// Keys follow the pattern:
//   - rate:login:user:{username} → counter with TTL = Window
//   - rate:login:ip:{ip}        → counter with TTL = Window
type RateLimiter struct {
	redis  *redis.Client
	config RateLimitConfig
}

// NewRateLimiter creates a new RateLimiter with the given Redis client and config.
func NewRateLimiter(redisClient *redis.Client, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		config: config,
	}
}

// usernameKey returns the Redis key for a username rate limit counter.
func (rl *RateLimiter) usernameKey(username string) string {
	return fmt.Sprintf("rate:login:user:%s", username)
}

// ipKey returns the Redis key for an IP rate limit counter.
func (rl *RateLimiter) ipKey(ip string) string {
	return fmt.Sprintf("rate:login:ip:%s", ip)
}

// Check verifies whether the login attempt is allowed for the given username and IP.
// Returns nil if allowed, &auth.RateLimitError if rate limit exceeded,
// or auth.ErrServiceUnavailable if Redis is unavailable (fail-closed).
func (rl *RateLimiter) Check(ctx context.Context, username, ip string) error {
	userKey := rl.usernameKey(username)
	userCount, err := rl.redis.Get(ctx, userKey).Int()
	if err != nil && err != redis.Nil {
		return auth.ErrServiceUnavailable
	}

	if userCount >= rl.config.MaxPerUsername {
		retryAfter, ttlErr := rl.getTTL(ctx, userKey)
		if ttlErr != nil {
			return auth.ErrServiceUnavailable
		}
		return &auth.RateLimitError{RetryAfter: retryAfter}
	}

	ipKeyStr := rl.ipKey(ip)
	ipCount, err := rl.redis.Get(ctx, ipKeyStr).Int()
	if err != nil && err != redis.Nil {
		return auth.ErrServiceUnavailable
	}

	if ipCount >= rl.config.MaxPerIP {
		retryAfter, ttlErr := rl.getTTL(ctx, ipKeyStr)
		if ttlErr != nil {
			return auth.ErrServiceUnavailable
		}
		return &auth.RateLimitError{RetryAfter: retryAfter}
	}

	return nil
}

// IncrementFailed increments both the username and IP counters after a failed login attempt.
// Sets the TTL on first increment (when counter goes from 0 to 1).
func (rl *RateLimiter) IncrementFailed(ctx context.Context, username, ip string) error {
	userKey := rl.usernameKey(username)
	if err := rl.incrementWithTTL(ctx, userKey); err != nil {
		return auth.ErrServiceUnavailable
	}

	ipKeyStr := rl.ipKey(ip)
	if err := rl.incrementWithTTL(ctx, ipKeyStr); err != nil {
		return auth.ErrServiceUnavailable
	}

	return nil
}

// ResetUsername deletes the username counter on successful login.
// The IP counter is intentionally left unchanged.
func (rl *RateLimiter) ResetUsername(ctx context.Context, username string) error {
	userKey := rl.usernameKey(username)
	err := rl.redis.Del(ctx, userKey).Err()
	if err != nil {
		return auth.ErrServiceUnavailable
	}
	return nil
}

// incrementWithTTL increments a counter key and sets the TTL only on the first increment.
func (rl *RateLimiter) incrementWithTTL(ctx context.Context, key string) error {
	val, err := rl.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	if val == 1 {
		if err := rl.redis.Expire(ctx, key, rl.config.Window).Err(); err != nil {
			return err
		}
	}

	return nil
}

// getTTL returns the remaining TTL for a key in seconds.
// If the key has no TTL or doesn't exist, returns the full window duration.
func (rl *RateLimiter) getTTL(ctx context.Context, key string) (int, error) {
	ttl, err := rl.redis.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	if ttl < 0 {
		return int(rl.config.Window.Seconds()), nil
	}

	return int(ttl.Seconds()), nil
}
